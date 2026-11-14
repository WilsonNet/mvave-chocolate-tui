// Copyright (C) 2026 Wilson Neto and contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

// The pedal screen does not know which Chocolate it is talking to. Everything
// model-specific -- what the configuration bytes mean, which of them may be
// written, what a switch's slots are called -- sits behind Device. Reading
// and writing are the same on every model (internal/sysex/image.go); only
// the meaning of the bytes differs.

import (
	"fmt"
	"sort"

	"mvave-chocolate-tui/internal/sysex"
)

// Message is one MIDI message a switch sends.
type Message struct {
	Type    byte // MsgPC, MsgCC, MsgNoteOn or MsgNoteOff
	Channel byte // 1-16; 0 = this model stores no channel or value
	Number  byte // PC, CC or note number
	Value   byte // CC value or velocity; unused by PC
}

// Message types. The Plus stores exactly these values; other models map to
// them.
const (
	MsgPC      = sysex.PlusPC
	MsgCC      = sysex.PlusCC
	MsgNoteOn  = sysex.PlusNoteOn
	MsgNoteOff = sysex.PlusNoteOff
)

var msgTypes = []byte{MsgPC, MsgCC, MsgNoteOn, MsgNoteOff}

func msgTypeName(t byte) string {
	if n, ok := sysex.PlusTypeNames[t]; ok {
		return n
	}
	return fmt.Sprintf("type %d", t)
}

// Stored reports whether channel and value came from the pedal. The original
// Chocolate keeps only a CC number per switch, and a screen must not show an
// invented value as if it had been read.
func (m Message) Stored() bool { return m.Channel != 0 }

func (m Message) String() string {
	if !m.Stored() {
		return fmt.Sprintf("CC%d", m.Number)
	}
	ch := fmt.Sprintf("ch%d ", m.Channel)
	switch m.Type {
	case MsgPC:
		return fmt.Sprintf("%sPC%d", ch, m.Number)
	case MsgCC:
		return fmt.Sprintf("%sCC%d=%d", ch, m.Number, m.Value)
	case MsgNoteOn:
		return fmt.Sprintf("%snote %d on %d", ch, m.Number, m.Value)
	case MsgNoteOff:
		return fmt.Sprintf("%snote %d off", ch, m.Number)
	}
	return fmt.Sprintf("%s%s %d=%d", ch, msgTypeName(m.Type), m.Number, m.Value)
}

// Short is what fits in a switch's box on the pedal drawing.
func (m Message) Short() string {
	if !m.Stored() {
		return fmt.Sprintf("CC%d", m.Number)
	}
	switch m.Type {
	case MsgPC:
		return fmt.Sprintf("PC%d", m.Number)
	case MsgCC:
		return fmt.Sprintf("CC%d=%d", m.Number, m.Value)
	case MsgNoteOn, MsgNoteOff:
		return fmt.Sprintf("note %d", m.Number)
	}
	return msgTypeName(m.Type)
}

// Switch is one footswitch: its mode, and per slot (a bank, a press length,
// ...) the messages it sends. What a slot is called comes from the device.
type Switch struct {
	Mode  byte
	Flags []byte // per slot, model-specific (the original's "latching")
	Slots [][]Message
}

// Config is a model-neutral picture of the pedal.
type Config struct {
	Mode     byte
	Switches [4]Switch
}

func (c Config) Clone() Config {
	out := c
	for s, sw := range c.Switches {
		out.Switches[s].Flags = append([]byte(nil), sw.Flags...)
		out.Switches[s].Slots = make([][]Message, len(sw.Slots))
		for i, slot := range sw.Slots {
			out.Switches[s].Slots[i] = append([]Message(nil), slot...)
		}
	}
	return out
}

// Field is one editable value of a switch, shown in a row of its detail.
type Field struct {
	Slot    int    // the slot it belongs to; -1 for the switch itself
	Msg     int    // the message within the slot; -1 if not a message's
	Row     string // the row it is drawn in ("mode", "short", "bank 2", ...)
	Label   string // drawn before the value ("ch", "CC", "=", ...)
	Min     int
	Max     int
	Choices []byte // if set, the value cycles through these
	Get     func(Switch) int
	Set     func(*Switch, int)
	Format  func(int) string
}

