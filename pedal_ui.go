// Copyright (C) 2026 Wilson Neto and contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

// The pedal screen. It takes over the view once a read has been recognised
// as a model in `devices`: the pedal drawn as it looks, the selected switch in
// detail, presses as they happen, and editing that writes only what changed
// -- after showing exactly what that is -- and then reads the pedal back to
// prove it. Nothing here knows which model it is drawing; see pedal.go.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mvave-chocolate-tui/internal/sysex"
)

const (
	flashFor = 450 * time.Millisecond // how long a pressed switch stays lit
	ackWait  = 1500 * time.Millisecond
	liveKeep = 6 // presses shown under the pedal
)

var (
	pBrand    = lipgloss.Color("212")
	pAccent   = lipgloss.Color("42")
	pWarn     = lipgloss.Color("214")
	pDim      = lipgloss.Color("243")
	pFaint    = lipgloss.Color("238")
	pTitle    = lipgloss.NewStyle().Bold(true).Foreground(pBrand)
	pMuted    = lipgloss.NewStyle().Foreground(pDim)
	pFaintTxt = lipgloss.NewStyle().Foreground(pFaint)
	pGood     = lipgloss.NewStyle().Foreground(pAccent)
	pChanged  = lipgloss.NewStyle().Foreground(pWarn).Bold(true)
	pCursor   = lipgloss.NewStyle().Background(pBrand).Foreground(lipgloss.Color("0")).Bold(true)
	pPanel    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(pFaint).Padding(0, 1)
)

// pedalUI is the pedal screen's own state.
type pedalUI struct {
	sel   int // selected switch, 0..3
	group int // the group being viewed and edited, from 0 -- not necessarily the pedal's
	// liveGroup is the group the pedal was last heard sending from; -1 until
	// a press says. Display only.
	liveGroup int
	editing   bool // moving between fields of the selected switch
	field     int  // index into the device's Fields()
	typed     string

	draft  *Config // edits not yet written; nil = none
	expect *Config // what a read after writing must show

	naming bool
	name   string
	labels switchLabels

	confirm bool          // showing the write confirmation
	pending []sysex.Write // what that confirmation would write
	writing bool

	mark   sysex.Image // compare against this read; nil = not comparing
	markAt time.Time

	picking bool     // choosing a backup to restore
	files   []string // newest first
	pick    int

	flashAt   [4]time.Time
	flashSlot [4]int
	live      []string
}

// pedalAck is signalled by the read loop for every OK the pedal sends.
var pedalAck = make(chan struct{}, 32)

type writeDoneMsg struct {
	written int
	err     error
}

// --- names, kept beside the TUI's own config -------------------------------

func labelsPath() string { return filepath.Join(filepath.Dir(configFilePath()), "switch-labels.json") }

// switchLabels are names per group and switch: group 2's A is not group
// 1's A, and naming one must not rename the other.
type switchLabels [8][4]string

func loadLabels() (l switchLabels) {
	data, err := os.ReadFile(labelsPath())
	if err != nil {
		return l
	}
	if json.Unmarshal(data, &l) == nil {
		return l
	}
	// Written before groups: four names, one per switch -- group 1's.
	var old [4]string
	if json.Unmarshal(data, &old) == nil {
		l[0] = old
	}
	return l
}

func saveLabels(l switchLabels) {
	if data, err := json.MarshalIndent(l, "", "  "); err == nil {
		_ = os.MkdirAll(filepath.Dir(labelsPath()), 0o755)
		_ = os.WriteFile(labelsPath(), data, 0o644)
	}
}

// shown is what the screen displays: the draft if there is one, else the pedal.
func (m *Model) shown() Config {
	if m.pui.draft != nil {
		return *m.pui.draft
	}
	return *m.cfg
}

func (m *Model) curFields() []Field { return m.device.Fields(m.shown().Groups[m.pui.group][m.pui.sel]) }

func fieldText(f Field, v int) string {
	if f.Format != nil {
		return f.Format(v)
	}
	return strconv.Itoa(v)
}

func (m *Model) setField(f Field, v int) {
	if m.pui.draft == nil {
		d := m.cfg.Clone()
		m.pui.draft = &d
	}
	f.Set(&m.pui.draft.Groups[m.pui.group][m.pui.sel], v)
}

