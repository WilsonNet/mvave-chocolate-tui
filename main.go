// Copyright (C) 2026 Wilson Neto
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const defaultMidiDevice = "/dev/snd/midiC5D0"

const (
	modeCustom         = 0x07
	modeProgramChangeA = 0x00
	modeProgramChangeB = 0x01
	modeProgramChangeC = 0x0B
	modeKeyboardA      = 0x03
	modeKeyboardB      = 0x04
	modeMultiMedia     = 0x05
	modeTouchScreen    = 0x02
	modeManufacturer   = 0x06
	modeVideo          = 0x08
	modeAdvancedCustom = 0x09
	modeCustomKeyboard = 0x0A
)

var modeNames = map[byte]string{
	modeCustom:         "Custom CC",
	modeProgramChangeA: "Program Change A",
	modeProgramChangeB: "Program Change B",
	modeProgramChangeC: "Program Change C",
	modeKeyboardA:      "Keyboard A",
	modeKeyboardB:      "Keyboard B",
	modeMultiMedia:     "MultiMedia",
	modeTouchScreen:    "Touch Screen",
	modeManufacturer:   "Manufacturer",
	modeVideo:          "Video",
	modeAdvancedCustom: "Advanced Custom",
	modeCustomKeyboard: "Custom Keyboard",
}

type SwitchConfig struct {
	Type     string
	CC       int
	Channel  int
	Value1   int
	Value2   int
	Latching bool
}

type SwitchState int

const (
	stateReleased SwitchState = iota
	statePressed
)

type MidiMsg struct {
	Timestamp time.Time
	Hex       string
}

// textField holds state for an editable text field
type textField struct {
	value       string
	cursor      int
	placeholder string
}

func newTextField(placeholder string) textField {
	return textField{placeholder: placeholder}
}

func (tf *textField) set(v string) { tf.value = v; tf.cursor = len(v) }
func (tf *textField) insert(ch rune) {
	tf.value = tf.value[:tf.cursor] + string(ch) + tf.value[tf.cursor:]
	tf.cursor++
}
func (tf *textField) backspace() {
	if tf.cursor > 0 {
		tf.value = tf.value[:tf.cursor-1] + tf.value[tf.cursor:]
		tf.cursor--
	}
}
func (tf *textField) deleteForward() {
	if tf.cursor < len(tf.value) {
		tf.value = tf.value[:tf.cursor] + tf.value[tf.cursor+1:]
	}
}
func (tf *textField) left() {
	if tf.cursor > 0 {
		tf.cursor--
	}
}
func (tf *textField) right() {
	if tf.cursor < len(tf.value) {
		tf.cursor++
	}
}
func (tf *textField) display() string {
	if tf.value == "" {
		return dimStyle.Render(tf.placeholder)
	}
	v := tf.value
	if tf.cursor >= len(v) {
		return v + cursorStyle.Render(" ")
	}
	return v[:tf.cursor] + cursorStyle.Render(string(v[tf.cursor])) + v[tf.cursor+1:]
}

type tickMsg time.Time

type Model struct {
	midiPath   string
	midi       *MidiDevice
	midiMsgs   chan MidiMsg
	ready      bool
	connected  bool
	mode       byte
	log        []MidiMsg
	statusMsg  string
	errorMsg   string
	width      int
	height     int
	logView    bool
	modeSelect bool
	editSwitch bool
	editIdx    int
	editField  int

	config  [16]SwitchConfig
	swState [16]SwitchState

	table    table.Model
	viewport viewport.Model
	help     help.Model
	keymap   keyMap

	fields [6]textField
}

type keyMap struct {
	Quit      key.Binding
	Tab       key.Binding
	Edit      key.Binding
	Send      key.Binding
	LogToggle key.Binding
	Mode      key.Binding
	Read      key.Binding
	Up        key.Binding
	Down      key.Binding
	Left      key.Binding
	Right     key.Binding
	Confirm   key.Binding
	Cancel    key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Edit, k.Send, k.Read, k.Mode, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Tab, k.Edit, k.Send, k.Read, k.Mode},
		{k.LogToggle, k.Up, k.Down, k.Left, k.Right, k.Quit},
	}
}

var defaultKeymap = keyMap{
	Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Tab:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "log view")),
	Edit:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit switch")),
	Send:      key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "send all to device")),
	LogToggle: key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "log")),
	Mode:      key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mode select")),
	Read:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "read config from device")),
	Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Left:      key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "switch left")),
	Right:     key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "switch right")),
	Confirm:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	Cancel:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).MarginBottom(1)
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
	cursorStyle = lipgloss.NewStyle().Background(lipgloss.Color("212")).Foreground(lipgloss.Color("0"))
	highlight   = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
)

