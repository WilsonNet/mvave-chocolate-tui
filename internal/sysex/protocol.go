// Copyright (C) 2026 Wilson Neto
// SPDX-License-Identifier: GPL-3.0-or-later

// Package sysex provides SysEx message construction for the M-Vave Chocolate
// MIDI controller (Jieli vendor 00 32). Protocol documented at
// https://github.com/cbix/mvave-chocolate-sysex
package sysex

// Operating mode constants — value is sent directly in BuildModeChange payload.
const (
	ModeCustom         = byte(0x07)
	ModeProgramChangeA = byte(0x00)
	ModeProgramChangeB = byte(0x01)
	ModeProgramChangeC = byte(0x0B)
	ModeKeyboardA      = byte(0x03)
	ModeKeyboardB      = byte(0x04)
	ModeMultiMedia     = byte(0x05)
	ModeTouchScreen    = byte(0x02)
	ModeManufacturer   = byte(0x06)
	ModeVideo          = byte(0x08)
	ModeAdvancedCustom = byte(0x09)
	ModeCustomKeyboard = byte(0x0A)
)

var ModeNames = map[byte]string{
	ModeCustom:         "Custom CC",
	ModeProgramChangeA: "Program Change A",
	ModeProgramChangeB: "Program Change B",
	ModeProgramChangeC: "Program Change C",
	ModeKeyboardA:      "Keyboard A",
	ModeKeyboardB:      "Keyboard B",
	ModeMultiMedia:     "MultiMedia",
	ModeTouchScreen:    "Touch Screen",
	ModeManufacturer:   "Manufacturer",
	ModeVideo:          "Video",
	ModeAdvancedCustom: "Advanced Custom",
	ModeCustomKeyboard: "Custom Keyboard",
}

// Custom CC mode — bank-shared CC/latch subcmds.
// Subcmd for bank n: CustomCCSubcmdBase + n*CustomBankStride (CC), +1 for latch.
// Verified: device ACKs these but they are NOT reflected in 0D41 readback.
const (
	CustomCCSubcmdBase    = byte(0x02) // bank 0 CC; bank n = base + n*CustomBankStride
	CustomLatchSubcmdBase = byte(0x03) // bank 0 latch; bank n = base + n*CustomBankStride
	CustomBankStride      = 2          // subcmd increment per bank
)

// Advanced Custom mode — per-switch subcmds (reverse-engineered via device probing).
// Subcmd = AdvCustomSubcmdBase + switchIndex*AdvCustomSwitchStride + attr.
// byte[10] must be AdvCustomLiveWrite (0x0E) for the write to take effect immediately.
// Verified: device ACKs all subcmds in 0x30–0x6F range with byte[10]=0x0E.
// Writes with byte[10]=0x00 update the 0D41 config dump instead of device RAM.
const (
	AdvCustomSubcmdBase   = byte(0x30) // first switch, first attr
	AdvCustomSwitchStride = 4          // subcmds per switch (4 attrs per switch)
	AdvCustomAttrCC       = 0          // attr offset: CC# value (0–127)
	AdvCustomAttrLatch    = 1          // attr offset: latch mode (0=momentary, 1=latching)
	AdvCustomAttrType     = 2          // attr offset: switch type (see AdvCustomSwitchType*)
	AdvCustomLiveWrite    = byte(0x0E) // byte[10]: write reaches device RAM (immediate effect)
	AdvCustomSwitchTypeCC = byte(0x07) // switch type: output CC message on press
)

func BuildModeChange(mode byte) []byte {
	msg := []byte{
		0xF0, 0x00, 0x32, 0x09, 0x49,
		0x00, 0x00, 0x00, 0x02,
		0x00, 0x00, 0x00, 0x00,
		0x10,
		0x00, 0x00, 0x00,
		mode,
		0x00, 0x00,
		0xF7,
	}
	cs1, cs2 := Checksum(msg[1 : len(msg)-3])
	msg[len(msg)-3] = cs1
	msg[len(msg)-2] = cs2
	return msg
}

