package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mvave-chocolate-tui/internal/sysex"
)

// plusModel is a Model that has read the real pedal's two replies.
func plusModel(t *testing.T) *Model {
	t.Helper()
	m := NewModel("/dev/null")
	m.width, m.height = 100, 40
	m.pui.labels = switchLabels{}
	m.pedalImage = sysex.Image{}
	for _, name := range []string{"plus-read1.syx", "plus-read2.syx"} {
		frame, err := os.ReadFile("internal/sysex/testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		m.parseMidiBytes(frame)
	}
	if m.cfg == nil {
		t.Fatal("fixture did not decode")
	}
	return &m
}

func keys(m *Model, ks ...string) {
	for _, k := range ks {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		m.handlePedalKey(msg)
	}
}

// The change made on the real pedal, through the screen: A, edit, down to
// "2nd press value" (mode; ch, type, #, value; ch, type, #, value), type 4.
func editABankBValueTo4(m *Model) {
	keys(m, "a", "enter", "down", "down", "down", "down", "down", "down", "down", "down", "4", "enter", "esc")
}

func TestEditingStagesOnlyTheChangedByte(t *testing.T) {
	m := plusModel(t)
	m.allowWrite = true
	editABankBValueTo4(m)

	if m.pui.draft == nil || m.pui.draft.Groups[0][0].Slots[1][0].Value != 4 {
		t.Fatalf("draft not edited: %+v", m.pui.draft)
	}
	if m.cfg.Groups[0][0].Slots[1][0].Value != 1 {
		t.Fatal("editing changed the read configuration itself")
	}
	keys(m, "w")
	if !m.pui.confirm || len(m.pui.pending) != 1 || m.pui.pending[0] != (sysex.Write{Addr: 178, Old: 1, New: 4}) {
		t.Fatalf("confirm=%v pending=%+v", m.pui.confirm, m.pui.pending)
	}
	if !strings.Contains(m.viewPedal(), "Write 1 byte(s)") {
		t.Error("confirmation not shown")
	}
	keys(m, "n")
	if m.pui.confirm || m.pui.writing {
		t.Error("cancel did not cancel")
	}
}

func TestReadOnlyRefusesToWrite(t *testing.T) {
	m := plusModel(t)
	editABankBValueTo4(m)
	keys(m, "w")
	if m.pui.confirm || m.pui.writing {
		t.Fatal("asked to write without --write")
	}
	if !strings.Contains(m.statusMsg, "--write") {
		t.Errorf("status %q does not say how to enable writing", m.statusMsg)
	}
}

func TestUndoDropsTheDraft(t *testing.T) {
	m := plusModel(t)
	editABankBValueTo4(m)
	keys(m, "u")
	if m.pui.draft != nil {
		t.Error("undo kept the draft")
	}
}

func TestSwitchModeCyclesThroughAllFive(t *testing.T) {
	m := plusModel(t)
	keys(m, "a", "enter", "+", "esc")
	if got := m.pui.draft.Groups[0][0].Mode; got != 2 {
		t.Fatalf("mode after + is %d, want 2 (press / release)", got)
	}
	keys(m, "enter", "-", "-", "esc")
	if got := m.pui.draft.Groups[0][0].Mode; got != 0 {
		t.Fatalf("mode after two - is %d, want 0 (single tap)", got)
	}
}

func TestAPressLightsTheSwitchThatSentIt(t *testing.T) {
	m := plusModel(t)
	m.parseMidiBytes([]byte{0xB0, 6, 127}) // D long: ch1 CC6 127
	if time.Since(m.pui.flashAt[3]) > time.Second || m.pui.flashSlot[3] != 1 {
		t.Fatalf("D long not lit: %v %d", m.pui.flashAt[3], m.pui.flashSlot[3])
	}
	if !strings.Contains(m.drawLive(), "D  long") {
		t.Errorf("live feed:\n%s", m.drawLive())
	}
	m.parseMidiBytes([]byte{0xB0, 107, 2}) // B
	if !strings.Contains(m.drawLive(), "B  1st press") {
		t.Errorf("live feed:\n%s", m.drawLive())
	}
}

func TestAReadBackThatMatchesIsVerified(t *testing.T) {
	m := plusModel(t)
	// Pretend the pedal already holds what was written: expect == current.
	c := m.cfg.Clone()
	m.pui.expect = &c
	m.pui.writing = true
	m.notePedalRead()
	if m.statusMsg != "Written and verified ✓" || m.pui.writing {
		t.Errorf("status %q writing=%v", m.statusMsg, m.pui.writing)
	}
}

func TestAReadBackThatDiffersIsReported(t *testing.T) {
	m := plusModel(t)
	w := m.cfg.Clone()
	want := &w
	want.Groups[0][0].Slots[1][0].Value = 4 // the pedal still says 1
	m.pui.expect, m.pui.draft, m.pui.writing = want, want, true
	m.notePedalRead()
	if !strings.Contains(m.statusMsg, "does not match") || m.pui.draft == nil {
		t.Errorf("status %q, draft kept=%v", m.statusMsg, m.pui.draft != nil)
	}
}

func TestTheScreenDraws(t *testing.T) {
	m := plusModel(t)
	keys(m, "d") // the detail panel shows the selected switch
	out := m.viewPedal()
	for _, want := range []string{"Chocolate Plus", "Advanced custom mode", "read-only", "step short / step long", "CC107=1"} {
		if !strings.Contains(out, want) && !strings.Contains(m.drawDetail(m.shown()), want) {
			t.Errorf("screen lacks %q", want)
		}
	}
}

func TestNamesAreKept(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	m := plusModel(t)
	keys(m, "d", "n", "N", "e", "x", "t", "enter")
	if m.pui.labels[0][3] != "Next" {
		t.Fatalf("label %q", m.pui.labels[0][3])
	}
	if got := loadLabels()[0][3]; got != "Next" {
		t.Errorf("not saved: %q", got)
	}
}

// The original Chocolate in Advanced Custom mode, built by hand: the layout
// comes from this TUI's own decoder, NOT from a pedal read for this test.
func originalAdvancedImage() sysex.Image {
	img := sysex.Image{}
	for a := 0; a < 0x80; a++ {
		img[a] = 0
	}
	img[0] = sysex.ModeAdvancedCustom
	for i := 0; i < 16; i++ { // switch s, bank b -> slot i = 4b + s
		img[0x30+4*i] = byte(20 + i) // CC
		img[0x31+4*i] = byte(i % 2)  // latching on odd slots
	}
	return img
}

func TestTheOriginalIsRecognisedOnlyInAdvancedCustom(t *testing.T) {
	img := originalAdvancedImage()
	if d := recognise(img); d == nil || d.Product() != "M-VAVE Chocolate" {
		t.Fatalf("recognised as %v", d)
	}
	img[0] = sysex.ModeProgramChangeA
	if d := recognise(img); d != nil {
		t.Errorf("Program Change A recognised as %s; it should fall back", d.Product())
	}
}

func TestTheOriginalDecodesBanksPerSwitch(t *testing.T) {
	cfg := originalDevice{}.Decode(originalAdvancedImage())
	// Switch B (s=1), bank 3 (b=2): slot 4*2+1 = 9 -> CC 29, latching (odd).
	if got := cfg.Groups[0][1].Slots[2][0].Number; got != 29 {
		t.Errorf("B bank 3 CC = %d, want 29", got)
	}
	if got := cfg.Groups[0][1].Flags[2]; got != 1 {
		t.Errorf("B bank 3 latching = %d, want 1", got)
	}
}

// Every change goes to the field and to its live copy 1792 higher, as this
// TUI's BuildAdvCustomCCFull does -- in address terms.
func TestTheOriginalWritesBothCopies(t *testing.T) {
	img := originalAdvancedImage()
	d := originalDevice{}
	want := d.Decode(img)
	want.Groups[0][0].Slots[0][0].Number = 99 // A bank 1: slot 0 at 0x30
	ch, err := d.Changes(img, want)
	if err != nil {
		t.Fatal(err)
	}
	if len(ch) != 2 || ch[0] != (sysex.Write{Addr: 0x30, Old: 20, New: 99}) ||
		ch[1] != (sysex.Write{Addr: 0x30 + 1792, Old: 20, New: 99}) {
		t.Errorf("changes %+v", ch)
	}
	msg, err := sysex.BuildWrite(ch[1].Addr, ch[1].New)
	if err != nil || msg[9] != 0x30 || msg[10] != 0x0E {
		t.Errorf("live copy written as % X (%v); want subcmd 30, byte 10 = 0E", msg, err)
	}
}

func TestTheOriginalScreenSaysItIsUntested(t *testing.T) {
	m := NewModel("/dev/null")
	m.width, m.height = 100, 40
	m.pedalImage = originalAdvancedImage()
	m.notePedalRead()
	if m.cfg == nil {
		t.Fatal("original not shown on the pedal screen")
	}
	if out := m.viewPedal(); !strings.Contains(out, "untested on hardware") || strings.Contains(out, "─E─") {
		t.Errorf("screen:\n%s", out)
	}
}

func TestABackupRestoresAsEditsAndWritesOnlyTheDifference(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := plusModel(t)
	m.allowWrite = true
	m.exportBackup()
	if !strings.Contains(m.statusMsg, "Backed up to") {
		t.Fatalf("export: %s", m.statusMsg)
	}
	// The pedal changes after the backup: A's 2nd press now says 4.
	m.pedalImage[178] = 4
	m.notePedalRead()

	keys(m, "i")
	if !m.pui.picking || len(m.pui.files) != 1 {
		t.Fatalf("picker: picking=%v files=%v", m.pui.picking, m.pui.files)
	}
	keys(m, "enter")
	if m.pui.draft == nil || m.pui.draft.Groups[0][0].Slots[1][0].Value != 1 {
		t.Fatalf("backup not loaded as edits: %s", m.statusMsg)
	}
	keys(m, "w")
	if len(m.pui.pending) != 1 || m.pui.pending[0] != (sysex.Write{Addr: 178, Old: 4, New: 1}) {
		t.Errorf("restore would write %+v; want only 178: 4 -> 1", m.pui.pending)
	}
}

func TestABackupNeedingAnUntestedChangeIsRefused(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := plusModel(t)
	m.exportBackup()
	// The pedal now has a gap in A's first bank: message 3 in use, 2 not --
	// something the official app does not make, and this tool will not edit.
	m.pedalImage[104] = 1
	m.notePedalRead()
	keys(m, "i", "enter")
	if m.pui.draft != nil || !strings.Contains(m.statusMsg, "Cannot restore") {
		t.Errorf("draft=%v status=%q", m.pui.draft != nil, m.statusMsg)
	}
}

func TestCompareListsWhatChangedAndWhatItIs(t *testing.T) {
	m := plusModel(t)
	keys(m, "v")
	m.pedalImage[178] = 4 // as if the official app had changed it, then r
	out := m.drawCompare()
	if !strings.Contains(out, "178") || !strings.Contains(out, "switch A · bank B #1 · value") {
		t.Errorf("compare:\n%s", out)
	}
	if strings.Count(out, "→") != 1 {
		t.Errorf("listed more than the one change:\n%s", out)
	}
	keys(m, "v")
	if m.pui.mark != nil {
		t.Error("v did not stop comparing")
	}
}

func TestPlusDescribesItsBytes(t *testing.T) {
	d := plusDevice{}
	for addr, want := range map[int]string{
		0:    "device mode",
		93:   "switch A · mode",
		94:   "switch A · bank A #1 · in use",
		95:   "switch A · bank A #1 · channel",
		96:   "switch A · bank A #1 · type",
		97:   "switch A · bank A #1 · number",
		99:   "switch A · bank A #2 · in use",
		174:  "switch A · bank B #1 · in use",
		178:  "switch A · bank B #1 · value",
		1348: "switch D · bank A #1 · number",
		1349: "switch D · bank A #1 · value",
		1428: "switch D · bank B #1 · number",
		12:   "not mapped",
	} {
		if got := d.Describe(addr); got != want {
			t.Errorf("%d: %q, want %q", addr, got, want)
		}
	}
}

// An original Chocolate in a mode no Device handles keeps the table screen,
// and that screen still learns the mode -- from the reply at address 0 only.
func TestAnUnhandledModelKeepsTheTableAndItsMode(t *testing.T) {
	m := NewModel("/dev/null")
	m.pedalImage = sysex.Image{}
	for _, name := range []string{"original-read1.syx", "original-read2.syx"} {
		frame, err := os.ReadFile("internal/sysex/testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		m.parseMidiBytes(frame)
	}
	if m.cfg != nil {
		t.Fatalf("recognised as %s; Program Change B has no pedal screen", m.device.Product())
	}
	if !m.modeKnown || m.deviceMode != sysex.ModeProgramChangeB {
		t.Errorf("modeKnown=%v deviceMode=%d, want Program Change B", m.modeKnown, m.deviceMode)
	}
}

// keysOn feeds keys through Update, as the program does, so both the mode
// picker and the screen underneath see them in the right order.
func keysOn(m *Model, ks ...string) tea.Cmd {
	var last tea.Cmd
	for _, k := range ks {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		_, last = m.Update(msg)
	}
	return last
}

func TestModePickerOpensOnTheCurrentMode(t *testing.T) {
	m := plusModel(t)
	dev, _ := openFileDevice(t)
	m.midi = dev
	keysOn(m, "m")
	if !m.mp.open || m.mp.modes[m.mp.pick] != 3 {
		t.Fatalf("open=%v pick=%v", m.mp.open, m.mp.modes)
	}
	if !strings.Contains(m.viewPedal(), "Advanced custom mode  · current") {
		t.Errorf("picker:\n%s", m.drawModePicker())
	}
}

func TestModeChangeNeedsWrite(t *testing.T) {
	m := plusModel(t)
	dev, path := openFileDevice(t)
	m.midi = dev
	keysOn(m, "m", "up", "enter")
	if m.mp.confirm || m.mp.writing || !strings.Contains(m.statusMsg, "--write") {
		t.Fatalf("confirm=%v writing=%v status=%q", m.mp.confirm, m.mp.writing, m.statusMsg)
	}
	assertOnlyReads(t, path)
}

// The change tested on a real Chocolate Plus: Advanced custom (3) to Custom
// control (2) -- one byte at address 0, acknowledged, read back.
func TestAModeChangeWritesAddressZeroAndVerifies(t *testing.T) {
	m := plusModel(t)
	m.allowWrite = true
	dev, path := openFileDevice(t)
	m.midi = dev
	keysOn(m, "m", "up", "enter")
	if !m.mp.confirm || m.mp.target != 2 {
		t.Fatalf("confirm=%v target=%d", m.mp.confirm, m.mp.target)
	}
	cmd := keysOn(m, "y")
	pedalAck <- struct{}{} // the device's OK
	m.Update(cmd())

	sent, _ := os.ReadFile(path)
	want, _ := sysex.BuildWrite(0, 2)
	if !strings.Contains(string(sent), string(want)) {
		t.Fatalf("mode write not sent; device got % X", sent)
	}
	if !m.mp.expect {
		t.Fatal("not waiting to verify the mode")
	}
	// The read-back: the same configuration, mode byte now 2 -- as the real
	// pedal answered.
	img := sysex.Image{}
	for _, name := range []string{"plus-read1.syx", "plus-read2.syx"} {
		frame, _ := os.ReadFile("internal/sysex/testdata/" + name)
		img.Add(frame)
	}
	img[0] = 2
	m.pedalImage = img
	m.notePedalRead()
	if m.statusMsg != "Mode changed to Custom control ✓" || m.cfg == nil {
		t.Errorf("status %q, still on the pedal screen=%v", m.statusMsg, m.cfg != nil)
	}
}

func TestAnOriginalLeavingAdvancedCustomGetsTheTableBack(t *testing.T) {
	m := NewModel("/dev/null")
	m.width, m.height = 100, 40
	m.pedalImage = originalAdvancedImage()
	for a := 0x80; a < 1858; a++ { // a finished read covers both chunks
		m.pedalImage[a] = 0
	}
	m.notePedalRead()
	if m.cfg == nil {
		t.Fatal("Advanced Custom not on the pedal screen")
	}
	m.pedalImage[0] = sysex.ModeProgramChangeA
	m.notePedalRead()
	if m.cfg != nil || m.device != nil || !m.keymap.Edit.Enabled() {
		t.Errorf("still on the pedal screen after leaving Advanced Custom")
	}
}

func TestTheTableScreenOffersTheOriginalsModes(t *testing.T) {
	m := NewModel("/dev/null")
	m.width, m.height = 100, 40
	dev, _ := openFileDevice(t)
	m.midi = dev
	keysOn(&m, "m")
	if !m.mp.open || len(m.mp.modes) != len(sysex.ModeNames) {
		t.Fatalf("open=%v modes=%v", m.mp.open, m.mp.modes)
	}
	if !strings.Contains(m.View(), "Advanced Custom") {
		t.Error("table screen does not show the picker")
	}
}

// n adds a message to the bank under the cursor; writing it sets the in-use
// flag first, exactly as the official app's add did on a real pedal (99).
func TestAddingAMessageThroughTheScreen(t *testing.T) {
	m := plusModel(t)
	m.allowWrite = true
	keys(m, "a", "enter", "down", "n", "esc") // cursor on bank A's channel
	if got := len(m.pui.draft.Groups[0][0].Slots[0]); got != 2 {
		t.Fatalf("bank A holds %d messages, want 2", got)
	}
	keys(m, "w")
	if !m.pui.confirm || len(m.pui.pending) == 0 || m.pui.pending[0] != (sysex.Write{Addr: 99, Old: 0, New: 1}) {
		t.Fatalf("pending %+v; want the in-use flag at 99 first", m.pui.pending)
	}
}

// x removes the LAST message of the bank under the cursor.
func TestRemovingAMessageThroughTheScreen(t *testing.T) {
	m := plusModel(t)
	m.allowWrite = true
	keys(m, "a", "enter", "down", "x", "esc")
	if got := len(m.pui.draft.Groups[0][0].Slots[0]); got != 0 {
		t.Fatalf("bank A holds %d messages, want 0", got)
	}
	keys(m, "w")
	n := len(m.pui.pending)
	if n == 0 || m.pui.pending[n-1] != (sysex.Write{Addr: 94, Old: 1, New: 0}) {
		t.Errorf("pending %+v; want the in-use flag at 94 cleared last", m.pui.pending)
	}
}

func TestChangingAMessageType(t *testing.T) {
	m := plusModel(t)
	keys(m, "a", "enter", "down", "down", "-", "esc") // CC -> PC
	if got := m.pui.draft.Groups[0][0].Slots[0][0].Type; got != MsgPC {
		t.Fatalf("type %d, want PC", got)
	}
	if !strings.Contains(m.drawDetail(m.shown()), "PC") {
		t.Errorf("detail:\n%s", m.drawDetail(m.shown()))
	}
}

func TestAProgramChangePressIsTraced(t *testing.T) {
	m := plusModel(t)
	c := m.cfg.Clone()
	c.Groups[0][2].Slots[0][0] = Message{Type: MsgPC, Channel: 1, Number: 5}
	m.cfg = &c
	m.parseMidiBytes([]byte{0xC0, 5})
	if !strings.Contains(m.drawLive(), "C  1st press ch1 PC5") {
		t.Errorf("live feed:\n%s", m.drawLive())
	}
}

// Drawing a switch with a message that exists only in the draft crashed: the
// "was" lookup indexed the pedal's shorter bank.
func TestANewMessageDrawsAsNew(t *testing.T) {
	m := plusModel(t)
	keys(m, "a", "enter", "down", "n")
	out := m.drawDetail(m.shown())
	if !strings.Contains(out, "(new)") {
		t.Errorf("detail:\n%s", out)
	}
}

// fullModel is a Model that has read all eight groups of a real Chocolate
// Plus (the official app's full read, recorded from its USB traffic): two
// groups in use, the pedal on group 1, group 2 the app's defaults with switch
// A set to PC 1.
func fullModel(t *testing.T) *Model {
	t.Helper()
	data, err := os.ReadFile("internal/sysex/testdata/plus-full.syx")
	if err != nil {
		t.Fatal(err)
	}
	m := NewModel("/dev/null")
	m.width, m.height = 110, 50
	m.pui.labels = switchLabels{}
	m.pedalImage = sysex.Image{}
	m.parseMidiBytes(data)
	if m.cfg == nil || len(m.cfg.Groups) != 8 {
		t.Fatalf("full read not decoded: %v", m.statusMsg)
	}
	return &m
}

func TestPagingToGroup2ShowsItsSwitches(t *testing.T) {
	m := fullModel(t)
	if !strings.Contains(m.groupLine(*m.cfg), "group 1/8") || !strings.Contains(m.groupLine(*m.cfg), "2 in use") ||
		!strings.Contains(m.groupLine(*m.cfg), "pedal on 1") {
		t.Errorf("header: %q", m.groupLine(*m.cfg))
	}
	keys(m, "]")
	if m.pui.group != 1 {
		t.Fatalf("viewing group %d", m.pui.group+1)
	}
	if out := m.drawDetail(m.shown()); !strings.Contains(out, "single tap") || !strings.Contains(out, "PC") {
		t.Errorf("group 2 detail:\n%s", out)
	}
	keys(m, "[", "[")
	if m.pui.group != 0 {
		t.Errorf("[ went below group 1: %d", m.pui.group)
	}
}

func TestEditingGroup2WritesOnlyThere(t *testing.T) {
	m := fullModel(t)
	m.allowWrite = true
	keys(m, "]", "a", "enter", "+", "esc") // switch A of group 2: single tap -> single step
	keys(m, "w")
	if !m.pui.confirm || len(m.pui.pending) != 1 || m.pui.pending[0] != (sysex.Write{Addr: 1761, Old: 0, New: 1}) {
		t.Fatalf("pending %+v; want one write, at 1761", m.pui.pending)
	}
}

func TestGroupCountAndPedalGroupGoThroughWrite(t *testing.T) {
	m := fullModel(t)
	m.allowWrite = true
	keys(m, "+")
	if m.pui.draft.GroupCount != 3 || !strings.Contains(m.groupLine(m.shown()), "3 in use") {
		t.Fatalf("count %d, header %q", m.pui.draft.GroupCount, m.groupLine(m.shown()))
	}
	keys(m, "]", "p")
	if m.pui.draft.CurrentGroup != 1 {
		t.Fatalf("pedal group %d", m.pui.draft.CurrentGroup+1)
	}
	keys(m, "w")
	want := []sysex.Write{{Addr: 23639, Old: 1, New: 2}, {Addr: 23640, Old: 0, New: 1}}
	if len(m.pui.pending) != 2 || m.pui.pending[0] != want[0] || m.pui.pending[1] != want[1] {
		t.Errorf("pending %+v; want %+v", m.pui.pending, want)
	}
}

func TestTheGroupCountStaysWithinWhatExists(t *testing.T) {
	m := fullModel(t)
	keys(m, "-")
	if m.pui.draft.GroupCount != 1 {
		t.Fatalf("count %d", m.pui.draft.GroupCount)
	}
	keys(m, "-")
	if m.pui.draft.GroupCount != 1 {
		t.Errorf("went below one group: %d", m.pui.draft.GroupCount)
	}
	keys(m, "]", "p") // group 2 is beyond the one group now in use
	if m.pui.draft.CurrentGroup != 0 {
		t.Errorf("pedal moved to a group beyond the count: %d", m.pui.draft.CurrentGroup+1)
	}
}

// A press from a group the screen is not showing still names its group, and
// the header follows the pedal there -- without touching the read config.
func TestAPressFromAnotherGroupIsTraced(t *testing.T) {
	m := fullModel(t)
	m.parseMidiBytes([]byte{0xC0, 1}) // group 2 switch A: PC 1, channel 1
	if !strings.Contains(m.drawLive(), "2A") {
		t.Errorf("live:\n%s", m.drawLive())
	}
	if !strings.Contains(m.groupLine(*m.cfg), "pedal on 2") || m.cfg.CurrentGroup != 0 {
		t.Errorf("header %q, read current group %d", m.groupLine(*m.cfg), m.cfg.CurrentGroup)
	}
}

// Naming group 2's A must not rename group 1's A -- the stage preset.
func TestNamesBelongToTheirGroup(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := fullModel(t)
	keys(m, "]", "a", "n", "P", "l", "a", "y", "enter")
	if m.pui.labels[1][0] != "Play" || m.pui.labels[0][0] != "" {
		t.Errorf("labels group 1 A %q, group 2 A %q", m.pui.labels[0][0], m.pui.labels[1][0])
	}
}

// Names saved before groups were four, one per switch; they become group 1's.
func TestOldNamesBecomeGroup1s(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = os.MkdirAll(filepath.Dir(labelsPath()), 0o755)
	_ = os.WriteFile(labelsPath(), []byte(`["Clean","Drive","Fuzz","Next"]`), 0o644)
	l := loadLabels()
	if l[0] != [4]string{"Clean", "Drive", "Fuzz", "Next"} || l[1] != [4]string{} {
		t.Errorf("loaded %v", l)
	}
}
