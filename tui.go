// Copyright (C) 2026 Wilson Neto
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mvave-chocolate-tui/internal/detect"
	"mvave-chocolate-tui/internal/midi"
	"mvave-chocolate-tui/internal/sysex"
)

const defaultMidiDevice = "/dev/snd/midiC5D0"

// SwitchConfig holds the Advanced Custom configuration for one footswitch.
type SwitchConfig struct {
	CC       int
	Latching bool
}

type SwitchState int

const (
	StateReleased SwitchState = iota
	StatePressed
)

type MidiMsg struct {
	Timestamp time.Time
	Hex       string
}

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

type midiConnectedMsg struct {
	dev *midi.Device
	err error
}

// deviceReadyMsg is posted after the on-connect auto-configure completes.
type deviceReadyMsg struct {
	deviceMode byte
	err        string
}

type Model struct {
	midiPath   string
	midi       *midi.Device
	midiMsgs   chan MidiMsg
	ready      bool
	connected  bool
	deviceMode byte // last mode reported by device
	log        []MidiMsg
	statusMsg  string
	errorMsg   string
	width      int
	height     int
	logView    bool
	editSwitch bool
	editIdx    int
	editField  int

	config  [16]SwitchConfig
	swState [16]SwitchState

	table    table.Model
	viewport viewport.Model
	help     help.Model
	keymap   keyMap

	fields [2]textField // CC, Latch
}

type keyMap struct {
	Quit      key.Binding
	Tab       key.Binding
	Edit      key.Binding
	Send      key.Binding
	Read      key.Binding
	LogToggle key.Binding
	Up        key.Binding
	Down      key.Binding
	Confirm   key.Binding
	Cancel    key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Edit, k.Send, k.Read, k.Tab, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Edit, k.Send, k.Read},
		{k.Tab, k.Up, k.Down, k.Quit},
	}
}

var defaultKeymap = keyMap{
	Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Tab:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "log")),
	Edit:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Send:      key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "send")),
	Read:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "read")),
	LogToggle: key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "log")),
	Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
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

// --- Config file persistence ---

type savedConfig struct {
	Switches [16]struct {
		CC       int  `json:"cc"`
		Latching bool `json:"latching"`
	} `json:"switches"`
}

func configFilePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.Getenv("HOME")
	}
	return filepath.Join(dir, "mvave-chocolate-tui", "config.json")
}

func defaultSwitchConfig() [16]SwitchConfig {
	var cfg [16]SwitchConfig
	for i := range cfg {
		cfg[i] = SwitchConfig{CC: i + 1} // CC 1–16 as defaults
	}
	return cfg
}

func loadConfig() [16]SwitchConfig {
	cfg := defaultSwitchConfig()
	data, err := os.ReadFile(configFilePath())
	if err != nil {
		return cfg
	}
	var saved savedConfig
	if err := json.Unmarshal(data, &saved); err != nil {
		return cfg
	}
	for i, sw := range saved.Switches {
		if sw.CC >= 0 && sw.CC <= 127 {
			cfg[i].CC = sw.CC
		}
		cfg[i].Latching = sw.Latching
	}
	return cfg
}

func saveConfig(config [16]SwitchConfig) {
	var saved savedConfig
	for i, cfg := range config {
		saved.Switches[i].CC = cfg.CC
		saved.Switches[i].Latching = cfg.Latching
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(configFilePath()), 0755)
	_ = os.WriteFile(configFilePath(), data, 0644)
}

// --- Model ---

func NewModel(midiPath string) Model {
	cfg := loadConfig()

	cols := []table.Column{
		{Title: "Switch", Width: 8},
		{Title: "CC", Width: 6},
		{Title: "Latch", Width: 6},
		{Title: "State", Width: 6},
	}

	rows := make([]table.Row, 16)
	for i := 0; i < 16; i++ {
		rows[i] = makeSwitchRow(i, cfg[i], StateReleased)
	}

	t := table.New(table.WithColumns(cols), table.WithRows(rows), table.WithHeight(18))
	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).BorderBottom(true).Bold(true)
	s.Selected = s.Selected.Foreground(lipgloss.Color("0")).Background(lipgloss.Color("212"))
	t.SetStyles(s)

	vp := viewport.New(80, 10)
	vp.SetContent("MIDI log...")

	return Model{
		midiPath:  midiPath,
		midiMsgs:  make(chan MidiMsg, 256),
		config:    cfg,
		table:     t,
		viewport:  vp,
		help:      help.New(),
		keymap:    defaultKeymap,
		fields:    [2]textField{newTextField("0-127"), newTextField("yes/no")},
		statusMsg: "Starting...",
	}
}

