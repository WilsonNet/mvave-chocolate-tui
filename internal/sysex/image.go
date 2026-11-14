// Copyright (C) 2026 Wilson Neto and contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package sysex

import "fmt"

// What every Chocolate shares: its configuration lives in region 2 of the
// device's memory, is read with 0D 41 requests and written one byte at a time
// with 09 49 messages -- whose "subcommand" is simply the byte's address.
// What the bytes MEAN differs per model; see plus.go.

// Read requests for the two chunks of configuration region 2, exactly as the
// official app sends them on connect. Together they cover bytes 0..1857.
var ReadRequests = [][]byte{
	{0xF0, 0x00, 0x32, 0x0D, 0x41, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x10, 0x7E, 0x00, 0x00, 0x07, 0x00, 0xF7},
	{0xF0, 0x00, 0x32, 0x0D, 0x41, 0x00, 0x00, 0x00, 0x02, 0x71, 0x07, 0x00, 0x00, 0x10, 0x6A, 0x00, 0x00, 0x33, 0x01, 0xF7},
}

// word28 reads four 7-bit MIDI bytes as one little-endian 28-bit number.
func word28(b []byte) int {
	return int(b[0]) | int(b[1])<<7 | int(b[2])<<14 | int(b[3])<<21
}

// DecodeReadReply turns one 0D 49 reply into the address it starts at and the
// bytes it carries. The header is three 28-bit words -- region (with a block
// count in its low bits), address, and length*16 -- followed by the data,
// 7-bit packed LSB first. ok is false for anything that is not such a reply.
func DecodeReadReply(frame []byte) (addr int, data []byte, ok bool) {
	if len(frame) < 21 || frame[0] != 0xF0 || frame[len(frame)-1] != 0xF7 ||
		frame[1] != 0x00 || frame[2] != 0x32 || frame[3] != 0x0D || frame[4] != 0x49 {
		return 0, nil, false
	}
	addr = word28(frame[9:13])
	want := word28(frame[13:17]) / 16
	payload := frame[17 : len(frame)-3] // two checksum bytes, then F7
	var bits uint32
	n := 0
	for _, b := range payload {
		bits |= uint32(b&0x7F) << n
		n += 7
		for n >= 8 && len(data) < want {
			data = append(data, byte(bits))
			bits >>= 8
			n -= 8
		}
	}
	// The packing stops as soon as the last byte's set bits are sent, so the
	// final byte can arrive with fewer than eight bits.
	if len(data) < want && n > 0 {
		data = append(data, byte(bits))
	}
	return addr, data, true
}

// Image collects read replies into one address-indexed image.
type Image map[int]byte

// Add stores a reply's bytes at their addresses. Returns false if the frame
// is not a read reply.
func (img Image) Add(frame []byte) bool {
	addr, data, ok := DecodeReadReply(frame)
	if !ok {
		return false
	}
	for i, b := range data {
		img[addr+i] = b
	}
	return true
}

// --- writing ----------------------------------------------------------------

// ConfigEnd is one past the last address the two reads cover.
const ConfigEnd = 1858

// BuildWrite sets one byte of configuration region 2. The device answers
// an accepted write with the OK message (IsOKResponse) and says nothing about
// one it refuses -- a bad checksum, or a message slot the bank does not use.
func BuildWrite(addr int, value byte) ([]byte, error) {
	// Two 7-bit bytes of address. Wider than what the two reads cover
	// (ConfigEnd): the original Chocolate's live copy of a field sits 1792
	// higher.
	if addr < 0 || addr >= 1<<14 {
		return nil, fmt.Errorf("address %d does not fit in a write (0..%d)", addr, 1<<14-1)
	}
	if value > 0x7F {
		return nil, fmt.Errorf("value %d does not fit in a MIDI data byte", value)
	}
	data := []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02,
		byte(addr & 0x7F), byte((addr >> 7) & 0x7F), 0x00, 0x00,
		0x10, 0x00, 0x00, 0x00, value}
	lo, hi := Checksum(data)
	msg := append([]byte{0xF0}, data...)
	return append(msg, lo, hi, 0xF7), nil
}

// Write is one byte to change: where, from what, to what.
type Write struct {
	Addr     int
	Old, New byte
}