// BuildAdvCustomCC builds the SysEx messages to configure one switch in Advanced Custom mode.
// Returns 3 concatenated messages (CC#, latch, switch type); use SplitMessages before sending.
// Subcmds use AdvCustomLiveWrite (byte[10]=0x0E) so writes take effect immediately in device RAM.
//
// Per-switch subcmd layout (stride AdvCustomSwitchStride=4):
//
//	AdvCustomSubcmdBase + switchIndex*4 + AdvCustomAttrCC    = CC# value
//	AdvCustomSubcmdBase + switchIndex*4 + AdvCustomAttrLatch = latch mode
//	AdvCustomSubcmdBase + switchIndex*4 + AdvCustomAttrType  = AdvCustomSwitchTypeCC (0x07)
func BuildAdvCustomCC(switchIndex int, ccValue byte, latching bool, channel byte) []byte {
	base := AdvCustomSubcmdBase + byte(switchIndex)*AdvCustomSwitchStride

	latchVal := byte(0)
	if latching {
		latchVal = 1
	}

	buildOne := func(subcmd, val byte) []byte {
		msg := []byte{
			0xF0, 0x00, 0x32, 0x09, 0x49,
			0x00, 0x00, 0x00, 0x02,
			subcmd, AdvCustomLiveWrite, 0x00, 0x00,
			0x10,
			0x00, 0x00, 0x00,
			val,
			0x00, 0x00,
			0xF7,
		}
		cs1, cs2 := Checksum(msg[1 : len(msg)-3])
		msg[len(msg)-3] = cs1
		msg[len(msg)-2] = cs2
		return msg
	}

	result := buildOne(base+AdvCustomAttrCC, ccValue)
	result = append(result, buildOne(base+AdvCustomAttrLatch, latchVal)...)
	result = append(result, buildOne(base+AdvCustomAttrType, AdvCustomSwitchTypeCC)...)
	return result
}

func BuildCCConfig(switchIndex int, ccValue byte, latching bool, channel byte) []byte {
	bank := switchIndex / 4

	ccSubcmd := CustomCCSubcmdBase + byte(bank)*CustomBankStride
	ccMsg := []byte{
		0xF0, 0x00, 0x32, 0x09, 0x49,
		0x00, 0x00, 0x00, 0x02,
		ccSubcmd, 0x00, 0x00, 0x00,
		0x10,
		0x00, 0x00, 0x00,
		ccValue,
		0x00, 0x00,
		0xF7,
	}
	cs1, cs2 := Checksum(ccMsg[1 : len(ccMsg)-3])
	ccMsg[len(ccMsg)-3] = cs1
	ccMsg[len(ccMsg)-2] = cs2

	latchSubcmd := CustomLatchSubcmdBase + byte(bank)*CustomBankStride
	latchVal := byte(0)
	if latching {
		latchVal = 1
	}
	latchMsg := []byte{
		0xF0, 0x00, 0x32, 0x09, 0x49,
		0x00, 0x00, 0x00, 0x02,
		latchSubcmd, 0x00, 0x00, 0x00,
		0x10,
		0x00, 0x00, 0x00,
		latchVal,
		0x00, 0x00,
		0xF7,
	}
	cs1L, cs2L := Checksum(latchMsg[1 : len(latchMsg)-3])
	latchMsg[len(latchMsg)-3] = cs1L
	latchMsg[len(latchMsg)-2] = cs2L

	return append(ccMsg, latchMsg...)
}

func BuildReadSettings() []byte {
	return []byte{
		0xF0, 0x00, 0x32, 0x0D, 0x41,
		0x00, 0x00, 0x00, 0x02,
		0x00, 0x00, 0x00, 0x00,
		0x10, 0x7E,
		0x00, 0x00, 0x07, 0x00,
		0xF7,
	}
}

func BuildDiscovery() []byte {
	return []byte{
		0xF0, 0x00, 0x32, 0x45,
		0x00, 0x00, 0x00, 0x40,
		0x7F,
		0xF7,
	}
}

