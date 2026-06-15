// Copyright (C) 2026 Wilson Neto
// SPDX-License-Identifier: GPL-3.0-or-later

package sysex

import (
	"encoding/hex"
	"strings"
	"testing"
	"testing/quick"
)

func hexEq(a, b string) bool { return strings.EqualFold(a, b) }

func TestKnownChecksums(t *testing.T) {
	for _, ex := range KnownExamples {
		t.Run(ex.Name, func(t *testing.T) {
			csLow, csHigh := Checksum(ex.Data)
			if csLow != ex.CsLow || csHigh != ex.CsHigh {
				t.Errorf("checksum mismatch:\n  got:      %02X %02X\n  expected: %02X %02X\n  data:     %s",
					csLow, csHigh, ex.CsLow, ex.CsHigh, hex.EncodeToString(ex.Data))
			}
		})
	}
}

func TestChecksumRoundtrip(t *testing.T) {
	fn := func(data []byte) bool {
		if len(data) == 0 {
			return true
		}
		for i := range data {
			data[i] = data[i] & 0x7F
		}
		c1l, c1h := Checksum(data)
		c2l, c2h := Checksum(data)
		return c1l == c2l && c1h == c2h
	}
	if err := quick.Check(fn, nil); err != nil {
		t.Error(err)
	}
}

func TestBuildModeChange(t *testing.T) {
	tests := []struct {
		name, expected string
		mode           byte
	}{
		{"ModeChange(0x07)", "f000320949000000020000000010000000076603f7", ModeCustom},
		{"ModeChange(0x00)", "f000320949000000020000000010000000007403f7", ModeProgramChangeA},
		{"ModeChange(0x01)", "f000320949000000020000000010000000017203f7", ModeProgramChangeB},
	}
	for _, tt := range tests {
		got := BuildModeChange(tt.mode)
		if !hexEq(hex.EncodeToString(got), tt.expected) {
			t.Errorf("%s:\n  got: %s\n  exp: %s", tt.name, hex.EncodeToString(got), tt.expected)
		}
	}
}

func TestCCConfigKnownMessages(t *testing.T) {
	cc0 := KnownExamples[0]     // CC=0 bank1
	latch0 := KnownExamples[15] // Momentary bank1
	data := BuildCCConfig(0, 0, false, 0)

	msgLen := len(cc0.Data) + 4
	if len(data) != 2*msgLen {
		t.Fatalf("BuildCCConfig(0,0,false,0): len=%d, expected %d", len(data), 2*msgLen)
	}

	ccExpected := make([]byte, msgLen)
	ccExpected[0] = 0xF0
	copy(ccExpected[1:], cc0.Data)
	ccExpected[len(cc0.Data)+1] = cc0.CsLow
	ccExpected[len(cc0.Data)+2] = cc0.CsHigh
	ccExpected[len(cc0.Data)+3] = 0xF7

	latchExpected := make([]byte, msgLen)
	latchExpected[0] = 0xF0
	copy(latchExpected[1:], latch0.Data)
	latchExpected[len(latch0.Data)+1] = latch0.CsLow
	latchExpected[len(latch0.Data)+2] = latch0.CsHigh
	latchExpected[len(latch0.Data)+3] = 0xF7

	expected := append(ccExpected, latchExpected...)

	if !hexEq(hex.EncodeToString(data), hex.EncodeToString(expected)) {
		t.Errorf("BuildCCConfig(0,0,false,0):\n  got: %s\n  exp: %s",
			hex.EncodeToString(data), hex.EncodeToString(expected))
	}
}

