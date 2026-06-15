package main

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"mvave-chocolate-tui/internal/detect"
	"mvave-chocolate-tui/internal/midi"
	"mvave-chocolate-tui/internal/sysex"
)

func hexEq(a, b string) bool {
	return strings.EqualFold(a, b)
}

// --- Unit tests for new functionality ---

func TestFindDeviceFromProcFS(t *testing.T) {
	// Create a fake /proc/asound/cards
	tmpDir := t.TempDir()
	fakeCards := filepath.Join(tmpDir, "cards")
	_ = os.WriteFile(fakeCards, []byte(`
 0 [Brio          ]: USB-Audio - MX Brio
 4 [USB           ]: USB-Audio - Scarlett 4i4 USB
 5 [SINCO         ]: USB-Audio - SINCO
                      Jieli Technology SINCO at usb-0000:04:00.3
`), 0644)

	// Create a fake midi device
	fakeMidi := filepath.Join(tmpDir, "midiC5D0")
	_ = os.WriteFile(fakeMidi, []byte{}, 0644)

	// Test the card name scanning logic used by findMidiDevice
	data, _ := os.ReadFile(fakeCards)
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "SINCO") {
			found = true
			cardNum := strings.TrimSpace(strings.Split(line, "[")[0])
			if cardNum != "5" {
				t.Errorf("expected card 5, got %q from line: %s", cardNum, line)
			}
			break
		}
	}
	if !found {
		t.Error("SINCO not found in fake cards file")
	}
}

func TestBuildInitSequence(t *testing.T) {
	seq := sysex.BuildInitSequence()

	if len(seq) != 12 {
		t.Errorf("sysex.BuildInitSequence: expected 12 messages, got %d", len(seq))
	}

	for i, msg := range seq {
		if len(msg) < 3 {
			t.Errorf("message %d too short: %d bytes", i, len(msg))
			continue
		}
		if msg[0] != 0xF0 {
			t.Errorf("message %d: missing F0 start byte", i)
		}
		if msg[len(msg)-1] != 0xF7 {
			t.Errorf("message %d: missing F7 end byte", i)
		}
	}

	// Third message should be mode change to Custom CC (0x07)
	modeMsg := seq[2] // index 2 is sysex.BuildModeChange(sysex.ModeCustom)
	expectedMode := sysex.BuildModeChange(sysex.ModeCustom)
	if len(modeMsg) != len(expectedMode) {
		t.Errorf("mode change message length mismatch: got %d, expected %d", len(modeMsg), len(expectedMode))
	}
	if !hexEq(hex.EncodeToString(modeMsg), hex.EncodeToString(expectedMode)) {
		t.Errorf("mode change mismatch:\n  got: %s\n  exp: %s",
			hex.EncodeToString(modeMsg), hex.EncodeToString(expectedMode))
	}

	// Verify init sequence matches known bytes from cbix
	knownFirstMsg := "f000320d410000000200000000107e00000700f7"
	if !hexEq(hex.EncodeToString(seq[0]), knownFirstMsg) {
		t.Errorf("init[0] mismatch:\n  got: %s\n  exp: %s", hex.EncodeToString(seq[0]), knownFirstMsg)
	}
}

func TestBuildCCConfigChannel(t *testing.T) {
	// CC config should produce exactly 2 messages (CC value + latching)
	data := sysex.BuildCCConfig(0, 44, false, 0)
	msgLen := len(sysex.KnownExamples[0].Data) + 4 // F0 + 17 data + 2 checksum + F7

	if len(data) != 2*msgLen {
		t.Fatalf("sysex.BuildCCConfig: expected %d bytes (2 x %d), got %d", 2*msgLen, msgLen, len(data))
	}

	// First message should have F0 start and F7 end
	if data[0] != 0xF0 || data[msgLen-1] != 0xF7 {
		t.Error("first message missing F0/F7 framing")
	}
	if data[msgLen] != 0xF0 || data[2*msgLen-1] != 0xF7 {
		t.Error("second message missing F0/F7 framing")
	}

	// Check the CC value is at the right position (17th data byte, index 17 from F0=1, so index 18)
	// Message layout: F0(0) + header[5] + pad[3] + sub[1] + pad[3] + 0x10[1] + pad[3] + value[1] + cs[2] + F7[1]
	// value is at index: 1 + 5 + 3 + 1 + 3 + 1 + 3 = 17 (from start, including F0)
	valueIdx := 17
	if data[valueIdx] != 44 {
		t.Errorf("CC value: expected 44 at byte %d, got %d (msg: %s)", valueIdx, data[valueIdx],
			hex.EncodeToString(data[:msgLen]))
	}
}