// In reports whether sw has what this field reads -- a message added in an
// edit does not exist on the pedal yet.
func (f Field) In(sw Switch) bool {
	if f.Slot < 0 {
		return true
	}
	if f.Slot >= len(sw.Slots) {
		return false
	}
	return f.Msg < 0 || f.Msg < len(sw.Slots[f.Slot])
}

// Device is one Chocolate model.
type Device interface {
	Product() string
	// Recognise reports whether a complete read of this model is in img.
	Recognise(img sysex.Image) bool
	Decode(img sysex.Image) Config
	// Changes lists the bytes that turn img into want, refusing anything not
	// known to be safe on this model.
	Changes(img sysex.Image, want Config) ([]sysex.Write, error)
	ModeName(mode byte) string
	// Modes lists the operating modes the device can be switched to.
	Modes() []byte
	SlotNames(sw Switch) []string
	Fields(sw Switch) []Field
	// Match finds the switch and slot that send this message.
	Match(cfg Config, got Message) (sw, slot int, ok bool)
	// MaxMessages is how many messages a slot can hold; 1 means messages
	// cannot be added or removed.
	MaxMessages() int
	HasGroupKeys() bool
	// Caveat, if not empty, is shown in the header: how far this model's
	// support has been checked against real hardware.
	Caveat() string
}

// devices are tried in order after every complete read.
var devices = []Device{plusDevice{}, originalDevice{}}