func TestBuildAdvCustomCC(t *testing.T) {
	msgs := SplitMessages(BuildAdvCustomCC(0, 48, false, 0))
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	for i, msg := range msgs {
		if msg[0] != 0xF0 || msg[len(msg)-1] != 0xF7 {
			t.Errorf("msg[%d] not valid SysEx", i)
		}
		if msg[10] != AdvCustomLiveWrite {
			t.Errorf("msg[%d] byte10=0x%02X, want AdvCustomLiveWrite=0x%02X", i, msg[10], AdvCustomLiveWrite)
		}
	}

	// switch 0: subcmds = AdvCustomSubcmdBase + 0*AdvCustomSwitchStride + attr
	wantCCSubcmd := AdvCustomSubcmdBase + 0*AdvCustomSwitchStride + AdvCustomAttrCC
	if msgs[0][9] != wantCCSubcmd {
		t.Errorf("CC subcmd=0x%02X, want 0x%02X", msgs[0][9], wantCCSubcmd)
	}
	if msgs[0][17] != 48 {
		t.Errorf("CC value=%d, want 48", msgs[0][17])
	}
	wantTypeSubcmd := AdvCustomSubcmdBase + 0*AdvCustomSwitchStride + AdvCustomAttrType
	if msgs[2][9] != wantTypeSubcmd {
		t.Errorf("type subcmd=0x%02X, want 0x%02X", msgs[2][9], wantTypeSubcmd)
	}
	if msgs[2][17] != AdvCustomSwitchTypeCC {
		t.Errorf("type value=0x%02X, want AdvCustomSwitchTypeCC=0x%02X", msgs[2][17], AdvCustomSwitchTypeCC)
	}

	// switch 3: base subcmd = AdvCustomSubcmdBase + 3*AdvCustomSwitchStride
	sw3Base := AdvCustomSubcmdBase + 3*AdvCustomSwitchStride
	msgs3 := SplitMessages(BuildAdvCustomCC(3, 64, true, 0))
	if len(msgs3) != 3 {
		t.Fatalf("switch3: expected 3 msgs, got %d", len(msgs3))
	}
	if msgs3[0][9] != sw3Base+AdvCustomAttrCC {
		t.Errorf("switch3 CC subcmd=0x%02X, want 0x%02X", msgs3[0][9], sw3Base+AdvCustomAttrCC)
	}
	if msgs3[1][9] != sw3Base+AdvCustomAttrLatch {
		t.Errorf("switch3 latch subcmd=0x%02X, want 0x%02X", msgs3[1][9], sw3Base+AdvCustomAttrLatch)
	}
	if msgs3[1][17] != 1 {
		t.Errorf("switch3 latch value=%d, want 1 (latching)", msgs3[1][17])
	}

	// switch 15 (last): base subcmd = AdvCustomSubcmdBase + 15*AdvCustomSwitchStride = 0x30 + 60 = 0x6C
	sw15Base := AdvCustomSubcmdBase + 15*AdvCustomSwitchStride
	msgs15 := SplitMessages(BuildAdvCustomCC(15, 127, false, 0))
	if len(msgs15) != 3 {
		t.Fatalf("switch15: expected 3 msgs, got %d", len(msgs15))
	}
	if msgs15[0][9] != sw15Base+AdvCustomAttrCC {
		t.Errorf("switch15 CC subcmd=0x%02X, want 0x%02X", msgs15[0][9], sw15Base+AdvCustomAttrCC)
	}
}

func TestBuildDiscovery(t *testing.T) {
	expected := "f0003245000000407ff7"
	got := BuildDiscovery()
	if !hexEq(hex.EncodeToString(got), expected) {
		t.Errorf("BuildDiscovery:\n  got: %s\n  exp: %s", hex.EncodeToString(got), expected)
	}
}

func TestBuildReadSettings(t *testing.T) {
	expected := "f000320d410000000200000000107e00000700f7"
	got := BuildReadSettings()
	if !hexEq(hex.EncodeToString(got), expected) {
		t.Errorf("BuildReadSettings:\n  got: %s\n  exp: %s", hex.EncodeToString(got), expected)
	}
}

func TestChecksumProperty(t *testing.T) {
	fn := func(data []byte) bool {
		clamped := make([]byte, len(data))
		for i, b := range data {
			clamped[i] = b & 0x7F
		}
		l1, h1 := Checksum(clamped)
		l2, h2 := Checksum(clamped)
		return l1 == l2 && h1 == h2
	}
	if err := quick.Check(fn, nil); err != nil {
		t.Error(err)
	}
}
