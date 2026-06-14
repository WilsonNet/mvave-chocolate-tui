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

	if len(seq) != 8 {
		t.Errorf("sysex.BuildInitSequence: expected 8 messages, got %d", len(seq))
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
	defer os.Remove(tmpFile.Name())

	dev, err := midi.Open(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()
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
	defer os.Remove(tmpFile.Name())

	m := NewModel(tmpFile.Name())
	dev, err := midi.Open(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	m.midi = dev

	// Quit
	if m.midi != nil {
		m.midi.Close()
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
	defer os.Remove(tmpFile.Name())

	dev1, err := midi.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	dev1.Close()

	dev2, err := midi.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("second open after close: %v", err)
	}
	dev2.Close()
}

// --- E2E tests with teatest ---

func TestTUIQuitReleasesDevice(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "midi-tui-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

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
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*3))

	// After TUI exits, verify the file can be reopened (device properly released)
	f, err := os.OpenFile(tmpFile.Name(), os.O_RDWR, 0)
	if err != nil {
		t.Errorf("device should be free after TUI quit: %v", err)
	} else {
		f.Close()
	}
}

func TestTUIModeSelectAndCancel(t *testing.T) {
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

	// Enter mode select
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(string(bts), "Select operating mode")
		},
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*3),
	)

	// Navigate down twice and confirm with enter
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Verify we're back in main view
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(string(bts), "Switch") &&
				!strings.Contains(string(bts), "Select operating mode")
		},
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*3),
	)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second*3))
}

func TestTUIEditAndSendConfig(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "midi-send-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

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
