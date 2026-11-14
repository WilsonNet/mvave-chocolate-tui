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
	sel     int  // selected switch, 0..3
	editing bool // moving between fields of the selected switch
	field   int  // index into the device's Fields()
	typed   string

	draft  *Config // edits not yet written; nil = none
	expect *Config // what a read after writing must show

	naming bool
	name   string
	labels [4]string

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

func loadLabels() (l [4]string) {
	if data, err := os.ReadFile(labelsPath()); err == nil {
		_ = json.Unmarshal(data, &l)
	}
	return l
}

func saveLabels(l [4]string) {
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

func (m *Model) curFields() []Field { return m.device.Fields(m.shown().Switches[m.pui.sel]) }

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
	f.Set(&m.pui.draft.Switches[m.pui.sel], v)
}

func (m *Model) nudgeField(delta int) {
	f := m.curFields()[m.pui.field]
	cur := f.Get(m.shown().Switches[m.pui.sel])
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
			p.labels[p.sel] = strings.TrimSpace(p.name)
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
		p.naming, p.name = true, p.labels[p.sel]
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
		if p.mark != nil {
			p.mark = nil
			m.statusMsg = "Stopped comparing"
		} else {
			m.markForCompare()
		}
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

func (m *Model) notePress(channel, number, value byte) {
	if m.cfg == nil {
		return
	}
	stamp := time.Now().Format("15:04:05")
	msg := Message{Channel: channel, Number: number, Value: value}
	if s, slot, ok := m.device.Match(*m.cfg, channel, number, value); ok {
		m.pui.flashAt[s] = time.Now()
		m.pui.flashSlot[s] = slot
		name := m.device.SlotNames(m.cfg.Switches[s])[slot]
		m.pushLive(fmt.Sprintf("%s  %c  %-9s %s", stamp, 'A'+s, name, msg))
		return
	}
	m.pushLive(fmt.Sprintf("%s  ?  %-9s %s", stamp, "", msg))
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
		head += pMuted.Render("  ·  group 1")
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
		sw := cfg.Switches[s]
		lit := time.Since(m.pui.flashAt[s]) < flashFor
		border := pFaint
		if s == m.pui.sel {
			border = pBrand
		}
		if lit {
			border = pAccent
		}
		name := m.pui.labels[s]
		if m.pui.naming && s == m.pui.sel {
			name = m.pui.name + "▏"
		}
		if name == "" {
			name = pFaintTxt.Render("unnamed")
		}
		first := "—"
		if len(sw.Slots) > 0 && len(sw.Slots[0]) > 0 {
			first = sw.Slots[0][0].String()
			if sw.Slots[0][0].Stored() {
				first = fmt.Sprintf("CC%d=%d", sw.Slots[0][0].Number, sw.Slots[0][0].Value)
			}
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
	sw := cfg.Switches[p.sel]
	was := m.cfg.Switches[p.sel]
	fields := m.device.Fields(sw)

	title := fmt.Sprintf("Switch %c", 'A'+p.sel)
	if l := p.labels[p.sel]; l != "" {
		title += " · " + l
	}
	lines := []string{pTitle.Render(title)}

	var row string
	var cells []string
	flush := func() {
		if row != "" {
			lines = append(lines, pMuted.Render(fmt.Sprintf("%-10s", row))+"  "+strings.Join(cells, "  "))
		}
	}
	for i, f := range fields {
		if f.Row != row {
			flush()
			row, cells = f.Row, nil
		}
		v := f.Get(sw)
		text := fieldText(f, v)
		if len(f.Choices) == 0 && f.Format == nil {
			text = fmt.Sprintf("%3d", v)
		}
		old := f.Get(was)
		cell := text
		switch {
		case p.editing && i == p.field:
			if p.typed != "" {
				text = p.typed + "▏"
			}
			cell = pCursor.Render(text) // no padding: it would knock the column out of line
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
	case p.writing:
		return "writing…"
	case p.confirm:
		return "y write · n cancel"
	case p.picking:
		return "↑↓ choose · enter load as edits · esc cancel"
	case p.naming:
		return "type a name · enter save · esc cancel"
	case p.editing:
		return "↑↓ field · ←→ or −+ change · 0-9 type · enter set · esc done"
	}
	return "←→/a-d switch · enter edit · n name · w write · u undo · r read\n" +
		"x export · i import · m mark & compare · tab log · q quit"
}