func makeSwitchRow(i int, cfg SwitchConfig, st SwitchState) table.Row {
	bank := i/4 + 1
	sw := i % 4
	label := fmt.Sprintf("%d%c", bank, 'A'+sw)
	state := "⬆"
	if st == StatePressed {
		state = "⬇"
	}
	latch := "no"
	if cfg.Latching {
		latch = "yes"
	}
	return table.Row{label, fmt.Sprintf("%d", cfg.CC), latch, state}
}

func (m *Model) refreshTable() {
	rows := make([]table.Row, 16)
	for i := 0; i < 16; i++ {
		rows[i] = makeSwitchRow(i, m.config[i], m.swState[i])
	}
	m.table.SetRows(rows)
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), connectMidiCmd(m.midiPath))
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func connectMidiCmd(path string) tea.Cmd {
	return func() tea.Msg {
		dev, err := midi.Open(path)
		return midiConnectedMsg{dev: dev, err: err}
	}
}

// autoConfigureCmd sets AdvancedCustom mode on the device and applies saved config.
// Runs as a Cmd so it returns a deviceReadyMsg when done.
func autoConfigureCmd(dev *midi.Device, config [16]SwitchConfig) tea.Cmd {
	return func() tea.Msg {
		// 1. Set Advanced Custom mode.
		modeCmd := sysex.BuildModeChange(sysex.ModeAdvancedCustom)
		if err := dev.SendSysex(modeCmd); err != nil {
			return deviceReadyMsg{err: err.Error()}
		}
		time.Sleep(50 * time.Millisecond)

		// 2. Write CC config for all 16 switches (RAM write only — byte[10]=0x0E).
		for i, cfg := range config {
			for _, msg := range sysex.SplitMessages(sysex.BuildAdvCustomCC(i, byte(cfg.CC), cfg.Latching, 0)) {
				if err := dev.SendSysex(msg); err != nil {
					return deviceReadyMsg{err: err.Error()}
				}
				time.Sleep(20 * time.Millisecond)
			}
		}

		// 3. Read back mode to confirm.
		raw, _ := dev.SendReceiveSysex(sysex.BuildReadSettings(), 3)
		deviceMode := sysex.ModeAdvancedCustom
		if len(raw) > 0 {
			parsed := parseHexOutput(raw)
			frames := sysex.FindSysExFrames(parsed, []byte{0xF0, 0x00, 0x32, 0x0D, 0x49})
			for _, f := range frames {
				if cr := sysex.ParseConfigResponse(f); cr != nil {
					deviceMode = cr.Mode
				}
			}
		}
		return deviceReadyMsg{deviceMode: deviceMode}
	}
}