func NewModel(midiPath string) Model {
	cfg := [16]SwitchConfig{}
	for i := range cfg {
		cfg[i] = SwitchConfig{
			Type:    "cc",
			CC:      20 + i,
			Channel: 0,
			Value1:  127,
			Value2:  0,
		}
	}

	cols := []table.Column{
		{Title: "Switch", Width: 8},
		{Title: "Type", Width: 6},
		{Title: "Ch", Width: 4},
		{Title: "CC/Note", Width: 8},
		{Title: "Val1", Width: 6},
		{Title: "Val2", Width: 6},
		{Title: "Latch", Width: 6},
		{Title: "State", Width: 6},
	}

	rows := make([]table.Row, 16)
	for i := 0; i < 16; i++ {
		rows[i] = makeSwitchRow(i, cfg[i], stateReleased)
	}

	t := table.New(table.WithColumns(cols), table.WithRows(rows), table.WithHeight(18))
	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).BorderBottom(true).Bold(true)
	s.Selected = s.Selected.Foreground(lipgloss.Color("0")).Background(lipgloss.Color("212"))
	t.SetStyles(s)

	vp := viewport.New(80, 10)
	vp.SetContent("MIDI log...")

	fields := [6]textField{
		newTextField("cc/pc/note/sysex"),
		newTextField("0-127"),
		newTextField("0-15"),
		newTextField("0-127"),
		newTextField("0-127"),
		newTextField("no/yes"),
	}

	return Model{
		midiPath:  midiPath,
		mode:      modeCustom,
		midiMsgs:  make(chan MidiMsg, 256),
		config:    cfg,
		table:     t,
		viewport:  vp,
		help:      help.New(),
		keymap:    defaultKeymap,
		fields:    fields,
		statusMsg: "Starting...",
	}
}

func makeSwitchRow(i int, cfg SwitchConfig, st SwitchState) table.Row {
	bank := i/4 + 1
	sw := i % 4
	label := fmt.Sprintf("%d%c", bank, 'A'+sw)
	state := "⬆"
	if st == statePressed {
		state = "⬇"
	}
	latch := "no"
	if cfg.Latching {
		latch = "yes"
	}
	ccLabel := fmt.Sprintf("%d", cfg.CC)
	if cfg.Type == "note" {
		ccLabel = fmt.Sprintf("N%d", cfg.CC)
	} else if cfg.Type == "pc" {
		ccLabel = fmt.Sprintf("P%d", cfg.CC)
	}
	return table.Row{label, cfg.Type, fmt.Sprintf("%d", cfg.Channel), ccLabel, fmt.Sprintf("%d", cfg.Value1), fmt.Sprintf("%d", cfg.Value2), latch, state}
}

func (m *Model) refreshTable() {
	rows := make([]table.Row, 16)
	for i := 0; i < 16; i++ {
		rows[i] = makeSwitchRow(i, m.config[i], m.swState[i])
	}
	m.table.SetRows(rows)
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		connectMidiCmd(m.midiPath),
	)
}

type midiConnectedMsg struct {
	dev *MidiDevice
	err error
}

