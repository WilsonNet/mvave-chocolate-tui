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