// parseHexOutput converts amidi -d output (space-separated hex) to bytes.
func parseHexOutput(raw []byte) []byte {
	clean := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			return r
		}
		return -1
	}, string(raw))
	data, _ := hex.DecodeString(clean)
	return data
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Drain ALL pending MIDI messages on every Update call.
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
			m.statusMsg = "Connected — setting Advanced Custom mode..."
			cmds = append(cmds, m.midiReadLoop())
			cmds = append(cmds, autoConfigureCmd(msg.dev, m.config))
		}
		return m, tea.Batch(cmds...)

	case deviceReadyMsg:
		if msg.err != "" {
			m.statusMsg = "Config error: " + msg.err
		} else {
			m.deviceMode = msg.deviceMode
			if msg.deviceMode == sysex.ModeAdvancedCustom {
				m.statusMsg = "Advanced Custom active — config applied"
			} else {
				m.statusMsg = fmt.Sprintf("Warning: device mode=%02X, expected Advanced Custom", msg.deviceMode)
			}
		}

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
		if m.editSwitch {
			cmds = append(cmds, m.handleEdit(msg))
			m.refreshTable()
			return m, tea.Batch(cmds...)
		}

		switch {
		case key.Matches(msg, m.keymap.Quit):
			if m.midi != nil {
				_ = m.midi.Close()
				m.midi = nil
			}
			return m, tea.Quit
		case key.Matches(msg, m.keymap.Tab), key.Matches(msg, m.keymap.LogToggle):
			m.logView = !m.logView
		case key.Matches(msg, m.keymap.Edit):
			m.startEdit(m.table.Cursor())
		case key.Matches(msg, m.keymap.Read):
			m.requestConfig()
		case key.Matches(msg, m.keymap.Send):
			m.sendAllConfig()
		case key.Matches(msg, m.keymap.Up):
			m.table.MoveUp(1)
		case key.Matches(msg, m.keymap.Down):
			m.table.MoveDown(1)
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
		buf := make([]byte, 4096)
		for {
			if m.midi == nil {
				return nil
			}
			n, err := m.midi.Read(buf)
			if err != nil {
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
		m.statusMsg = strings.TrimPrefix(msg.Hex, "DONE: ")
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
			frame := data[i : end+1]
			m.log = append(m.log, MidiMsg{Timestamp: time.Now(), Hex: "SYX " + hex.EncodeToString(frame)})
			if sysex.IsOKResponse(frame) {
				m.log = append(m.log, MidiMsg{Timestamp: time.Now(), Hex: "OK — config write accepted"})
			}
			if cr := sysex.ParseConfigResponse(frame); cr != nil {
				// Only update UI mode display; never overwrite config from dump
				// (AdvCustom CC is not stored in 0D41 dump).
				m.deviceMode = cr.Mode
			}
			i = end + 1
		default:
			i++
		}
	}
}

func (m *Model) setSwitchByCC(cc int, pressed bool) {
	for i, cfg := range m.config {
		if cfg.CC == cc {
			if pressed {
				m.swState[i] = StatePressed
			} else {
				m.swState[i] = StateReleased
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
	m.fields[0].set(strconv.Itoa(cfg.CC))
	if cfg.Latching {
		m.fields[1].set("yes")
	} else {
		m.fields[1].set("no")
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
	case "tab", "down":
		m.editField = (m.editField + 1) % len(m.fields)
		return nil
	case "shift+tab", "up":
		m.editField = (m.editField - 1 + len(m.fields)) % len(m.fields)
		return nil
	case "enter":
		m.applyEdit()
		m.editSwitch = false
		m.statusMsg = "Switch updated — press 's' to send to device"
		return nil
	case "left":
		m.fields[m.editField].left()
	case "right":
		m.fields[m.editField].right()
	case "backspace":
		m.fields[m.editField].backspace()
	case "delete":
		m.fields[m.editField].deleteForward()
	default:
		if len(msg.Runes) == 1 {
			r := msg.Runes[0]
			if r >= 32 && r < 127 {
				m.fields[m.editField].insert(r)
			}
		}
	}
	return nil
}

func (m *Model) applyEdit() {
	cfg := &m.config[m.editIdx]
	if v, err := strconv.Atoi(m.fields[0].value); err == nil && v >= 0 && v <= 127 {
		cfg.CC = v
	}
	cfg.Latching = strings.ToLower(m.fields[1].value) == "yes" || m.fields[1].value == "1"
}

// sendAllConfig writes the full Advanced Custom config to the device:
// 1. Set mode to Advanced Custom.
// 2. For each switch: BuildAdvCustomCC (RAM write, byte[10]=0x0E).
// No init sequence — TestHeadlessAdvCustomCC48 confirmed it's not needed.
// No flash write — flash encoding is not linear (CC=99 stores as 0x40/0x31, not 99).
func (m *Model) sendAllConfig() {
	if m.midi == nil {
		m.statusMsg = "Not connected"
		return
	}
	config := m.config
	midiDev := m.midi
	m.statusMsg = "Sending Advanced Custom config..."

	go func() {
		modeCmd := sysex.BuildModeChange(sysex.ModeAdvancedCustom)
		if err := midiDev.SendSysex(modeCmd); err != nil {
			m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "ERR: " + err.Error()}
			return
		}
		m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "TX " + hex.EncodeToString(modeCmd)}
		time.Sleep(50 * time.Millisecond)

		for i, cfg := range config {
			for _, cmd := range sysex.SplitMessages(sysex.BuildAdvCustomCC(i, byte(cfg.CC), cfg.Latching, 0)) {
				if err := midiDev.SendSysex(cmd); err != nil {
					m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "ERR: " + err.Error()}
					return
				}
				m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "TX " + hex.EncodeToString(cmd)}
				time.Sleep(20 * time.Millisecond)
			}
		}

		saveConfig(config)
		m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "DONE: all config sent and saved"}
	}()
}