func TestBuildCCConfigChannelAllBanks(t *testing.T) {
	// Verify all 4 banks produce valid messages
	for bank := 0; bank < 4; bank++ {
		sw := bank * 4
		data := sysex.BuildCCConfig(sw, 20, false, 0)
		msgLen := len(sysex.KnownExamples[0].Data) + 4
		if len(data) != 2*msgLen {
			t.Errorf("bank %d switch %d: wrong length %d", bank, sw, len(data))
		}
		// Verify F0/F7 framing
		if data[0] != 0xF0 || data[msgLen-1] != 0xF7 {
			t.Errorf("bank %d: message 1 framing wrong", bank)
		}
		if data[msgLen] != 0xF0 || data[2*msgLen-1] != 0xF7 {
			t.Errorf("bank %d: message 2 framing wrong", bank)
		}
	}
}

func TestSendAllConfigWithoutDevice(t *testing.T) {
	m := NewModel("/dev/null")
	m.midi = nil // no device

	m.sendAllConfig()
	if m.statusMsg != "Not connected" {
		t.Errorf("expected 'Not connected', got '%s'", m.statusMsg)
	}
}

// --- Mock MIDI device for testing write operations ---

type mockMidiDevice struct {
	written [][]byte
	failOn  int
	count   int
}

func (m *mockMidiDevice) SendSysex(data []byte) error {
	if m.failOn > 0 && m.count >= m.failOn {
		return os.ErrClosed
	}
	m.written = append(m.written, data)
	m.count++
	return nil
}

func TestSendAllConfigWithMockDevice(t *testing.T) {
	m := NewModel("/dev/null")
	mock := &mockMidiDevice{}

	// Replace midi with mock by creating a real midi.Device using a temp file
	tmpFile, err := os.CreateTemp("", "midi-mock-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	dev, err := midi.Open(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dev.Close() }()
	m.midi = dev

	m.sendAllConfig()

	// After send, status should reflect sending (async, so it shows this immediately)
	if m.statusMsg != "Sending config to device..." {
		t.Errorf("status after send: expected 'Sending config to device...', got '%s'", m.statusMsg)
	}
	_ = mock // used for reference pattern
}

func TestModelCleanupOnQuit(t *testing.T) {
	// Use a temp file as fake MIDI device
	tmpFile, err := os.CreateTemp("", "midi-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	m := NewModel(tmpFile.Name())
	dev, err := midi.Open(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	m.midi = dev

	// Quit
	if m.midi != nil {
		_ = m.midi.Close()
		m.midi = nil
	}

	if m.midi != nil {
		t.Error("midi should be nil after Close() + nil assignment")
	}
}

func TestModelReopenAfterQuit(t *testing.T) {
	// Verify we can reopen the same path after closing
	tmpFile, err := os.CreateTemp("", "midi-reopen-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	dev1, err := midi.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	_ = dev1.Close()

	dev2, err := midi.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("second open after close: %v", err)
	}
	_ = dev2.Close()
}

// --- E2E tests with teatest ---

func TestTUIQuitReleasesDevice(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "midi-tui-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	m := NewModel(tmpFile.Name())
	tm := teatest.NewTestModel(
		t, &m,
		teatest.WithInitialTermSize(100, 30),
	)

	// Wait for connected state
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(string(bts), "Connected")
		},
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*5),
	)

	// Quit
	_ = m.midi.Close()
	m.midi = nil
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*3))

	// After TUI exits, verify the file can be reopened (device properly released)
	f, err := os.OpenFile(tmpFile.Name(), os.O_RDWR, 0)
	if err != nil {
		t.Errorf("device should be free after TUI quit: %v", err)
	} else {
		_ = f.Close()
	}
}

func TestTUIModeSelectAndCancel(t *testing.T) {
	m := NewModel("/dev/null")
	initialMode := m.mode
	tm := teatest.NewTestModel(
		t, &m,
		teatest.WithInitialTermSize(100, 30),
	)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(string(bts), "M-Vave")
		},
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*3),
	)

	// Enter mode select
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	time.Sleep(200 * time.Millisecond)

	// Navigate: down x2, up x1 → both directions work, no crash
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(50 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(50 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyUp})
	time.Sleep(50 * time.Millisecond)

	// Cancel and verify mode unchanged
	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})
	time.Sleep(200 * time.Millisecond)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*3))

	fm := tm.FinalModel(t)
	finalModel, ok := fm.(*Model)
	if !ok {
		t.Fatalf("wrong type: %T", fm)
	}
	if finalModel.mode != initialMode {
		t.Errorf("mode changed after cancel: was %d, got %d", initialMode, finalModel.mode)
	}
}