func (m *Model) nudgeField(delta int) {
	f := m.curFields()[m.pui.field]
	cur := f.Get(m.shown().Groups[m.pui.group][m.pui.sel])
	if len(f.Choices) > 0 {
		i := 0
		for k, c := range f.Choices {
			if int(c) == cur {
				i = k
			}
		}
		m.setField(f, int(f.Choices[(i+delta+len(f.Choices))%len(f.Choices)]))
		return
	}
	m.setField(f, clamp(cur+delta, f.Min, f.Max))
}

// --- keys ------------------------------------------------------------------

func (m *Model) handlePedalKey(msg tea.KeyMsg) tea.Cmd {
	k := msg.String()
	p := &m.pui

	if p.writing {
		return nil // nothing until the writes and the read-back finish
	}

	if p.naming {
		switch k {
		case "enter":
			p.labels[p.group][p.sel] = strings.TrimSpace(p.name)
			saveLabels(p.labels)
			p.naming = false
		case "esc":
			p.naming = false
		case "backspace":
			if len(p.name) > 0 {
				p.name = p.name[:len(p.name)-1]
			}
		default:
			if len(msg.Runes) > 0 && len(p.name) < 12 {
				p.name += string(msg.Runes)
			}
		}
		return nil
	}

	if p.picking {
		switch k {
		case "up", "k":
			if p.pick > 0 {
				p.pick--
			}
		case "down", "j":
			if p.pick < len(p.files)-1 {
				p.pick++
			}
		case "enter":
			p.picking = false
			if len(p.files) > 0 {
				m.loadBackup(p.files[p.pick])
			}
		case "esc", "i":
			p.picking = false
		}
		return nil
	}

	if p.confirm {
		switch k {
		case "y", "Y":
			p.confirm = false
			return m.writePending()
		case "n", "N", "esc":
			p.confirm = false
			m.statusMsg = "Not written"
		}
		return nil
	}

	if p.editing {
		fs := m.curFields()
		f := fs[p.field]
		switch k {
		case "esc", "enter":
			if p.typed != "" {
				if v, err := strconv.Atoi(p.typed); err == nil {
					m.setField(f, clamp(v, f.Min, f.Max))
				}
				p.typed = ""
				if k == "enter" {
					return nil
				}
			}
			p.editing = false
		case "up", "k":
			p.typed = ""
			if p.field > 0 {
				p.field--
			}
		case "down", "j":
			p.typed = ""
			if p.field < len(fs)-1 {
				p.field++
			}
		case "left", "h", "-":
			p.typed = ""
			m.nudgeField(-1)
		case "right", "l", "+", "=":
			p.typed = ""
			m.nudgeField(+1)
		case "backspace":
			if len(p.typed) > 0 {
				p.typed = p.typed[:len(p.typed)-1]
			}
		case "n":
			m.addMessage(f)
		case "x":
			m.removeMessage(f)
		default:
			if len(k) == 1 && k[0] >= '0' && k[0] <= '9' && len(f.Choices) == 0 && len(p.typed) < 3 {
				p.typed += k
			}
		}
		return nil
	}

	switch k {
	case "q", "ctrl+c":
		if m.midi != nil {
			_ = m.midi.Close()
			m.midi = nil
		}
		return tea.Quit
	case "left", "h":
		if p.sel > 0 {
			p.sel--
		}
	case "right", "l":
		if p.sel < 3 {
			p.sel++
		}
	case "a", "b", "c", "d":
		p.sel = int(k[0] - 'a')
	case "enter", "e":
		p.editing, p.field, p.typed = true, 0, ""
	case "n":
		p.naming, p.name = true, p.labels[p.group][p.sel]
	case "u":
		p.draft = nil
		m.statusMsg = "Edits discarded"
	case "w":
		m.askToWrite()
	case "r":
		if p.draft != nil {
			m.statusMsg = "Unwritten edits — write them (w) or discard them (u) first"
			return nil
		}
		m.requestConfig()
	case "x":
		m.exportBackup()
	case "i":
		p.files, p.pick, p.picking = backups(m.device.Product()), 0, true
	case "m":
		m.openModePicker()
	case "v":
		if p.mark != nil {
			p.mark = nil
			m.statusMsg = "Stopped comparing"
		} else {
			m.markForCompare()
		}
	case "]", "[":
		n := len(m.shown().Groups)
		if k == "]" && p.group < n-1 {
			p.group++
		} else if k == "[" && p.group > 0 {
			p.group--
		}
		if n > 1 {
			m.statusMsg = fmt.Sprintf("Viewing group %d of %d", p.group+1, n)
		}
	case "+", "-":
		m.nudgeGroupCount(map[string]int{"+": 1, "-": -1}[k])
	case "p":
		m.pedalToViewedGroup()
	case "tab":
		m.logView = !m.logView
	}
	return nil
}