func connectMidiCmd(path string) tea.Cmd {
	return func() tea.Msg {
		dev, err := OpenMidiDevice(path)
		return midiConnectedMsg{dev: dev, err: err}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Drain ALL pending MIDI messages on every Update call
	for {
		select {
		case midiMsg := <-m.midiMsgs:
			m.processMidiMessage(midiMsg)
		default:
			goto drained
		}
	}
drained:

	switch msg := msg.(type) {
	case midiConnectedMsg:
		if msg.err != nil {
			m.errorMsg = msg.err.Error()
			m.ready = true
		} else {
			m.midi = msg.dev
			m.connected = true
			m.ready = true
			m.statusMsg = fmt.Sprintf("Connected to %s — press 'r' to read device config", m.midiPath)
			cmds = append(cmds, m.midiReadLoop())
		}
		return m, tea.Batch(cmds...)

	case tickMsg:
		cmds = append(cmds, tickCmd())

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 20
		m.table.SetWidth(msg.Width - 4)
		m.table.SetHeight(msg.Height - 20)

	case tea.KeyMsg:
		if m.modeSelect {
			cmds = append(cmds, m.handleModeSelect(msg))
			m.refreshTable()
			return m, tea.Batch(cmds...)
		}
		if m.editSwitch {
			cmds = append(cmds, m.handleEdit(msg))
			m.refreshTable()
			return m, tea.Batch(cmds...)
		}

		switch {
		case key.Matches(msg, m.keymap.Quit):
			if m.midi != nil {
				m.midi.Close()
				m.midi = nil
			}
			return m, tea.Quit
		case key.Matches(msg, m.keymap.Tab):
			m.logView = !m.logView
		case key.Matches(msg, m.keymap.LogToggle):
			m.logView = !m.logView
		case key.Matches(msg, m.keymap.Edit):
			m.startEdit(m.table.Cursor())
		case key.Matches(msg, m.keymap.Mode):
			m.modeSelect = true
			m.statusMsg = "Select mode (↑↓ to choose, enter to confirm, esc to cancel)"
		case key.Matches(msg, m.keymap.Read):
			m.requestConfig()
		case key.Matches(msg, m.keymap.Send):
			m.sendAllConfig()
		case key.Matches(msg, m.keymap.Up):
			m.table.MoveUp(1)
		case key.Matches(msg, m.keymap.Down):
			m.table.MoveDown(1)
		case key.Matches(msg, m.keymap.Left):
			m.table.MoveUp(4)
		case key.Matches(msg, m.keymap.Right):
			m.table.MoveDown(4)
		}
	}

	m.refreshTable()
	m.updateViewport()
	return m, tea.Batch(cmds...)
}

func (m *Model) updateViewport() {
	var lines []string
	start := 0
	if len(m.log) > 100 {
		start = len(m.log) - 100
	}
	for _, msg := range m.log[start:] {
		lines = append(lines, msg.Hex)
	}
	if len(lines) > 0 {
		m.viewport.SetContent(strings.Join(lines, "\n"))
		m.viewport.GotoBottom()
	}
}

func (m *Model) midiReadLoop() tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 64)
		for {
			if m.midi == nil {
				return nil
			}
			n, err := m.midi.Read(buf)
			if err != nil {
				// Device closed or error - don't block on send
				select {
				case m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "ERR: " + err.Error()}:
				default:
				}
				return nil
			}
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				select {
				case m.midiMsgs <- MidiMsg{
					Timestamp: time.Now(),
					Hex:       "RX " + hex.EncodeToString(data),
				}:
				default:
				}
			}
			time.Sleep(time.Millisecond)
		}
	}
}

func (m *Model) processMidiMessage(msg MidiMsg) {
	if strings.HasPrefix(msg.Hex, "DONE:") {
		m.statusMsg = msg.Hex
		m.log = append(m.log, msg)
		return
	}
	m.log = append(m.log, msg)
	if len(m.log) > 1000 {
		m.log = m.log[1:]
	}
	if strings.HasPrefix(msg.Hex, "RX ") {
		hexStr := strings.TrimPrefix(msg.Hex, "RX ")
		data, err := hex.DecodeString(strings.ReplaceAll(hexStr, " ", ""))
		if err != nil {
			return
		}
		m.parseMidiBytes(data)
	}
}

func (m *Model) parseMidiBytes(data []byte) {
	for i := 0; i < len(data); {
		b := data[i]
		switch {
		case b >= 0xB0 && b < 0xC0:
			if i+3 > len(data) {
				return
			}
			cc := int(data[i+1])
			val := int(data[i+2])
			m.setSwitchByCC(cc, val > 0)
			i += 3
		case b >= 0x90 && b < 0xA0:
			if i+3 > len(data) {
				return
			}
			note := int(data[i+1])
			vel := int(data[i+2])
			m.setSwitchByNote(note, vel > 0)
			i += 3
		case b == 0xF0:
			end := -1
			for j := i; j < len(data); j++ {
				if data[j] == 0xF7 {
					end = j
					break
				}
			}
			if end < 0 {
				return
			}
			m.log = append(m.log, MidiMsg{Timestamp: time.Now(), Hex: "SYX " + hex.EncodeToString(data[i:end+1])})
			i = end + 1
		default:
			i++
		}
	}
}

func (m *Model) setSwitchByCC(cc int, pressed bool) {
	for i, cfg := range m.config {
		if cfg.Type == "cc" && cfg.CC == cc {
			if pressed {
				m.swState[i] = statePressed
			} else {
				m.swState[i] = stateReleased
			}
			return
		}
	}
}

func (m *Model) setSwitchByNote(note int, pressed bool) {
	for i, cfg := range m.config {
		if cfg.Type == "note" && cfg.CC == note {
			if pressed {
				m.swState[i] = statePressed
			} else {
				m.swState[i] = stateReleased
			}
			return
		}
	}
}

