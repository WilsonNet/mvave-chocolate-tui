// Copyright (C) 2026 Wilson Neto and contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package sysex

import "fmt"

// Chocolate Plus ("FootCtrlPlus") configuration: reading it, and changing
// the parts that have been tested on a real pedal.
//
// The Plus answers the same 0D 41 read requests as the original Chocolate and
// shows up on USB under the same name (SINCO), but lays its configuration out
// differently: a mode per switch, and per switch two banks that each hold a
// LIST of messages. Mapped by reading a pedal whose settings were known from
// the official app's own screens, so every offset below was checked against a
// real value -- see plus_test.go and testdata/.

// PlusMessage is one MIDI message a switch sends.
type PlusMessage struct {
	Type    byte // see PlusTypeNames
	Channel byte // 1-16, as the app shows it (stored as 0-15)
	Number  byte // CC, PC or note number
	Value   byte // CC value or velocity; unused by PC
}

// Message types as stored -- mapped on a real pedal by setting each in the
// official app and reading it back. The order is M-VAVE's manual's.
const (
	PlusPC      byte = 0
	PlusCC      byte = 1
	PlusNoteOn  byte = 2
	PlusNoteOff byte = 3
	PlusSysEx   byte = 4 // per the manual; NOT checked, and never written
)

var PlusTypeNames = map[byte]string{
	PlusPC: "PC", PlusCC: "CC", PlusNoteOn: "Note On", PlusNoteOff: "Note Off", PlusSysEx: "SysEx",
}

// PlusSwitch is one footswitch in the current group.
type PlusSwitch struct {
	Mode  byte             // see PlusSwitchModeNames
	Banks [2][]PlusMessage // Bank A, Bank B
}

// PlusConfig is what a read of the Plus decodes to.
type PlusConfig struct {
	Mode     byte // device mode, an index into PlusModeNames
	Switches [4]PlusSwitch
}

// The official app's mode list, in order; the stored value is the index.
var PlusModeNames = map[byte]string{
	0: "Program change A",
	1: "Program change B",
	2: "Custom control",
	3: "Advanced custom mode",
	4: "Manufacturer control",
	5: "Touch screen (Android)",
	6: "Video model",
}

// The five switch modes, in the order M-VAVE's "Chocolate Plus Software
// Instructions" lists them. All five were set in the official app on a real
// pedal and read back at exactly these values.
var PlusSwitchModeNames = map[byte]string{
	0: "single tap",              // the same message every tap
	1: "single step (two banks)", // alternates: 1st press bank A, 2nd bank B
	2: "press / release",         // bank A on press, bank B on release
	3: "long press",              // sends on a long press
	4: "step short / step long",  // bank A on a short tap, bank B on a long press
}

// PlusSwitchModesChecked are the switch modes a write may set.
var PlusSwitchModesChecked = []byte{0, 1, 2, 3, 4}

// The layout, mapped on a real pedal:
//
//	93 + 417*s          switch s's mode
//	  + 1 + 80*b        bank b: sixteen 5-byte messages
//	      + 5*i         [in use, channel-1, type, number, value]
//
// Adding a message in the official app set only its "in use" byte (to 1;
// the rest was already 0: channel 1, PC 0, value 0 -- the app's default).
// The device refuses writes into a slot that is not in use.
const (
	plusSwitchBase   = 93
	plusSwitchStride = 417
	plusBankStride   = 80
	plusMsgSize      = 5
	plusMaxMessages  = plusBankStride / plusMsgSize // 16
)

func plusMsgAt(s, bank, i int) int {
	return plusSwitchBase + s*plusSwitchStride + 1 + bank*plusBankStride + i*plusMsgSize
}

// PlusComplete reports whether every byte a Plus's four switches need has arrived.
func (img Image) PlusComplete() bool {
	last := plusMsgAt(3, 1, plusMaxMessages) - 1
	for a := 0; a <= last; a++ {
		if _, ok := img[a]; !ok {
			return false
		}
	}
	return true
}

// plusBank reads one bank: the run of in-use messages from slot 0, and
// whether any slot after that run is in use too (a gap the official app
// does not make, and this tool will not edit around).
func (img Image) plusBank(s, bank int) (msgs []PlusMessage, gap bool) {
	i := 0
	for ; i < plusMaxMessages && img[plusMsgAt(s, bank, i)] == 1; i++ {
		at := plusMsgAt(s, bank, i)
		msgs = append(msgs, PlusMessage{
			Channel: img[at+1] + 1, Type: img[at+2], Number: img[at+3], Value: img[at+4],
		})
	}
	for ; i < plusMaxMessages; i++ {
		if img[plusMsgAt(s, bank, i)] != 0 {
			gap = true
		}
	}
	return msgs, gap
}