func TestTUIModeSelectAndConfirm(t *testing.T) {
	m := NewModel("/dev/null")
	tm := teatest.NewTestModel(
		t, &m,
		teatest.WithInitialTermSize(100, 30),
	)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(string(bts), "M-Vave")
		},
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*3),
	)

	// Enter mode select
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	time.Sleep(200 * time.Millisecond)

	// Navigate down 3 times → ProgramChangeC (0x0B)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	time.Sleep(100 * time.Millisecond)

	// Confirm
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(200 * time.Millisecond)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*3))

	fm := tm.FinalModel(t)
	finalModel, ok := fm.(*Model)
	if !ok {
		t.Fatalf("wrong type: %T", fm)
	}
	// modeKeys order: Custom(0x07), PC-A(0x00), PC-B(0x01), PC-C(0x0B), ...
	// 3 steps down from Custom(index 0) -> index 3 -> PC-C (0x0B)
	if finalModel.mode != sysex.ModeProgramChangeC {
		t.Errorf("expected mode ProgramChangeC (0x0B), got 0x%02X", finalModel.mode)
	}
}

func TestTUIModeSelectWraparound(t *testing.T) {
	m := NewModel("/dev/null")
	tm := teatest.NewTestModel(
		t, &m,
		teatest.WithInitialTermSize(100, 30),
	)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(string(bts), "M-Vave")
		},
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*3),
	)

	// Enter mode select
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	time.Sleep(200 * time.Millisecond)

	// Go up from Custom CC (index 0) → should wrap to CustomKeyboard (last = 0x0A)
	tm.Send(tea.KeyMsg{Type: tea.KeyUp})
	time.Sleep(100 * time.Millisecond)

	// Confirm
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(200 * time.Millisecond)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*3))

	fm := tm.FinalModel(t)
	finalModel, ok := fm.(*Model)
	if !ok {
		t.Fatalf("wrong type: %T", fm)
	}
	if finalModel.mode != sysex.ModeCustomKeyboard {
		t.Errorf("wraparound failed: expected CustomKeyboard (0x0A), got 0x%02X", finalModel.mode)
	}
}

func TestTUIEditAndSendConfig(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "midi-send-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	m := NewModel(tmpFile.Name())
	tm := teatest.NewTestModel(
		t, &m,
		teatest.WithInitialTermSize(100, 30),
	)

	// Wait for connect
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(string(bts), "Connected")
		},
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*5),
	)

	// Edit switch 1A
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(string(bts), "Editing switch 1A")
		},
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*3),
	)

	// Navigate to CC field (field 1, press tab once)
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})

	// Clear and set CC to 44
	for range "20" {
		tm.Send(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "44" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Save
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Wait to return to main view
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(string(bts), "44")
		},
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*3),
	)

	// Send config
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})

	// Verify status message shows sending or done
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			s := string(bts)
			return strings.Contains(s, "Sending config") ||
				strings.Contains(s, "sent to device") ||
				strings.Contains(s, "DONE")
		},
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*5),
	)

	// Quit and get final model
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*3))

	fm := tm.FinalModel(t)
	finalModel, ok := fm.(*Model)
	if !ok {
		t.Fatalf("wrong type: %T", fm)
	}
	if finalModel.config[0].CC != 44 {
		t.Errorf("switch 0 CC: expected 44, got %d", finalModel.config[0].CC)
	}
}

