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
	Type    byte // 0 = Control Change; other values not yet mapped
	Channel byte // 1-16, as the app shows it
	Number  byte // CC number
	Value   byte
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

// Switch modes seen so far. Others exist in the app and are not yet mapped.
var PlusSwitchModeNames = map[byte]string{
	1: "single step (two banks)",
	4: "step short / step long",
}

const (
	plusSwitchBase   = 93  // first byte of switch A
	plusSwitchStride = 417 // bytes per switch
	plusBankStride   = 80  // Bank B starts this far after Bank A's count
	plusMaxMessages  = 19  // (80-1)/4: what fits in a bank
)

// PlusComplete reports whether every byte a Plus's four switches need has arrived.
func (img Image) PlusComplete() bool {
	last := plusSwitchBase + 3*plusSwitchStride + plusBankStride + 1 + 4*plusMaxMessages
	for a := 0; a <= last; a++ {
		if _, ok := img[a]; !ok {
			return false
		}
	}
	return true
}

// LooksLikePlus checks the SHAPE of the data, not its values: every switch
// has a plausible mode and bank sizes, every message in use a real channel,
// and at least one bank holds something. An original Chocolate has no
// switch blocks at these addresses, so it fails; a Plus whose settings
// differ from the one this was mapped on still passes.
func (img Image) LooksLikePlus() bool {
	if !img.PlusComplete() {
		return false
	}
	used := 0
	for s := 0; s < 4; s++ {
		base := plusSwitchBase + s*plusSwitchStride
		if img[base] == 0 || img[base] > 0x1F {
			return false
		}
		for bank := 0; bank < 2; bank++ {
			countAt := base + 1 + bank*plusBankStride
			n := int(img[countAt])
			if n > plusMaxMessages {
				return false
			}
			for i := 0; i < n; i++ {
				if ch := img[countAt+2+4*i]; ch < 1 || ch > 16 {
					return false
				}
			}
			used += n
		}
	}
	return used > 0
}

// DecodePlus reads the device mode and the four switches out of a complete image.
func (img Image) DecodePlus() PlusConfig {
	cfg := PlusConfig{Mode: img[0]}
	for s := 0; s < 4; s++ {
		base := plusSwitchBase + s*plusSwitchStride
		sw := PlusSwitch{Mode: img[base]}
		for bank := 0; bank < 2; bank++ {
			countAt := base + 1 + bank*plusBankStride
			count := int(img[countAt])
			if count > plusMaxMessages {
				count = plusMaxMessages
			}
			for i := 0; i < count; i++ {
				at := countAt + 1 + 4*i
				sw.Banks[bank] = append(sw.Banks[bank], PlusMessage{
					Type: img[at], Channel: img[at+1], Number: img[at+2], Value: img[at+3],
				})
			}
		}
		cfg.Switches[s] = sw
	}
	return cfg
}

// PlusChanges lists the byte writes that turn the pedal's current image into
// `want`, smallest address first. Only what has been tested on a real pedal
// is allowed: a switch's mode (to a known mode), and the type, channel, number
// and value of messages a bank already holds. Adding or removing messages
// changes a bank's count, which is untested, and is refused.
func (img Image) PlusChanges(want PlusConfig) ([]Write, error) {
	if !img.PlusComplete() {
		return nil, fmt.Errorf("the pedal has not been read completely")
	}
	have := img.DecodePlus()
	var out []Write
	set := func(addr int, v byte) {
		if img[addr] != v {
			out = append(out, Write{Addr: addr, Old: img[addr], New: v})
		}
	}
	for s := 0; s < 4; s++ {
		base := plusSwitchBase + s*plusSwitchStride
		w := want.Switches[s]
		if w.Mode != have.Switches[s].Mode {
			if _, known := PlusSwitchModeNames[w.Mode]; !known {
				return nil, fmt.Errorf("switch %c: mode %d is not one this tool knows", 'A'+s, w.Mode)
			}
			set(base, w.Mode)
		}
		for bank := 0; bank < 2; bank++ {
			if len(w.Banks[bank]) != len(have.Switches[s].Banks[bank]) {
				return nil, fmt.Errorf("switch %c bank %c: adding or removing messages is not supported yet",
					'A'+s, 'A'+bank)
			}
			countAt := base + 1 + bank*plusBankStride
			for i, m := range w.Banks[bank] {
				if m.Channel < 1 || m.Channel > 16 || m.Number > 0x7F || m.Value > 0x7F || m.Type > 0x7F {
					return nil, fmt.Errorf("switch %c bank %c message %d: out of range (%+v)", 'A'+s, 'A'+bank, i+1, m)
				}
				at := countAt + 1 + 4*i
				set(at, m.Type)
				set(at+1, m.Channel)
				set(at+2, m.Number)
				set(at+3, m.Value)
			}
		}
	}
	return out, nil
}