func BuildInitSequence() [][]byte {
	return [][]byte{
		// 1-2: Session init / read requests (0D 41)
		{0xF0, 0x00, 0x32, 0x0D, 0x41, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x10, 0x7E, 0x00, 0x00, 0x07, 0x00, 0xF7},
		{0xF0, 0x00, 0x32, 0x0D, 0x41, 0x00, 0x00, 0x00, 0x02, 0x71, 0x07, 0x00, 0x00, 0x10, 0x6A, 0x00, 0x00, 0x33, 0x01, 0xF7},
		// 3: Mode change to Custom CC (overridden later by sendAllConfig)
		BuildModeChange(ModeCustom),
		// 4-8: Bank CC probes (CustomCCSubcmdBase + bank*CustomBankStride, value=probe)
		// subcmds 0x02, 0x04, 0x06, 0x08, 0x0A with hardcoded probe values from cbix reference
		{0xF0, 0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x02, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x20, 0x30, 0x03, 0xF7},
		{0xF0, 0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x04, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x21, 0x2A, 0x03, 0xF7},
		{0xF0, 0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x06, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x22, 0x24, 0x03, 0xF7},
		{0xF0, 0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x08, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x03, 0x5E, 0x03, 0xF7},
		{0xF0, 0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x0A, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x2C, 0x08, 0x03, 0xF7},
		// 9-12: Per-switch type init (AdvCustomSubcmdBase+2, AdvCustomLiveWrite=0x0E, type values)
		// subcmds 0x32, 0x36, 0x3A, 0x3E = AdvCustomSubcmdBase + switch*AdvCustomSwitchStride + AdvCustomAttrType
		{0xF0, 0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x32, 0x0E, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x07, 0x74, 0x02, 0xF7},
		{0xF0, 0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x36, 0x0E, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x09, 0x68, 0x02, 0xF7},
		{0xF0, 0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x3A, 0x0E, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x0A, 0x5E, 0x02, 0xF7},
		{0xF0, 0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x3E, 0x0E, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x0B, 0x54, 0x02, 0xF7},
	}
}

// SplitMessages splits concatenated SysEx messages (F0...F7 F0...F7) into individual frames.
func SplitMessages(data []byte) [][]byte {
	var msgs [][]byte
	searchFrom := 0
	for searchFrom < len(data) {
		f0 := -1
		for i := searchFrom; i < len(data); i++ {
			if data[i] == 0xF0 {
				f0 = i
				break
			}
		}
		if f0 < 0 {
			break
		}
		f7 := -1
		for i := f0 + 1; i < len(data); i++ {
			if data[i] == 0xF7 {
				f7 = i
				break
			}
		}
		if f7 < 0 {
			break
		}
		msgs = append(msgs, data[f0:f7+1])
		searchFrom = f7 + 1
	}
	if len(msgs) == 0 && len(data) > 0 {
		msgs = append(msgs, data)
	}
	return msgs
}

// ConfigResponse holds parsed device configuration from a read response.
type ConfigResponse struct {
	Mode  byte
	CC    [4]byte // bank 0-3 CC values — UNRELIABLE: 0D41 response does not reflect 09 49 CC writes
	Latch [4]bool // bank 0-3 latching state — UNRELIABLE: same caveat
}