func recognise(img sysex.Image) Device {
	for _, d := range devices {
		if d.Recognise(img) {
			return d
		}
	}
	return nil
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// messageFields are channel / type / number / value for every message of a slot.
func messageFields(row string, slot, i int, withChannel bool) []Field {
	part := func(label string, lo, hi int, get func(Message) byte, set func(*Message, byte)) Field {
		return Field{Slot: slot, Msg: i, Row: row, Label: label, Min: lo, Max: hi,
			Get: func(sw Switch) int { return int(get(sw.Slots[slot][i])) },
			Set: func(sw *Switch, v int) { set(&sw.Slots[slot][i], byte(clamp(v, lo, hi))) }}
	}
	var f []Field
	if withChannel {
		typ := part("", 0, 3, func(m Message) byte { return m.Type }, func(m *Message, v byte) { m.Type = v })
		typ.Choices = msgTypes
		typ.Format = func(v int) string { return msgTypeName(byte(v)) }
		f = append(f,
			part("ch", 1, 16, func(m Message) byte { return m.Channel }, func(m *Message, v byte) { m.Channel = v }),
			typ)
	}
	return append(f,
		part("#", 0, 127, func(m Message) byte { return m.Number }, func(m *Message, v byte) { m.Number = v }),
		part("=", 0, 127, func(m Message) byte { return m.Value }, func(m *Message, v byte) { m.Value = v }))
}

// --- Chocolate Plus ---------------------------------------------------------

type plusDevice struct{}

func (plusDevice) Product() string    { return "M-VAVE Chocolate Plus" }
func (plusDevice) HasGroupKeys() bool { return true }
func (plusDevice) Caveat() string     { return "" }
func (plusDevice) Recognise(img sysex.Image) bool {
	return img.PlusComplete() && img.LooksLikePlus()
}

func (plusDevice) ModeName(mode byte) string {
	if n, ok := sysex.PlusModeNames[mode]; ok {
		return n
	}
	return fmt.Sprintf("mode %d", mode)
}

// Modes are the device modes `m` offers: the first four, where the official
// app's list and M-VAVE's manual agree (2 and 3 also checked on a real pedal).
// After those the two lists differ by firmware -- the manual has Advanced
// Custom 1 AND 2 -- so a mode index past 3 could mean either; not offered.
func (plusDevice) Modes() []byte { return []byte{0, 1, 2, 3} }

func (plusDevice) SlotNames(sw Switch) []string {
	switch sw.Mode {
	case 0:
		return []string{"tap", "bank B"}
	case 1:
		return []string{"1st press", "2nd press"}
	case 2:
		return []string{"press", "release"}
	case 3:
		return []string{"long", "bank B"}
	case 4:
		return []string{"short", "long"}
	}
	return []string{"bank A", "bank B"}
}

func (plusDevice) Decode(img sysex.Image) Config {
	p := img.DecodePlus()
	cfg := Config{Mode: p.Mode}
	for s, sw := range p.Switches {
		out := Switch{Mode: sw.Mode, Slots: make([][]Message, 2)}
		for b, bank := range sw.Banks {
			for _, m := range bank {
				out.Slots[b] = append(out.Slots[b], Message{m.Type, m.Channel, m.Number, m.Value})
			}
		}
		cfg.Switches[s] = out
	}
	return cfg
}

func (plusDevice) Changes(img sysex.Image, want Config) ([]sysex.Write, error) {
	p := sysex.PlusConfig{Mode: want.Mode}
	for s, sw := range want.Switches {
		p.Switches[s].Mode = sw.Mode
		for b := 0; b < 2 && b < len(sw.Slots); b++ {
			for _, m := range sw.Slots[b] {
				p.Switches[s].Banks[b] = append(p.Switches[s].Banks[b],
					sysex.PlusMessage{Type: m.Type, Channel: m.Channel, Number: m.Number, Value: m.Value})
			}
		}
	}
	return img.PlusChanges(p)
}

// plusSwitchModes are the switch modes editing cycles through: only those
// checked on a real pedal.
var plusSwitchModes = sysex.PlusSwitchModesChecked

func (d plusDevice) Fields(sw Switch) []Field {
	f := []Field{{Slot: -1, Msg: -1, Row: "mode", Choices: plusSwitchModes,
		Get:    func(s Switch) int { return int(s.Mode) },
		Set:    func(s *Switch, v int) { s.Mode = byte(v) },
		Format: func(v int) string { return plusSwitchModeName(byte(v)) }}}
	names := d.SlotNames(sw)
	for slot, msgs := range sw.Slots {
		for i := range msgs {
			row := names[slot]
			if len(msgs) > 1 {
				row = fmt.Sprintf("%s #%d", row, i+1)
			}
			f = append(f, messageFields(row, slot, i, true)...)
		}
	}
	return f
}

func plusSwitchModeName(mode byte) string {
	if n, ok := sysex.PlusSwitchModeNames[mode]; ok {
		return n
	}
	return fmt.Sprintf("mode %d (not mapped)", mode)
}

func (plusDevice) MaxMessages() int { return 16 }

// Match compares what arrived with what each switch is set to send. A PC has
// no value, and a Note Off's velocity is not what identifies it.
func (plusDevice) Match(cfg Config, got Message) (int, int, bool) {
	for s, sw := range cfg.Switches {
		for slot, msgs := range sw.Slots {
			for _, m := range msgs {
				if m.Type != got.Type || m.Channel != got.Channel || m.Number != got.Number {
					continue
				}
				if (m.Type == MsgCC || m.Type == MsgNoteOn) && m.Value != got.Value {
					continue
				}
				return s, slot, true
			}
		}
	}
	return 0, 0, false
}

// --- the original Chocolate, Advanced Custom mode ------------------------------
//
// From this TUI's own decoder (ParseConfigResponse) and writer
// (BuildAdvCustomCCFull): sixteen slots, four banks of four switches, each a
// CC number, a latching flag and a switch type at 0x30 + 4·i. Writes go to the
// same address with byte 10 = 0x0E as well -- in address terms 1792 higher --
// because that is what makes them take effect immediately on this model.
//
// NOT CHECKED ON HARDWARE by whoever wrote this layer; see Caveat.

type originalDevice struct{}

const (
	origAdvBase      = 0x30
	origAdvStride    = 4
	origLiveMirror   = 0x0E << 7 // the "RAM path": the same field, 1792 higher
	origLastAdvField = origAdvBase + 15*origAdvStride + 2
)

func (originalDevice) Product() string    { return "M-VAVE Chocolate" }
func (originalDevice) HasGroupKeys() bool { return false }
func (originalDevice) Caveat() string {
	return "untested on hardware — please report back"
}

func (originalDevice) Recognise(img sysex.Image) bool {
	for a := 0; a <= origLastAdvField; a++ {
		if _, ok := img[a]; !ok {
			return false
		}
	}
	return !img.LooksLikePlus() && img[0] == sysex.ModeAdvancedCustom
}

func (originalDevice) ModeName(mode byte) string {
	if n, ok := sysex.ModeNames[mode]; ok {
		return n
	}
	return fmt.Sprintf("mode %d", mode)
}

func (originalDevice) Modes() []byte {
	var out []byte
	for mode := range sysex.ModeNames {
		out = append(out, mode)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (originalDevice) SlotNames(Switch) []string {
	return []string{"bank 1", "bank 2", "bank 3", "bank 4"}
}

func origIndex(bank, sw int) int { return bank*4 + sw }

func (originalDevice) Decode(img sysex.Image) Config {
	cfg := Config{Mode: img[0]}
	for s := 0; s < 4; s++ {
		sw := Switch{Slots: make([][]Message, 4), Flags: make([]byte, 4)}
		for bank := 0; bank < 4; bank++ {
			at := origAdvBase + origAdvStride*origIndex(bank, s)
			sw.Slots[bank] = []Message{{Type: MsgCC, Number: img[at]}} // no channel or value stored
			sw.Flags[bank] = img[at+1]
		}
		cfg.Switches[s] = sw
	}
	return cfg
}

func (originalDevice) Changes(img sysex.Image, want Config) ([]sysex.Write, error) {
	var out []sysex.Write
	set := func(addr int, v byte) {
		if img[addr] != v {
			out = append(out,
				sysex.Write{Addr: addr, Old: img[addr], New: v},
				sysex.Write{Addr: addr + origLiveMirror, Old: img[addr], New: v})
		}
	}
	for s, sw := range want.Switches {
		if len(sw.Slots) != 4 || len(sw.Flags) != 4 {
			return nil, fmt.Errorf("switch %c: expected four banks", 'A'+s)
		}
		for bank := 0; bank < 4; bank++ {
			at := origAdvBase + origAdvStride*origIndex(bank, s)
			if len(sw.Slots[bank]) != 1 {
				return nil, fmt.Errorf("switch %c bank %d: this model sends one message per bank", 'A'+s, bank+1)
			}
			set(at, byte(clamp(int(sw.Slots[bank][0].Number), 0, 127)))
			set(at+1, sw.Flags[bank]&1)
		}
	}
	return out, nil
}

func (d originalDevice) Fields(sw Switch) []Field {
	var f []Field
	for bank := 0; bank < 4; bank++ {
		bank := bank
		row := d.SlotNames(sw)[bank]
		f = append(f,
			Field{Slot: bank, Msg: 0, Row: row, Label: "CC", Min: 0, Max: 127,
				Get: func(s Switch) int { return int(s.Slots[bank][0].Number) },
				Set: func(s *Switch, v int) { s.Slots[bank][0].Number = byte(clamp(v, 0, 127)) }},
			Field{Slot: bank, Msg: -1, Row: row, Label: "latch", Choices: []byte{0, 1},
				Get:    func(s Switch) int { return int(s.Flags[bank]) },
				Set:    func(s *Switch, v int) { s.Flags[bank] = byte(v) },
				Format: func(v int) string { return map[int]string{0: "no", 1: "yes"}[v] }})
	}
	return f
}

func (originalDevice) MaxMessages() int { return 1 }

// The original sends a CC on press; match on the number alone, since the
// value it sends is not part of what is stored.
func (originalDevice) Match(cfg Config, got Message) (int, int, bool) {
	if got.Type != MsgCC {
		return 0, 0, false
	}
	for s, sw := range cfg.Switches {
		for slot, msgs := range sw.Slots {
			for _, m := range msgs {
				if m.Number == got.Number {
					return s, slot, true
				}
			}
		}
	}
	return 0, 0, false
}

// Describe names a Plus configuration byte, for comparing reads.
func (d plusDevice) Describe(addr int) string {
	if addr == 0 {
		return "device mode"
	}
	const base, stride, bank = 93, 417, 80
	if addr < base || addr >= base+4*stride {
		return "not mapped"
	}
	s, off := (addr-base)/stride, (addr-base)%stride
	where := fmt.Sprintf("switch %c", 'A'+s)
	if off == 0 {
		return where + " · mode"
	}
	if off > 2*bank {
		return where + " · not mapped"
	}
	b, k := (off-1)/bank, (off-1)%bank
	i, part := k/5, k%5
	return fmt.Sprintf("%s · %s #%d · %s", where, []string{"bank A", "bank B"}[b], i+1,
		[]string{"in use", "channel", "type", "number", "value"}[part])
}