func (m *Model) askToWrite() {
	if m.pui.draft == nil {
		m.statusMsg = "Nothing changed"
		return
	}
	if !m.allowWrite {
		m.statusMsg = "Read-only — restart with --write to write to the pedal"
		return
	}
	changes, err := m.device.Changes(m.pedalImage, *m.pui.draft)
	if err != nil {
		m.statusMsg = "Cannot write: " + err.Error()
		return
	}
	if len(changes) == 0 {
		m.pui.draft = nil
		m.statusMsg = "Nothing changed"
		return
	}
	m.pui.pending = changes
	m.pui.confirm = true
}

// writePending sends each byte, waiting for the pedal's OK before the next,
// and stops at the first one it does not acknowledge.
func (m *Model) writePending() tea.Cmd {
	changes := m.pui.pending
	dev := m.midi
	m.pui.writing = true
	m.pui.expect = m.pui.draft
	m.statusMsg = fmt.Sprintf("Writing %d byte(s)…", len(changes))
	for len(pedalAck) > 0 {
		<-pedalAck
	}
	return func() tea.Msg {
		for i, c := range changes {
			msg, err := sysex.BuildWrite(c.Addr, c.New)
			if err != nil {
				return writeDoneMsg{i, err}
			}
			if err := dev.SendSysex(msg); err != nil {
				return writeDoneMsg{i, err}
			}
			select {
			case <-pedalAck:
			case <-time.After(ackWait):
				return writeDoneMsg{i, fmt.Errorf("the pedal did not acknowledge address %d", c.Addr)}
			}
		}
		return writeDoneMsg{written: len(changes)}
	}
}

func (m *Model) finishWrite(done writeDoneMsg) {
	if done.err != nil {
		m.pui.writing = false
		m.pui.expect = nil
		m.pui.draft = nil
		m.statusMsg = fmt.Sprintf("Stopped after %d of %d: %v — reading the pedal back",
			done.written, len(m.pui.pending), done.err)
		m.requestConfig()
		return
	}
	m.statusMsg = fmt.Sprintf("%d byte(s) written — reading back to verify…", done.written)
	m.requestConfig()
}

// verifyWrite runs when a read completes after writing.
func (m *Model) verifyWrite() {
	want := m.pui.expect
	m.pui.expect, m.pui.writing = nil, false
	if want == nil {
		return
	}
	if changes, err := m.device.Changes(m.pedalImage, *want); err == nil && len(changes) == 0 {
		m.pui.draft = nil
		m.statusMsg = "Written and verified ✓"
		return
	}
	m.statusMsg = "Read-back does not match what was written — edits kept"
}

// --- presses ---------------------------------------------------------------

func (m *Model) notePress(msg Message) {
	if m.cfg == nil {
		return
	}
	stamp := time.Now().Format("15:04:05")
	g, s, slot, ok := m.device.Match(*m.cfg, msg)
	if !ok {
		m.pushLive(fmt.Sprintf("%s  ?  %-9s %s", stamp, "", msg))
		return
	}
	// A press tells which group the pedal is on, whatever E/F did since the
	// last read. Shown, never written: it goes in pui, not in the read config,
	// or an edit started now would plan a write of the group byte.
	if m.device.HasGroupKeys() {
		m.pui.liveGroup = g
	}
	if g == m.pui.group {
		m.pui.flashAt[s] = time.Now()
		m.pui.flashSlot[s] = slot
	}
	where := fmt.Sprintf("%c", 'A'+s)
	if m.device.HasGroupKeys() {
		where = fmt.Sprintf("%d%c", g+1, 'A'+s)
	}
	name := m.device.SlotNames(m.cfg.Groups[g][s])[slot]
	m.pushLive(fmt.Sprintf("%s  %-3s %-9s %s", stamp, where, name, msg))
}