func (m *Model) startEdit(sw int) {
	m.editSwitch = true
	m.editIdx = sw
	m.editField = 0
	cfg := m.config[sw]
	m.fields[0].set(cfg.Type)
	m.fields[1].set(strconv.Itoa(cfg.CC))
	m.fields[2].set(strconv.Itoa(cfg.Channel))
	m.fields[3].set(strconv.Itoa(cfg.Value1))
	m.fields[4].set(strconv.Itoa(cfg.Value2))
	if cfg.Latching {
		m.fields[5].set("yes")
	} else {
		m.fields[5].set("no")
	}
	bank := sw/4 + 1
	letter := 'A' + sw%4
	m.statusMsg = fmt.Sprintf("Editing switch %d%c", bank, letter)
}

func (m *Model) handleEdit(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.editSwitch = false
		m.statusMsg = "Edit cancelled"
		return nil
	case "tab":
		m.editField = (m.editField + 1) % len(m.fields)
		return nil
	case "shift+tab":
		m.editField--
		if m.editField < 0 {
			m.editField = len(m.fields) - 1
		}
		return nil
	case "enter":
		m.applyEdit()
		m.editSwitch = false
		m.statusMsg = "Switch updated (press 's' to send to device)"
		return nil
	case "up":
		if m.editField > 0 {
			m.editField--
		}
		return nil
	case "down":
		if m.editField < len(m.fields)-1 {
			m.editField++
		}
		return nil
	}

	tf := &m.fields[m.editField]

	switch msg.String() {
	case "left":
		tf.left()
	case "right":
		tf.right()
	case "backspace":
		tf.backspace()
	case "delete":
		tf.deleteForward()
	}

	if len(msg.Runes) == 1 {
		r := msg.Runes[0]
		if r >= 32 && r < 127 {
			tf.insert(r)
		}
	}

	return nil
}

func (m *Model) applyEdit() {
	sw := m.editIdx
	cfg := &m.config[sw]

	if v := m.fields[0].value; v != "" {
		cfg.Type = v
	}
	if v, err := strconv.Atoi(m.fields[1].value); err == nil {
		if v >= 0 && v <= 127 {
			cfg.CC = v
		}
	}
	if v, err := strconv.Atoi(m.fields[2].value); err == nil {
		if v >= 0 && v <= 15 {
			cfg.Channel = v
		}
	}
	if v, err := strconv.Atoi(m.fields[3].value); err == nil {
		if v >= 0 && v <= 127 {
			cfg.Value1 = v
		}
	}
	if v, err := strconv.Atoi(m.fields[4].value); err == nil {
		if v >= 0 && v <= 127 {
			cfg.Value2 = v
		}
	}
	cfg.Latching = strings.ToLower(m.fields[5].value) == "yes" || m.fields[5].value == "1"
}

func (m *Model) handleModeSelect(msg tea.KeyMsg) tea.Cmd {
	if key.Matches(msg, m.keymap.Cancel) {
		m.modeSelect = false
		m.statusMsg = "Mode selection cancelled"
		return nil
	}
	if key.Matches(msg, m.keymap.Confirm) {
		m.modeSelect = false
		m.sendModeChange()
		m.statusMsg = fmt.Sprintf("Mode set to: %s", modeNames[m.mode])
		return nil
	}
	return nil
}

func (m *Model) sendModeChange() {
	if m.midi == nil {
		m.statusMsg = "Not connected"
		return
	}
	midiDev := m.midi
	mode := m.mode
	m.statusMsg = "Sending mode change..."

	go func() {
		cmd := BuildModeChange(mode)
		if err := midiDev.SendSysex(cmd); err != nil {
			m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "ERR: " + err.Error()}
			return
		}
		m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "TX " + hex.EncodeToString(cmd)}
		m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "DONE: mode change sent"}
	}()
}

func (m *Model) sendAllConfig() {
	if m.midi == nil {
		m.statusMsg = "Not connected"
		return
	}

	// Snapshot config and midi pointer to avoid races
	config := m.config
	midiDev := m.midi
	m.statusMsg = "Sending config to device..."

	go func() {
		for _, cmd := range BuildInitSequence() {
			if err := midiDev.SendSysex(cmd); err != nil {
				m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "ERR: " + err.Error()}
				return
			}
			m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "TX " + hex.EncodeToString(cmd)}
			time.Sleep(30 * time.Millisecond)
		}

		for i, cfg := range config {
			cmd := BuildCCConfig(i, byte(cfg.CC), cfg.Latching, byte(cfg.Channel))
			if err := midiDev.SendSysex(cmd); err != nil {
				m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "ERR: " + err.Error()}
				return
			}
			m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "TX " + hex.EncodeToString(cmd)}
			time.Sleep(20 * time.Millisecond)
		}
		m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "DONE: all config sent"}
	}()
}