// ParseConfigResponse parses a 0D 49 response frame.
// The mode byte (after the 10 7E 00 00 marker) IS live and reflects BuildModeChange.
// The CC and Latch fields are NOT reliable: the 0D 41 config dump is static and
// does not update when CC/latch are written via 09 49 messages (either Custom CC
// or Advanced Custom subcmds). Do not use CC/Latch to verify writes.
func ParseConfigResponse(frame []byte) *ConfigResponse {
	if len(frame) < 20 {
		return nil
	}
	if frame[0] != 0xF0 || frame[1] != 0x00 || frame[2] != 0x32 ||
		frame[3] != 0x0D || frame[4] != 0x49 {
		return nil
	}

	cr := &ConfigResponse{}

	// Find the config section after "10 7E 00 00" marker.
	configStart := -1
	for i := 5; i < len(frame)-6; i++ {
		if frame[i] == 0x10 && frame[i+1] == 0x7E && frame[i+2] == 0x00 && frame[i+3] == 0x00 {
			configStart = i + 4
			break
		}
	}
	if configStart <= 0 {
		return nil
	}

	searchEnd := len(frame) - 2 // exclude checksum + F7
	searchStart := configStart

	// Mode byte is live — reflects last BuildModeChange sent.
	if configStart < len(frame) {
		cr.Mode = frame[configStart]
	}

	// CC and Latch: parsed for completeness but static in the device response.
	for i := searchStart; i < searchEnd-1; i++ {
		b := frame[i]
		switch {
		case b == CustomCCSubcmdBase && frame[i+1] <= 127 && cr.CC[0] == 0:
			cr.CC[0] = frame[i+1]
		case b == CustomLatchSubcmdBase:
			cr.Latch[0] = frame[i+1] == 1
		case b == CustomCCSubcmdBase+CustomBankStride && frame[i+1] <= 127 && cr.CC[1] == 0:
			cr.CC[1] = frame[i+1]
		case b == CustomLatchSubcmdBase+CustomBankStride:
			cr.Latch[1] = frame[i+1] == 1
		case b == CustomCCSubcmdBase+2*CustomBankStride && frame[i+1] <= 127 && cr.CC[2] == 0:
			cr.CC[2] = frame[i+1]
		case b == CustomLatchSubcmdBase+2*CustomBankStride:
			cr.Latch[2] = frame[i+1] == 1
		case b == CustomCCSubcmdBase+3*CustomBankStride && frame[i+1] <= 127 && cr.CC[3] == 0:
			cr.CC[3] = frame[i+1]
		case b == CustomLatchSubcmdBase+3*CustomBankStride:
			cr.Latch[3] = frame[i+1] == 1
		}
	}

	return cr
}

// IsOKResponse checks if a SysEx frame is the standard OK acknowledgment.
func IsOKResponse(frame []byte) bool {
	return len(frame) >= 10 &&
		frame[0] == 0xF0 && frame[1] == 0x00 && frame[2] == 0x32 &&
		frame[3] == 0x01 && frame[4] == 0x08
}

// FindSysExFrames extracts all SysEx frames with the given header prefix from raw data.
func FindSysExFrames(data []byte, header []byte) [][]byte {
	var frames [][]byte
	searchFrom := 0
	for searchFrom < len(data) {
		idx := -1
		for i := searchFrom; i <= len(data)-len(header); i++ {
			match := true
			for j := 0; j < len(header); j++ {
				if data[i+j] != header[j] {
					match = false
					break
				}
			}
			if match {
				idx = i
				break
			}
		}
		if idx < 0 {
			break
		}
		start := idx
		end := -1
		for i := start; i < len(data); i++ {
			if data[i] == 0xF7 {
				end = i
				break
			}
		}
		if end < 0 {
			break
		}
		frames = append(frames, data[start:end+1])
		searchFrom = end + 1
	}
	return frames
}

// Checksum computes the 2-byte checksum for Chocolate SysEx messages.
// For 17-byte config messages (header 00 32 09 49):
//
//	checksum = 0x28A - sum(all data) - subcommand - value
func Checksum(data []byte) (byte, byte) {
	var sum uint16
	for _, b := range data {
		sum += uint16(b)
	}

	switch {
	case len(data) >= 4 && data[0] == 0x00 && data[1] == 0x32 &&
		data[2] == 0x09 && data[3] == 0x49:
		sub := uint16(data[8])
		val := uint16(data[len(data)-1])
		v := 0x28A - sum - sub - val
		return byte(v & 0x7F), byte((v >> 7) & 0x7F)

	case len(data) >= 4 && data[0] == 0x00 && data[1] == 0x32 &&
		data[2] == 0x01 && data[3] == 0x08:
		v := uint16(0x13A) - sum
		return byte(v & 0x7F), byte((v >> 7) & 0x7F)

	case len(data) >= 4 && data[0] == 0x00 && data[1] == 0x32 &&
		data[2] == 0x45:
		v := uint16(0x136) - sum
		return byte(v & 0x7F), byte((v >> 7) & 0x7F)

	default:
		v := uint16(0x200) - sum
		return byte(v & 0x7F), byte((v >> 7) & 0x7F)
	}
}

