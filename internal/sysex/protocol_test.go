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

// TestParseConfigResponsePacked verifies the packed 7-bit-per-byte config
// dump decoding: subcommand s owns an 8-bit field at bit offset 8*s from the
// config start (right after the "10 7E 00 00" marker).
func TestParseConfigResponsePacked(t *testing.T) {
	configStart := 17
	frame := make([]byte, 160)
	frame[0], frame[1], frame[2], frame[3], frame[4] = 0xF0, 0x00, 0x32, 0x0D, 0x49
	frame[13], frame[14], frame[15], frame[16] = 0x10, 0x7E, 0x00, 0x00

	setField := func(subcmd int, val byte) {
		for b := 0; b < 8; b++ {
			if val&(1<<b) != 0 {
				abs := 8*subcmd + b
				frame[configStart+abs/7] |= 1 << (abs % 7)
			}
		}
	}

	setField(0x00, 0x09) // mode: Advanced Custom
	setField(0x02, 48)   // bank 0 CC
	setField(0x03, 1)    // bank 0 latching
	setField(0x04, 49)   // bank 1 CC
	setField(0x30, 99)   // switch 0 CC
	setField(0x31, 1)    // switch 0 latching
	setField(0x32, 7)    // switch 0 type
	setField(0x34, 19)   // switch 1 CC

	cr := ParseConfigResponse(frame)
	if cr == nil {
		t.Fatal("ParseConfigResponse returned nil for synthetic frame")
	}
	if cr.Mode != 0x09 {
		t.Errorf("Mode = 0x%02X, want 0x09", cr.Mode)
	}
	if cr.CC != [4]byte{48, 49, 0, 0} {
		t.Errorf("CC = %v, want [48 49 0 0]", cr.CC)
	}
	if cr.Latch != [4]bool{true, false, false, false} {
		t.Errorf("Latch = %v, want [true false false false]", cr.Latch)
	}
	if cr.SwitchCC[0] != 99 || cr.SwitchCC[1] != 19 {
		t.Errorf("SwitchCC[0:2] = [%d %d], want [99 19]", cr.SwitchCC[0], cr.SwitchCC[1])
	}
	if !cr.SwitchLatch[0] || cr.SwitchType[0] != 7 {
		t.Errorf("SwitchLatch[0] = %v (want true), SwitchType[0] = %d (want 7)",
			cr.SwitchLatch[0], cr.SwitchType[0])
	}
}

// TestParseConfigResponseRealFrame is a regression test using the first bytes
// of a real 0D 49 dump captured from a device in Custom CC mode with bank 0
// CC=48 (bytes 19-20 are 0x40 0x01 in the raw frame).
func TestParseConfigResponseRealFrame(t *testing.T) {
	prefix := []byte{
		0xF0, 0x00, 0x32, 0x0D, 0x49, 0x3F, 0x00, 0x00, 0x02, 0x00,
		0x00, 0x00, 0x00, 0x10, 0x7E, 0x00, 0x00, 0x07, 0x00, 0x40,
		0x01, 0x10, 0x24, 0x00, 0x11, 0x00, 0x06, 0x00, 0x60, 0x02,
		0x00, 0x00, 0x00, 0x01,
	}
	frame := make([]byte, 160)
	copy(frame, prefix)

	cr := ParseConfigResponse(frame)
	if cr == nil {
		t.Fatal("ParseConfigResponse returned nil for real frame prefix")
	}
	if cr.Mode != 0x07 {
		t.Errorf("Mode = 0x%02X, want 0x07 (Custom CC)", cr.Mode)
	}
	if cr.CC[0] != 48 {
		t.Errorf("bank 0 CC = %d, want 48", cr.CC[0])
	}
}
