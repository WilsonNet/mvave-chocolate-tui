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

// PlusSwitch is one footswitch in one group.
type PlusSwitch struct {
	Mode  byte             // see PlusSwitchModeNames
	Banks [2][]PlusMessage // Bank A, Bank B
}

// PlusConfig is what a read of the Plus decodes to.
type PlusConfig struct {
	Mode byte // device mode, an index into PlusModeNames
	// Groups holds every group the read covered, in order: just group 1 from
	// a partial read, all eight from a full one.
	Groups [][4]PlusSwitch
	// GroupCount is how many groups E/F step through (1-8); 0 if the read did
	// not reach it. CurrentGroup is the group the pedal is on now, counted
	// from 0; -1 if not read.
	GroupCount   int
	CurrentGroup int
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
//	93 + 417*(4g + s)   switch s of group g (both from 0): its mode
//	  + 1 + 80*b        bank b: sixteen 5-byte messages
//	      + 5*i         [in use, channel-1, type, number, value]
//	23639               group count - 1
//	23640               current group (from 0) -- writing it switches the pedal
//
// Adding a message in the official app set only its "in use" byte (to 1;
// the rest was already 0: channel 1, PC 0, value 0 -- the app's default).
// The device refuses writes into a slot that is not in use. The groups and
// the two group bytes were mapped from the official app's own USB traffic.
const (
	PlusGroups       = 8
	plusSwitchBase   = 93
	plusSwitchStride = 417
	plusBankStride   = 80
	plusMsgSize      = 5
	plusMaxMessages  = plusBankStride / plusMsgSize // 16
	plusGroupCountAt = 23639
	plusCurrentAt    = 23640
)

func plusSwitchAt(g, s int) int { return plusSwitchBase + (4*g+s)*plusSwitchStride }

func plusMsgAt(g, s, bank, i int) int {
	return plusSwitchAt(g, s) + 1 + bank*plusBankStride + i*plusMsgSize
}

// plusGroupRead reports whether every byte of group g has arrived.
func (img Image) plusGroupRead(g int) bool {
	for a := plusSwitchAt(g, 0); a < plusSwitchAt(g, 3)+plusSwitchStride; a++ {
		if _, ok := img[a]; !ok {
			return false
		}
	}
	return true
}

// PlusComplete reports whether group 1 -- what the screen needs at least --
// has arrived.
func (img Image) PlusComplete() bool { return img.plusGroupRead(0) }

// plusBank reads one bank: the run of in-use messages from slot 0, and
// whether any slot after that run is in use too (a gap the official app
// does not make, and this tool will not edit around).
func (img Image) plusBank(g, s, bank int) (msgs []PlusMessage, gap bool) {
	i := 0
	for ; i < plusMaxMessages && img[plusMsgAt(g, s, bank, i)] == 1; i++ {
		at := plusMsgAt(g, s, bank, i)
		msgs = append(msgs, PlusMessage{
			Channel: img[at+1] + 1, Type: img[at+2], Number: img[at+3], Value: img[at+4],
		})
	}
	for ; i < plusMaxMessages; i++ {
		if img[plusMsgAt(g, s, bank, i)] != 0 {
			gap = true
		}
	}
	return msgs, gap
}

// LooksLikePlus checks the SHAPE of the data, not its values: in every group
// read, each switch has a known mode, every "in use" byte is 0 or 1, every
// message in use a real channel and type -- and group 1 uses at least one
// message. An original Chocolate has no switch blocks at these addresses, so
// it fails; a Plus whose settings differ from the one this was mapped on
// still passes.
func (img Image) LooksLikePlus() bool {
	if !img.PlusComplete() {
		return false
	}
	used := 0
	for g := 0; g < PlusGroups && img.plusGroupRead(g); g++ {
		for s := 0; s < 4; s++ {
			if _, ok := PlusSwitchModeNames[img[plusSwitchAt(g, s)]]; !ok {
				return false
			}
			for bank := 0; bank < 2; bank++ {
				for i := 0; i < plusMaxMessages; i++ {
					at := plusMsgAt(g, s, bank, i)
					switch img[at] {
					case 0:
					case 1:
						if img[at+1] > 15 || img[at+2] > PlusSysEx {
							return false
						}
						if g == 0 {
							used++
						}
					default:
						return false
					}
				}
			}
		}
	}
	return used > 0
}

// DecodePlus reads the device mode, every group the read covered, and the
// group count and current group if it reached them.
func (img Image) DecodePlus() PlusConfig {
	cfg := PlusConfig{Mode: img[0], CurrentGroup: -1}
	for g := 0; g < PlusGroups && img.plusGroupRead(g); g++ {
		var group [4]PlusSwitch
		for s := 0; s < 4; s++ {
			group[s].Mode = img[plusSwitchAt(g, s)]
			for bank := 0; bank < 2; bank++ {
				group[s].Banks[bank], _ = img.plusBank(g, s, bank)
			}
		}
		cfg.Groups = append(cfg.Groups, group)
	}
	if v, ok := img[plusGroupCountAt]; ok {
		cfg.GroupCount = int(v) + 1
	}
	if v, ok := img[plusCurrentAt]; ok {
		cfg.CurrentGroup = int(v)
	}
	return cfg
}

// PlusChanges lists the byte writes that turn the pedal's current image into
// `want`, IN THE ORDER THEY MUST BE SENT:
//
//   - a new message's "in use" byte before its fields, since the device
//     refuses writes into a slot that is not in use;
//   - a removed message's fields cleared before its "in use" byte, so the
//     slot ends exactly as a never-used one;
//   - the group count, then the current group, last.
//
// Messages are added and removed at the end of a bank only, keeping the
// unbroken run the official app keeps. SysEx is refused.
func (img Image) PlusChanges(want PlusConfig) ([]Write, error) {
	if !img.PlusComplete() {
		return nil, fmt.Errorf("the pedal has not been read completely")
	}
	have := img.DecodePlus()
	if len(want.Groups) != len(have.Groups) {
		return nil, fmt.Errorf("%d groups read, %d to write", len(have.Groups), len(want.Groups))
	}
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
	for g := range want.Groups {
		where := func(s int) string {
			if len(want.Groups) > 1 {
				return fmt.Sprintf("group %d switch %c", g+1, 'A'+s)
			}
			return fmt.Sprintf("switch %c", 'A'+s)
		}
		for s := 0; s < 4; s++ {
			w := want.Groups[g][s]
			if w.Mode != have.Groups[g][s].Mode {
				checked := false
				for _, ok := range PlusSwitchModesChecked {
					checked = checked || ok == w.Mode
				}
				if !checked {
					return nil, fmt.Errorf("%s: mode %d is not a known switch mode", where(s), w.Mode)
				}
				set(plusSwitchAt(g, s), w.Mode)
			}
			for bank := 0; bank < 2; bank++ {
				if _, gap := img.plusBank(g, s, bank); gap {
					return nil, fmt.Errorf("%s bank %c has a gap between its messages; edit it in the official app",
						where(s), 'A'+bank)
				}
				if len(w.Banks[bank]) > plusMaxMessages {
					return nil, fmt.Errorf("%s bank %c: at most %d messages", where(s), 'A'+bank, plusMaxMessages)
				}
				for i, m := range w.Banks[bank] {
					if m.Channel < 1 || m.Channel > 16 || m.Number > 0x7F || m.Value > 0x7F {
						return nil, fmt.Errorf("%s bank %c message %d: out of range (%+v)", where(s), 'A'+bank, i+1, m)
					}
					if m.Type > PlusNoteOff {
						return nil, fmt.Errorf("%s bank %c message %d: %s is not supported",
							where(s), 'A'+bank, i+1, PlusTypeNames[m.Type])
					}
					at := plusMsgAt(g, s, bank, i)
					set(at, 1) // in use -- first, or the field writes are refused
					set(at+1, m.Channel-1)
					set(at+2, m.Type)
					set(at+3, m.Number)
					set(at+4, m.Value)
				}
				// Messages removed from the end: last first, fields before flag.
				for i := len(have.Groups[g][s].Banks[bank]) - 1; i >= len(w.Banks[bank]); i-- {
					at := plusMsgAt(g, s, bank, i)
					for k := 4; k >= 1; k-- {
						set(at+k, 0)
					}
					set(at, 0)
				}
			}
		}
	}
	if want.GroupCount != have.GroupCount {
		if have.GroupCount == 0 {
			return nil, fmt.Errorf("the group count was not read")
		}
		if want.GroupCount < 1 || want.GroupCount > PlusGroups {
			return nil, fmt.Errorf("group count %d: must be 1-%d", want.GroupCount, PlusGroups)
		}
		set(plusGroupCountAt, byte(want.GroupCount-1))
	}
	if want.CurrentGroup != have.CurrentGroup {
		if have.CurrentGroup < 0 {
			return nil, fmt.Errorf("the current group was not read")
		}
		count := want.GroupCount
		if want.CurrentGroup < 0 || want.CurrentGroup >= count {
			return nil, fmt.Errorf("group %d is beyond the %d groups in use", want.CurrentGroup+1, count)
		}
		set(plusCurrentAt, byte(want.CurrentGroup))
	}
	return out, nil
}