func (m *Model) pushLive(line string) {
	m.pui.live = append(m.pui.live, line)
	if len(m.pui.live) > liveKeep {
		m.pui.live = m.pui.live[len(m.pui.live)-liveKeep:]
	}
}

// --- drawing -----------------------------------------------------------------

func (m *Model) viewPedal() string {
	cfg := m.shown()
	p := &m.pui

	head := pTitle.Render(m.device.Product()) +
		pMuted.Render("  ·  USB "+m.alsaName()+"  ·  "+m.device.ModeName(cfg.Mode))
	if m.device.HasGroupKeys() {
		head += pMuted.Render("  ·  ") + m.groupLine(cfg)
	}
	access := pMuted.Render("read-only")
	if m.allowWrite {
		access = pChanged.Render("write enabled")
	}
	head += pMuted.Render("  ·  ") + access
	if c := m.device.Caveat(); c != "" {
		head += "\n" + pChanged.Render("⚠ "+c)
	}

	var lower string
	switch {
	case p.confirm:
		lower = m.drawConfirm()
	case m.mp.open || m.mp.writing:
		lower = m.drawModePicker()
	case p.picking:
		lower = m.drawBackups()
	case p.mark != nil:
		lower = m.drawCompare()
	case m.logView:
		lower = m.viewport.View()
	default:
		lower = m.drawLive()
	}

	status := statusStyle.Render(m.statusMsg)
	if p.draft != nil && !p.writing {
		status = pChanged.Render("● unwritten edits") + "  " + status
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		head, "", m.drawPedal(cfg), "", m.drawDetail(cfg), "", lower, "", status, pMuted.Render(m.pedalHelp()))
}

func (m *Model) alsaName() string {
	// /dev/snd/midiC4D0 -> hw:4,0
	base := filepath.Base(m.midiPath)
	var c, d int
	if _, err := fmt.Sscanf(base, "midiC%dD%d", &c, &d); err == nil {
		return fmt.Sprintf("hw:%d,%d", c, d)
	}
	return base
}

func (m *Model) drawPedal(cfg Config) string {
	box := func(s int) string {
		sw := cfg.Groups[m.pui.group][s]
		lit := time.Since(m.pui.flashAt[s]) < flashFor
		border := pFaint
		if s == m.pui.sel {
			border = pBrand
		}
		if lit {
			border = pAccent
		}
		name := m.pui.labels[m.pui.group][s]
		if m.pui.naming && s == m.pui.sel {
			name = m.pui.name + "▏"
		}
		if name == "" {
			name = pFaintTxt.Render("unnamed")
		}
		first := "—"
		if len(sw.Slots) > 0 && len(sw.Slots[0]) > 0 {
			first = sw.Slots[0][0].Short()
		}
		sub := pMuted.Render(first)
		if lit {
			sub = pGood.Render("● " + m.device.SlotNames(sw)[m.pui.flashSlot[s]])
		}
		letter := lipgloss.NewStyle().Bold(true).Foreground(border).Render(string(rune('A' + s)))
		return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
			Width(13).Align(lipgloss.Center).
			Render(lipgloss.JoinVertical(lipgloss.Center, letter, name, sub))
	}
	link := func(l string) string {
		if !m.device.HasGroupKeys() {
			l = "─"
		}
		return lipgloss.NewStyle().Foreground(pFaint).PaddingTop(2).Render("─" + l + "─")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		box(0), link("E"), box(1), "     ", box(2), link("F"), box(3))
}