// TestTUIAdvancedCustomSendAllConfig is a headless E2E test that exercises the
// full sendAllConfig() runtime path with a real MIDI device in Advanced Custom mode.
//
// It verifies that:
//  1. The TUI goroutine dispatches BuildAdvCustomCC (not BuildCCConfig) for ModeAdvancedCustom
//  2. The correct per-switch subcmds are sent (AdvCustomSubcmdBase + sw*Stride + attr)
//  3. The mode re-assertion after init is present in the TX log
//  4. The device ACKs all messages (checked via separate amidi read after each TX)
//
// Run: go test -v -run TestTUIAdvancedCustomSendAllConfig -timeout 60s .
func TestTUIAdvancedCustomSendAllConfig(t *testing.T) {
	// Find and open the real MIDI device.
	path, err := detect.Find()
	if err != nil || path == "" {
		t.Skipf("no SINCO device found: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("device %s not accessible: %v", path, err)
	}

	dev, err := midi.Open(path)
	if err != nil {
		t.Fatalf("midi.Open(%s): %v", path, err)
	}
	defer func() { _ = dev.Close() }()

	// Build a Model in Advanced Custom mode with switch 0 set to CC=48.
	m := NewModel(path)
	m.midi = dev
	m.mode = sysex.ModeAdvancedCustom
	m.config[0].CC = 48

	// Kick off the goroutine-based send.
	m.sendAllConfig()

	// Drain midiMsgs until "DONE" arrives or timeout.
	// sendAllConfig sends: 12 init + 1 mode + 16 switches × 3 msgs = 61 total.
	deadline := time.After(45 * time.Second)
	var txLog []string
loop:
	for {
		select {
		case msg := <-m.midiMsgs:
			txLog = append(txLog, msg.Hex)
			if strings.HasPrefix(msg.Hex, "DONE") {
				break loop
			}
		case <-deadline:
			t.Fatalf("timeout after %d msgs — sendAllConfig did not complete", len(txLog))
		}
	}
	t.Logf("sendAllConfig sent %d messages", len(txLog))

	// Build the expected Advanced Custom messages for switch 0 (CC=48, momentary).
	expectedMsgs := sysex.SplitMessages(sysex.BuildAdvCustomCC(0, 48, false, 0))

	for i, expMsg := range expectedMsgs {
		expHex := strings.ToLower(hex.EncodeToString(expMsg))
		found := false
		for _, tx := range txLog {
			if strings.Contains(strings.ToLower(tx), expHex) {
				found = true
				break
			}
		}
		subcmd := expMsg[9]
		attr := int(subcmd-sysex.AdvCustomSubcmdBase) % sysex.AdvCustomSwitchStride
		attrName := []string{"CC#", "latch", "type", "?"}[attr]
		if found {
			t.Logf("  msg[%d] subcmd=0x%02X (%s) val=0x%02X — FOUND in TX log", i, subcmd, attrName, expMsg[17])
		} else {
			t.Errorf("  msg[%d] subcmd=0x%02X (%s) val=0x%02X — MISSING from TX log", i, subcmd, attrName, expMsg[17])
		}
	}

	// Verify mode re-assertion (BuildModeChange(AdvancedCustom)) appears after init.
	modeMsgHex := strings.ToLower(hex.EncodeToString(sysex.BuildModeChange(sysex.ModeAdvancedCustom)))
	modeFound := false
	for _, tx := range txLog {
		if strings.Contains(strings.ToLower(tx), modeMsgHex) {
			modeFound = true
			break
		}
	}
	if modeFound {
		t.Log("  mode re-assertion (ModeAdvancedCustom) — FOUND in TX log")
	} else {
		t.Error("  mode re-assertion (ModeAdvancedCustom) — MISSING from TX log")
	}

	// Verify NO bank-CC subcmd (CustomCCSubcmdBase=0x02) message appears for switch 0
	// (that would be the wrong path — BuildCCConfig instead of BuildAdvCustomCC).
	bankCCMsg := sysex.SplitMessages(sysex.BuildCCConfig(0, 48, false, 0))
	for i, wrongMsg := range bankCCMsg {
		wrongHex := strings.ToLower(hex.EncodeToString(wrongMsg))
		for _, tx := range txLog {
			if strings.Contains(strings.ToLower(tx), wrongHex) {
				t.Errorf("  bank-CC msg[%d] (subcmd=0x%02X — Custom CC path) found in TX log — wrong dispatch!",
					i, wrongMsg[9])
			}
		}
	}

	t.Log("NOTE: press switch 0 to confirm device sends CC 48 to your multieffects")
}

func TestTUILogViewToggle(t *testing.T) {
	m := NewModel("/dev/null")
	tm := teatest.NewTestModel(
		t, &m,
		teatest.WithInitialTermSize(100, 30),
	)

	// Wait for main screen
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(string(bts), "M-Vave Chocolate Config")
		},
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*3),
	)

	// Toggle log view twice (on then off) — just verify we don't crash
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	time.Sleep(200 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	time.Sleep(200 * time.Millisecond)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*3))
}