// requestConfig reads the device mode via combined amidi send+receive.
func (m *Model) requestConfig() {
	if m.midi == nil {
		m.statusMsg = "Not connected"
		return
	}
	dev := m.midi
	m.statusMsg = "Reading device mode..."

	go func() {
		raw, err := dev.SendReceiveSysex(sysex.BuildReadSettings(), 3)
		if err != nil {
			m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "ERR: read: " + err.Error()}
			return
		}
		parsed := parseHexOutput(raw)
		frames := sysex.FindSysExFrames(parsed, []byte{0xF0, 0x00, 0x32, 0x0D, 0x49})
		if len(frames) == 0 {
			m.midiMsgs <- MidiMsg{Timestamp: time.Now(), Hex: "DONE: no config frame in response"}
			return
		}
		for _, f := range frames {
			if cr := sysex.ParseConfigResponse(f); cr != nil {
				name := sysex.ModeNames[cr.Mode]
				m.midiMsgs <- MidiMsg{Timestamp: time.Now(),
					Hex: fmt.Sprintf("DONE: device mode=0x%02X (%s)", cr.Mode, name)}
			}
		}
	}()
}

func (m *Model) View() string {
	if !m.ready {
		err := ""
		if m.errorMsg != "" {
			err = redStyle.Render("\nError: " + m.errorMsg)
		}
		return fmt.Sprintf("M-Vave Chocolate — Advanced Custom\n\nConnecting to %s...%s\n\nPress q to quit", m.midiPath, err)
	}
	if m.width == 0 {
		return "Loading..."
	}

	connStr := redStyle.Render("○ Disconnected")
	if m.connected {
		modeOK := m.deviceMode == sysex.ModeAdvancedCustom || m.deviceMode == 0
		if modeOK {
			connStr = greenStyle.Render("● Advanced Custom")
		} else {
			connStr = greenStyle.Render("● Connected") + " " +
				redStyle.Render(fmt.Sprintf("[device mode=%02X?]", m.deviceMode))
		}
	}

	var body string
	switch {
	case m.editSwitch:
		body = m.renderEdit()
	case m.logView:
		body = m.viewport.View()
	default:
		body = m.table.View()
	}

	errStr := ""
	if m.errorMsg != "" {
		errStr = redStyle.Render("Error: "+m.errorMsg) + "\n"
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("M-Vave Chocolate")+"  "+connStr,
		"",
		body,
		statusStyle.Render(m.statusMsg),
		errStr,
		m.help.View(m.keymap),
	)
}

func (m *Model) renderEdit() string {
	bank := m.editIdx/4 + 1
	letter := 'A' + m.editIdx%4
	labels := []string{"CC (0-127)", "Latch (yes/no)"}
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
	lines = append(lines, "tab/↑↓: navigate  enter: save  esc: cancel")
	return strings.Join(lines, "\n")
}

func main() {
	midiPath := defaultMidiDevice
	if len(os.Args) > 1 {
		midiPath = os.Args[1]
	} else {
		if detected, err := detect.Find(); err == nil && detected != "" {
			midiPath = detected
			fmt.Fprintf(os.Stderr, "auto-detected: %s\n", detected)
		} else {
			fmt.Fprintf(os.Stderr, "No M-Vave Chocolate found (SINCO/FootCtrl).\n")
			fmt.Fprintf(os.Stderr, "Plug in with side switch on U, or specify path: mvave-chocolate-tui /dev/snd/midiC5D0\n")
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
