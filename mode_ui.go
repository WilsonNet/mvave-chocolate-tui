// Copyright (C) 2026 Wilson Neto and contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

// Changing the device's operating mode (`m`), on either screen.
//
// The mode is the byte at address 0 of configuration region 2 -- the same
// 09 49 write as BuildModeChange, which is BuildWrite(0, mode). On a
// Chocolate Plus, switching mode and back was checked to leave every other
// byte as it was: the Advanced Custom setup waits underneath.
//
// Like any write: needs --write, asks first, waits for the device's OK, and
// reads back to check the mode actually changed.

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mvave-chocolate-tui/internal/sysex"
)

type modePicker struct {
	open    bool
	modes   []byte
	pick    int
	confirm bool
	writing bool
	target  byte
	expect  bool // a read must now show target at address 0
}

type modeDoneMsg struct{ err error }

// availableModes lists the modes this device can be switched to, in order.
func (m *Model) availableModes() []byte {
	if m.device != nil {
		return m.device.Modes()
	}
	var out []byte
	for mode := range sysex.ModeNames {
		out = append(out, mode)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (m *Model) modeName(mode byte) string {
	if m.device != nil {
		return m.device.ModeName(mode)
	}
	if n, ok := sysex.ModeNames[mode]; ok {
		return n
	}
	return fmt.Sprintf("mode %d", mode)
}

// currentMode is the last mode read, and whether one has been read at all.
func (m *Model) currentMode() (byte, bool) {
	if m.cfg != nil {
		return m.cfg.Mode, true
	}
	return m.deviceMode, m.modeKnown
}

func (m *Model) openModePicker() {
	if m.midi == nil {
		m.statusMsg = "Not connected"
		return
	}
	mp := &m.mp
	mp.open, mp.confirm, mp.modes, mp.pick = true, false, m.availableModes(), 0
	if cur, ok := m.currentMode(); ok {
		for i, mode := range mp.modes {
			if mode == cur {
				mp.pick = i
			}
		}
	}
}

func (m *Model) handleModeKey(msg tea.KeyMsg) tea.Cmd {
	mp := &m.mp
	if mp.writing {
		return nil
	}
	k := msg.String()
	if mp.confirm {
		switch k {
		case "y", "Y":
			mp.confirm, mp.open = false, false
			return m.writeMode(mp.target)
		case "n", "N", "esc":
			mp.confirm = false
		}
		return nil
	}
	switch k {
	case "up", "k":
		if mp.pick > 0 {
			mp.pick--
		}
	case "down", "j":
		if mp.pick < len(mp.modes)-1 {
			mp.pick++
		}
	case "esc", "m", "q":
		mp.open = false
	case "enter":
		target := mp.modes[mp.pick]
		if cur, ok := m.currentMode(); ok && cur == target {
			m.statusMsg = "Already in " + m.modeName(target)
			mp.open = false
			return nil
		}
		if !m.allowWrite {
			m.statusMsg = "Read-only — restart with --write to change the mode"
			mp.open = false
			return nil
		}
		mp.target, mp.confirm = target, true
	}
	return nil
}

func (m *Model) writeMode(target byte) tea.Cmd {
	dev := m.midi
	m.mp.writing = true
	m.statusMsg = "Changing mode to " + m.modeName(target) + "…"
	for len(pedalAck) > 0 {
		<-pedalAck
	}
	return func() tea.Msg {
		msg, err := sysex.BuildWrite(0, target)
		if err == nil {
			err = dev.SendSysex(msg)
		}
		if err != nil {
			return modeDoneMsg{err}
		}
		select {
		case <-pedalAck:
			return modeDoneMsg{}
		case <-time.After(ackWait):
			return modeDoneMsg{fmt.Errorf("the device did not acknowledge the mode change")}
		}
	}
}

func (m *Model) finishMode(done modeDoneMsg) {
	m.mp.writing = false
	if done.err != nil {
		m.statusMsg = "Mode not changed: " + done.err.Error()
		return
	}
	m.mp.expect = true
	m.statusMsg = "Mode written — reading back to verify…"
	m.requestConfig()
}

// verifyMode runs on every read reply; it settles once address 0 is in, and
// reports whether it just did -- so the caller does not overwrite its result.
func (m *Model) verifyMode() bool {
	if !m.mp.expect {
		return false
	}
	got, ok := m.pedalImage[0]
	if !ok {
		return false
	}
	m.mp.expect = false
	if got == m.mp.target {
		m.statusMsg = "Mode changed to " + m.modeName(got) + " ✓"
	} else {
		m.statusMsg = fmt.Sprintf("Read-back says %s, not %s", m.modeName(got), m.modeName(m.mp.target))
	}
	return true
}

func (m *Model) drawModePicker() string {
	mp := &m.mp
	cur, known := m.currentMode()
	lines := []string{pMuted.Render("operating mode")}
	if mp.confirm {
		from := "an unread mode"
		if known {
			from = m.modeName(cur)
		}
		lines = []string{pChanged.Render(fmt.Sprintf("Change the mode from %s to %s?", from, m.modeName(mp.target))),
			pMuted.Render("One byte (address 0); the device must acknowledge it, then it is read back.")}
		return pPanel.BorderForeground(pWarn).Render(strings.Join(lines, "\n"))
	}
	for i, mode := range mp.modes {
		name := m.modeName(mode)
		if known && mode == cur {
			name += pMuted.Render("  · current")
		}
		if i == mp.pick {
			lines = append(lines, "  "+pCursor.Render(m.modeName(mode))+strings.TrimPrefix(name, m.modeName(mode)))
		} else {
			lines = append(lines, "  "+name)
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) modeHelp() string {
	if m.mp.writing {
		return "writing…"
	}
	if m.mp.confirm {
		return "y change mode · n cancel"
	}
	return "↑↓ choose · enter change · esc cancel"
}