func (m *Model) requestConfig() {
	if m.midi == nil {
		m.statusMsg = "Not connected"
		return
	}
	midiDev := m.midi
	m.statusMsg = "Reading device config..."

	go func() {
		cmd := BuildReadSettings()
		if err := midiDev.SendSysex(cmd); err != nil {
			m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "ERR: " + err.Error()}
			return
		}
		m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "TX " + hex.EncodeToString(cmd)}
		m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "DONE: read config sent — check log for response"}
	}()
}

func (m *Model) View() string {
	if !m.ready {
		err := ""
		if m.errorMsg != "" {
			err = redStyle.Render("\nError: " + m.errorMsg)
		}
		return fmt.Sprintf("M-Vave Chocolate Config\n\nConnecting to %s...%s\n\nPress q to quit", m.midiPath, err)
	}
	if m.width == 0 {
		return "Loading..."
	}

	var conn string
	if m.connected {
		conn = greenStyle.Render("● Connected")
	} else {
		conn = redStyle.Render("○ Disconnected")
	}

	modeStr := fmt.Sprintf("Mode: %s", modeNames[m.mode])

	var body string
	switch {
	case m.modeSelect:
		body = m.renderModeSelect()
	case m.editSwitch:
		body = m.renderEdit()
	case m.logView:
		body = m.viewport.View()
	default:
		body = lipgloss.JoinVertical(lipgloss.Left,
			modeStr,
			"",
			m.table.View(),
		)
	}

	errStr := ""
	if m.errorMsg != "" {
		errStr = redStyle.Render("Error: "+m.errorMsg) + "\n"
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("M-Vave Chocolate Config")+"  "+conn,
		body,
		statusStyle.Render(m.statusMsg),
		errStr,
		m.help.View(m.keymap),
	)
}

func (m *Model) renderModeSelect() string {
	var lines []string
	lines = append(lines, "Select operating mode:")
	lines = append(lines, "")

	keys := []byte{modeCustom, modeProgramChangeA, modeProgramChangeB, modeProgramChangeC,
		modeKeyboardA, modeKeyboardB, modeMultiMedia, modeTouchScreen,
		modeManufacturer, modeVideo, modeAdvancedCustom, modeCustomKeyboard}

	for _, k := range keys {
		name := modeNames[k]
		if k == m.mode {
			name = highlight.Render("> " + name)
		} else {
			name = "  " + name
		}
		lines = append(lines, name)
	}
	lines = append(lines, "")
	lines = append(lines, "↑↓ to choose, enter to confirm, esc to cancel")
	return strings.Join(lines, "\n")
}

func (m *Model) renderEdit() string {
	bank := m.editIdx/4 + 1
	letter := 'A' + m.editIdx%4

	labels := []string{"Type", "CC/Note", "Channel", "On Value", "Off Value", "Latching"}
	var lines []string
	lines = append(lines, fmt.Sprintf("Editing switch %d%c:", bank, letter))
	lines = append(lines, "")

	for i := range m.fields {
		prefix := "  "
		if i == m.editField {
			prefix = highlight.Render("> ")
		}
		lines = append(lines, fmt.Sprintf("%s%s: %s", prefix, labels[i], m.fields[i].display()))
	}
	lines = append(lines, "")
	lines = append(lines, "tab/↑↓: navigate | enter: save | esc: cancel")
	return strings.Join(lines, "\n")
}

func main() {
	midiPath := defaultMidiDevice
	if len(os.Args) > 1 {
		midiPath = os.Args[1]
	} else {
		if detected, err := findMidiDevice(); err == nil && detected != "" {
			midiPath = detected
			fmt.Fprintf(os.Stderr, "auto-detected: %s\n", detected)
		} else {
			fmt.Fprintf(os.Stderr, "No M-Vave Chocolate found (SINCO/FootCtrl).\n")
			fmt.Fprintf(os.Stderr, "Plug it in and try again, or specify path: mvave-tui /dev/snd/midiC5D0\n")
			os.Exit(1)
		}
	}
	m := NewModel(midiPath)
	p := tea.NewProgram(&m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