// drawDetail draws the selected switch's fields, grouped into their rows.
func (m *Model) drawDetail(cfg Config) string {
	p := &m.pui
	sw := cfg.Groups[p.group][p.sel]
	was := m.cfg.Groups[p.group][p.sel]
	fields := m.device.Fields(sw)

	title := fmt.Sprintf("Switch %c", 'A'+p.sel)
	if l := p.labels[p.group][p.sel]; l != "" {
		title += " · " + l
	}
	lines := []string{pTitle.Render(title)}

	width := 0 // the label column fits the widest row name
	for _, f := range fields {
		if n := len([]rune(f.Row)); n > width {
			width = n
		}
	}
	var row string
	var cells []string
	newRow := false
	flush := func() {
		if row != "" {
			line := pMuted.Render(fmt.Sprintf("%-*s", width, row)) + "  " + strings.Join(cells, "  ")
			if newRow {
				line += pFaintTxt.Render("  (new)")
			}
			lines = append(lines, line)
		}
	}
	for i, f := range fields {
		if f.Row != row {
			flush()
			row, cells, newRow = f.Row, nil, !f.In(was)
		}
		v := f.Get(sw)
		text := fieldText(f, v)
		if len(f.Choices) == 0 && f.Format == nil {
			text = fmt.Sprintf("%3d", v)
		}
		existed := f.In(was)
		old := 0
		if existed {
			old = f.Get(was)
		}
		cell := text
		switch {
		case p.editing && i == p.field:
			if p.typed != "" {
				text = p.typed + "▏"
			}
			cell = pCursor.Render(text) // no padding: it would knock the column out of line
		case !existed:
			cell = pChanged.Render(text)
		case v != old:
			cell = pChanged.Render(text) + pFaintTxt.Render(" (was "+fieldText(f, old)+")")
		}
		if f.Label != "" {
			cell = pMuted.Render(f.Label) + " " + cell
		}
		cells = append(cells, cell)
	}
	flush()
	return pPanel.Width(62).Render(strings.Join(lines, "\n"))
}

func (m *Model) drawLive() string {
	lines := []string{pMuted.Render("presses")}
	if len(m.pui.live) == 0 {
		lines = append(lines, pFaintTxt.Render("  step on a switch"))
	}
	for i := len(m.pui.live) - 1; i >= 0; i-- {
		lines = append(lines, "  "+m.pui.live[i])
	}
	return strings.Join(lines, "\n")
}

func (m *Model) drawConfirm() string {
	var b strings.Builder
	b.WriteString(pChanged.Render(fmt.Sprintf("Write %d byte(s) to the pedal?", len(m.pui.pending))) + "\n")
	for _, c := range m.pui.pending {
		fmt.Fprintf(&b, "  address %4d   %3d → %s\n", c.Addr, c.Old, pChanged.Render(strconv.Itoa(int(c.New))))
	}
	b.WriteString(pMuted.Render("Each is sent on its own and must be acknowledged; the pedal is read back after."))
	return pPanel.BorderForeground(pWarn).Render(b.String())
}

func (m *Model) pedalHelp() string {
	p := &m.pui
	switch {
	case m.mp.open || m.mp.writing:
		return m.modeHelp()
	case p.writing:
		return "writing…"
	case p.confirm:
		return "y write · n cancel"
	case p.picking:
		return "↑↓ choose · enter load as edits · esc cancel"
	case p.naming:
		return "type a name · enter save · esc cancel"
	case p.editing:
		if m.device.MaxMessages() > 1 {
			return "↑↓ field · ←→ or −+ change · 0-9 type · n add message · x remove last · enter set · esc done"
		}
		return "↑↓ field · ←→ or −+ change · 0-9 type · enter set · esc done"
	}
	groups := ""
	if m.device.HasGroupKeys() {
		groups = "[ ] group · +/- groups in use · p pedal to this group\n"
	}
	return "←→/a-d switch · enter edit · n name · w write · u undo · r read\n" + groups +
		"m mode · x export · i import · v mark & compare · tab log · q quit"
}

// addMessage appends a message to the slot under the cursor (the first slot
// when the cursor is on the switch's mode): a CC on the channel the slot
// already uses, so it is ready to edit.
func (m *Model) addMessage(f Field) {
	slot := f.Slot
	if slot < 0 {
		slot = 0
	}
	sw := m.shown().Groups[m.pui.group][m.pui.sel]
	if m.device.MaxMessages() <= 1 || len(sw.Slots[slot]) >= m.device.MaxMessages() {
		m.statusMsg = "No room for another message there"
		return
	}
	msg := Message{Type: MsgCC, Channel: 1, Value: 127}
	if n := len(sw.Slots[slot]); n > 0 {
		msg.Channel = sw.Slots[slot][n-1].Channel
	}
	if m.pui.draft == nil {
		d := m.cfg.Clone()
		m.pui.draft = &d
	}
	d := &m.pui.draft.Groups[m.pui.group][m.pui.sel]
	d.Slots[slot] = append(d.Slots[slot], msg)
	m.statusMsg = "Message added — w writes it"
}