// LooksLikePlus checks the SHAPE of the data, not its values: every switch
// has a known mode, every "in use" byte is 0 or 1, every message in use a
// real channel and type, and at least one message is in use. An original
// Chocolate has no switch blocks at these addresses, so it fails; a Plus
// whose settings differ from the one this was mapped on still passes.
func (img Image) LooksLikePlus() bool {
	if !img.PlusComplete() {
		return false
	}
	used := 0
	for s := 0; s < 4; s++ {
		if _, ok := PlusSwitchModeNames[img[plusSwitchBase+s*plusSwitchStride]]; !ok {
			return false
		}
		for bank := 0; bank < 2; bank++ {
			for i := 0; i < plusMaxMessages; i++ {
				at := plusMsgAt(s, bank, i)
				switch img[at] {
				case 0:
				case 1:
					if img[at+1] > 15 || img[at+2] > PlusSysEx {
						return false
					}
					used++
				default:
					return false
				}
			}
		}
	}
	return used > 0
}

// DecodePlus reads the device mode and the four switches out of a complete image.
func (img Image) DecodePlus() PlusConfig {
	cfg := PlusConfig{Mode: img[0]}
	for s := 0; s < 4; s++ {
		sw := PlusSwitch{Mode: img[plusSwitchBase+s*plusSwitchStride]}
		for bank := 0; bank < 2; bank++ {
			sw.Banks[bank], _ = img.plusBank(s, bank)
		}
		cfg.Switches[s] = sw
	}
	return cfg
}

// PlusChanges lists the byte writes that turn the pedal's current image into
// `want`, IN THE ORDER THEY MUST BE SENT:
//
//   - a new message's "in use" byte before its fields, since the device
//     refuses writes into a slot that is not in use;
//   - a removed message's fields cleared before its "in use" byte, so the
//     slot ends exactly as a never-used one.
//
// Messages are added and removed at the end of a bank only, keeping the
// unbroken run the official app keeps. SysEx is refused.
func (img Image) PlusChanges(want PlusConfig) ([]Write, error) {
	if !img.PlusComplete() {
		return nil, fmt.Errorf("the pedal has not been read completely")
	}
	have := img.DecodePlus()
	var out []Write
	cur := map[int]byte{} // what the device will hold once `out` is sent
	get := func(addr int) byte {
		if v, ok := cur[addr]; ok {
			return v
		}
		return img[addr]
	}
	set := func(addr int, v byte) {
		if get(addr) != v {
			out = append(out, Write{Addr: addr, Old: get(addr), New: v})
			cur[addr] = v
		}
	}
	for s := 0; s < 4; s++ {
		w := want.Switches[s]
		if w.Mode != have.Switches[s].Mode {
			checked := false
			for _, ok := range PlusSwitchModesChecked {
				checked = checked || ok == w.Mode
			}
			if !checked {
				return nil, fmt.Errorf("switch %c: mode %d is not a known switch mode", 'A'+s, w.Mode)
			}
			set(plusSwitchBase+s*plusSwitchStride, w.Mode)
		}
		for bank := 0; bank < 2; bank++ {
			if _, gap := img.plusBank(s, bank); gap {
				return nil, fmt.Errorf("switch %c bank %c has a gap between its messages; edit it in the official app",
					'A'+s, 'A'+bank)
			}
			if len(w.Banks[bank]) > plusMaxMessages {
				return nil, fmt.Errorf("switch %c bank %c: at most %d messages", 'A'+s, 'A'+bank, plusMaxMessages)
			}
			for i, m := range w.Banks[bank] {
				if m.Channel < 1 || m.Channel > 16 || m.Number > 0x7F || m.Value > 0x7F {
					return nil, fmt.Errorf("switch %c bank %c message %d: out of range (%+v)", 'A'+s, 'A'+bank, i+1, m)
				}
				if m.Type > PlusNoteOff {
					return nil, fmt.Errorf("switch %c bank %c message %d: %s is not supported",
						'A'+s, 'A'+bank, i+1, PlusTypeNames[m.Type])
				}
				at := plusMsgAt(s, bank, i)
				set(at, 1) // in use -- first, or the field writes are refused
				set(at+1, m.Channel-1)
				set(at+2, m.Type)
				set(at+3, m.Number)
				set(at+4, m.Value)
			}
			// Messages removed from the end: last first, fields before flag.
			for i := len(have.Switches[s].Banks[bank]) - 1; i >= len(w.Banks[bank]); i-- {
				at := plusMsgAt(s, bank, i)
				for k := 4; k >= 1; k-- {
					set(at+k, 0)
				}
				set(at, 0)
			}
		}
	}
	return out, nil
}