// KnownExample is a pre-verified SysEx example from the cbix repo.
type KnownExample struct {
	Name   string
	Hex    string
	Data   []byte
	CsLow  byte
	CsHigh byte
}

// KnownExamples contains 18 known-good SysEx messages for testing.
var KnownExamples = []KnownExample{
	{Name: "CC=0 bank1", Hex: "F00032094900000002020000001000000000007003F7", Data: []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x02, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x00}, CsLow: 0x70, CsHigh: 0x03},
	{Name: "CC=1 bank1", Hex: "F00032094900000002020000001000000000016E03F7", Data: []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x02, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x01}, CsLow: 0x6E, CsHigh: 0x03},
	{Name: "CC=2 bank1", Hex: "F00032094900000002020000001000000000026C03F7", Data: []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x02, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x02}, CsLow: 0x6C, CsHigh: 0x03},
	{Name: "CC=3 bank1", Hex: "F00032094900000002020000001000000000036A03F7", Data: []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x02, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x03}, CsLow: 0x6A, CsHigh: 0x03},
	{Name: "CC=125 bank1", Hex: "F000320949000000020200000010000000007D7601F7", Data: []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x02, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x7D}, CsLow: 0x76, CsHigh: 0x01},
	{Name: "CC=126 bank1", Hex: "F000320949000000020200000010000000007E7401F7", Data: []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x02, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x7E}, CsLow: 0x74, CsHigh: 0x01},
	{Name: "CC=127 bank1", Hex: "F000320949000000020200000010000000007F7201F7", Data: []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x02, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x7F}, CsLow: 0x72, CsHigh: 0x01},
	{Name: "CC=0 bank2", Hex: "F00032094900000002040000001000000000006C03F7", Data: []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x04, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x00}, CsLow: 0x6C, CsHigh: 0x03},
	{Name: "CC=1 bank2", Hex: "F00032094900000002040000001000000000016A03F7", Data: []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x04, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x01}, CsLow: 0x6A, CsHigh: 0x03},
	{Name: "CC=2 bank2", Hex: "F00032094900000002040000001000000000026803F7", Data: []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x04, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x02}, CsLow: 0x68, CsHigh: 0x03},
	{Name: "CC=3 bank2", Hex: "F00032094900000002040000001000000000036603F7", Data: []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x04, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x03}, CsLow: 0x66, CsHigh: 0x03},
	{Name: "OK response", Hex: "F000320108000000007F01F7", Data: []byte{0x00, 0x32, 0x01, 0x08, 0x00, 0x00, 0x00, 0x00}, CsLow: 0x7F, CsHigh: 0x01},
	{Name: "Mode: Program Change A", Hex: "F000320949000000020000000010000000007403F7", Data: []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x00}, CsLow: 0x74, CsHigh: 0x03},
	{Name: "Mode: Program Change B", Hex: "F000320949000000020000000010000000017203F7", Data: []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x01}, CsLow: 0x72, CsHigh: 0x03},
	{Name: "Mode: Custom CC", Hex: "F000320949000000020000000010000000076603F7", Data: []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x07}, CsLow: 0x66, CsHigh: 0x03},
	{Name: "Momentary bank1", Hex: "F00032094900000002030000001000000000006E03F7", Data: []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x03, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x00}, CsLow: 0x6E, CsHigh: 0x03},
	{Name: "Latching bank1", Hex: "F00032094900000002030000001000000000016C03F7", Data: []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x03, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x01}, CsLow: 0x6C, CsHigh: 0x03},
	{Name: "Discovery request", Hex: "F0003245000000407FF7", Data: []byte{0x00, 0x32, 0x45, 0x00, 0x00, 0x00, 0x40}, CsLow: 0x7F, CsHigh: 0x00},
}