// removeMessage drops the LAST message of the slot under the cursor: the
// pedal keeps a slot's messages as one unbroken run.
func (m *Model) removeMessage(f Field) {
	slot := f.Slot
	if slot < 0 {
		slot = 0
	}
	sw := m.shown().Groups[m.pui.group][m.pui.sel]
	if m.device.MaxMessages() <= 1 || len(sw.Slots[slot]) == 0 {
		m.statusMsg = "Nothing to remove there"
		return
	}
	if m.pui.draft == nil {
		d := m.cfg.Clone()
		m.pui.draft = &d
	}
	d := &m.pui.draft.Groups[m.pui.group][m.pui.sel]
	d.Slots[slot] = d.Slots[slot][:len(d.Slots[slot])-1]
	if fs := m.curFields(); m.pui.field >= len(fs) {
		m.pui.field = len(fs) - 1
	}
	m.statusMsg = "Last message removed — w writes it"
}

// --- groups ------------------------------------------------------------------

// groupLine is the header's account of groups: which one is on screen, how
// many the pedal steps through, and which one it is on. An unwritten change
// to either of the last two is marked.
func (m *Model) groupLine(cfg Config) string {
	line := pMuted.Render(fmt.Sprintf("group %d/%d", m.pui.group+1, len(cfg.Groups)))
	if cfg.GroupCount > 0 {
		inUse := fmt.Sprintf("%d in use", cfg.GroupCount)
		if cfg.GroupCount != m.cfg.GroupCount {
			line += pMuted.Render("  ·  ") + pChanged.Render(inUse)
		} else {
			line += pMuted.Render("  ·  " + inUse)
		}
	}
	on := cfg.CurrentGroup
	if m.pui.liveGroup >= 0 && cfg.CurrentGroup == m.cfg.CurrentGroup {
		on = m.pui.liveGroup // what the pedal last sent from beats a stale read
	}
	if on >= 0 {
		txt := fmt.Sprintf("pedal on %d", on+1)
		if cfg.CurrentGroup != m.cfg.CurrentGroup {
			line += pMuted.Render("  ·  ") + pChanged.Render(txt)
		} else {
			line += pMuted.Render("  ·  " + txt)
		}
	}
	return line
}

func (m *Model) ensureDraft() *Config {
	if m.pui.draft == nil {
		d := m.cfg.Clone()
		m.pui.draft = &d
	}
	return m.pui.draft
}

// nudgeGroupCount changes how many groups E/F step through.
func (m *Model) nudgeGroupCount(delta int) {
	if !m.device.HasGroupKeys() || m.cfg.GroupCount == 0 {
		m.statusMsg = "The group count has not been read"
		return
	}
	d := m.ensureDraft()
	n := d.GroupCount + delta
	if n < 1 || n > len(d.Groups) {
		m.statusMsg = fmt.Sprintf("Groups in use: 1-%d", len(d.Groups))
		return
	}
	d.GroupCount = n
	if d.CurrentGroup >= n {
		m.statusMsg = fmt.Sprintf("%d groups in use — but the pedal is on group %d; p to move it first", n, d.CurrentGroup+1)
		return
	}
	m.statusMsg = fmt.Sprintf("%d groups in use — w writes it", n)
}

// pedalToViewedGroup makes the group on screen the one the pedal is on.
func (m *Model) pedalToViewedGroup() {
	if !m.device.HasGroupKeys() || m.cfg.CurrentGroup < 0 {
		m.statusMsg = "The pedal's current group has not been read"
		return
	}
	d := m.ensureDraft()
	if m.pui.group >= d.GroupCount {
		m.statusMsg = fmt.Sprintf("Group %d is beyond the %d in use; + first", m.pui.group+1, d.GroupCount)
		return
	}
	d.CurrentGroup = m.pui.group
	m.statusMsg = fmt.Sprintf("Pedal to group %d — w writes it", m.pui.group+1)
}
