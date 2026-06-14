package main

import (
	"encoding/hex"
	"strings"
	"testing"
	"testing/quick"
)

func hexEq(a, b string) bool {
	return strings.EqualFold(a, b)
}

func testHex(t *testing.T, name, expected string, got []byte) {
	t.Helper()
	gotHex := hex.EncodeToString(got)
	if !hexEq(gotHex, expected) {
		t.Errorf("%s:\n  got:      %s\n  expected: %s", name, gotHex, expected)
	}
}

// Known-good examples from https://github.com/cbix/mvave-chocolate-sysex

type knownExample struct {
	name string
	hex  string
	// bytes between F0 and checksum (exclusive of both F0, checksum, F7)
	data   []byte
	csLow  byte
	csHigh byte
}

var knownExamples = []knownExample{
	// CC = 0 for bank 1 (subcommand 0x02)
	{
		name:   "CC=0 bank1",
		hex:    "F00032094900000002020000001000000000007003F7",
		data:   []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x02, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x00},
		csLow:  0x70,
		csHigh: 0x03,
	},
	// CC = 1 for bank 1
	{
		name:   "CC=1 bank1",
		hex:    "F00032094900000002020000001000000000016E03F7",
		data:   []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x02, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x01},
		csLow:  0x6E,
		csHigh: 0x03,
	},
	// CC = 2 for bank 1
	{
		name:   "CC=2 bank1",
		hex:    "F00032094900000002020000001000000000026C03F7",
		data:   []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x02, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x02},
		csLow:  0x6C,
		csHigh: 0x03,
	},
	// CC = 3 for bank 1
	{
		name:   "CC=3 bank1",
		hex:    "F00032094900000002020000001000000000036A03F7",
		data:   []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x02, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x03},
		csLow:  0x6A,
		csHigh: 0x03,
	},
	// CC = 125 for bank 1
	{
		name:   "CC=125 bank1",
		hex:    "F000320949000000020200000010000000007D7601F7",
		data:   []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x02, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x7D},
		csLow:  0x76,
		csHigh: 0x01,
	},
	// CC = 126 for bank 1
	{
		name:   "CC=126 bank1",
		hex:    "F000320949000000020200000010000000007E7401F7",
		data:   []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x02, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x7E},
		csLow:  0x74,
		csHigh: 0x01,
	},
	// CC = 127 for bank 1
	{
		name:   "CC=127 bank1",
		hex:    "F000320949000000020200000010000000007F7201F7",
		data:   []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x02, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x7F},
		csLow:  0x72,
		csHigh: 0x01,
	},
	// CC = 0 for bank 2 (subcommand 0x04)
	{
		name:   "CC=0 bank2",
		hex:    "F00032094900000002040000001000000000006C03F7",
		data:   []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x04, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x00},
		csLow:  0x6C,
		csHigh: 0x03,
	},
	// CC = 1 for bank 2
	{
		name:   "CC=1 bank2",
		hex:    "F00032094900000002040000001000000000016A03F7",
		data:   []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x04, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x01},
		csLow:  0x6A,
		csHigh: 0x03,
	},
	// CC = 2 for bank 2
	{
		name:   "CC=2 bank2",
		hex:    "F00032094900000002040000001000000000026803F7",
		data:   []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x04, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x02},
		csLow:  0x68,
		csHigh: 0x03,
	},
	// CC = 3 for bank 2
	{
		name:   "CC=3 bank2",
		hex:    "F00032094900000002040000001000000000036603F7",
		data:   []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x04, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x03},
		csLow:  0x66,
		csHigh: 0x03,
	},

	// OK response
	{
		name:   "OK response",
		hex:    "F000320108000000007F01F7",
		data:   []byte{0x00, 0x32, 0x01, 0x08, 0x00, 0x00, 0x00, 0x00},
		csLow:  0x7F,
		csHigh: 0x01,
	},

	// Mode selections
	{
		name:   "Mode: Program Change A (0x00)",
		hex:    "F000320949000000020000000010000000007403F7",
		data:   []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x00},
		csLow:  0x74,
		csHigh: 0x03,
	},
	{
		name:   "Mode: Program Change B (0x01)",
		hex:    "F000320949000000020000000010000000017203F7",
		data:   []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x01},
		csLow:  0x72,
		csHigh: 0x03,
	},
	{
		name:   "Mode: Custom CC (0x07)",
		hex:    "F000320949000000020000000010000000076603F7",
		data:   []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x07},
		csLow:  0x66,
		csHigh: 0x03,
	},

	// Latching/Momentary for bank 1 (subcommand 0x03)
	{
		name:   "Momentary bank1 (0x00)",
		hex:    "F00032094900000002030000001000000000006E03F7",
		data:   []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x03, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x00},
		csLow:  0x6E,
		csHigh: 0x03,
	},
	{
		name:   "Latching bank1 (0x01)",
		hex:    "F00032094900000002030000001000000000016C03F7",
		data:   []byte{0x00, 0x32, 0x09, 0x49, 0x00, 0x00, 0x00, 0x02, 0x03, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x01},
		csLow:  0x6C,
		csHigh: 0x03,
	},

	// Discovery
	{
		name:   "Discovery request",
		hex:    "F0003245000000407FF7",
		data:   []byte{0x00, 0x32, 0x45, 0x00, 0x00, 0x00, 0x40},
		csLow:  0x7F,
		csHigh: 0x00,
	},
}

// TestKnownChecksums verifies checksum computation against all known examples.
func TestKnownChecksums(t *testing.T) {
	for _, ex := range knownExamples {
		t.Run(ex.name, func(t *testing.T) {
			csLow, csHigh := SysexChecksum(ex.data)
			if csLow != ex.csLow || csHigh != ex.csHigh {
				t.Errorf("checksum mismatch:\n  got:      %02X %02X\n  expected: %02X %02X\n  data:     %s",
					csLow, csHigh, ex.csLow, ex.csHigh, hex.EncodeToString(ex.data))
			}
		})
	}
}

// TestChecksumRoundtrip uses testing/quick to verify checksum properties.
func TestChecksumRoundtrip(t *testing.T) {
	// The checksum should be deterministic: same input -> same output
	fn := func(data []byte) bool {
		if len(data) == 0 {
			return true
		}
		// Ensure bytes are in valid MIDI range (0x00-0x7F)
		for i := range data {
			data[i] = data[i] & 0x7F
		}
		c1l, c1h := SysexChecksum(data)
		c2l, c2h := SysexChecksum(data)
		return c1l == c2l && c1h == c2h
	}
	if err := quick.Check(fn, nil); err != nil {
		t.Error(err)
	}
}

// TestBuildModeChange verifies that BuildModeChange produces correct known messages.
func TestBuildModeChange(t *testing.T) {
	testHex(t, "BuildModeChange(0x07)", "F000320949000000020000000010000000076603F7",
		BuildModeChange(0x07))
	testHex(t, "BuildModeChange(0x00)", "F000320949000000020000000010000000007403F7",
		BuildModeChange(0x00))
	testHex(t, "BuildModeChange(0x01)", "F000320949000000020000000010000000017203F7",
		BuildModeChange(0x01))
}

// TestCCConfigKnownMessages verifies BuildCCConfig against known examples.
func TestCCConfigKnownMessages(t *testing.T) {
	// BuildCCConfig returns 2 concatenated messages: CC config + latching config.
	// Each message is: F0 + data(17) + checksum(2) + F7 = 21 bytes.
	cc0 := knownExamples[0]     // CC=0 bank1
	latch0 := knownExamples[15] // Momentary bank1
	data := BuildCCConfig(0, 0, false, 0)

	// Build expected by constructing message from known data
	msgLen := len(cc0.data) + 4 // F0 + data + 2 checksum + F7
	if len(data) != 2*msgLen {
		t.Fatalf("BuildCCConfig(0,0,false,0): len=%d, expected %d", len(data), 2*msgLen)
	}

	ccExpected := make([]byte, msgLen)
	ccExpected[0] = 0xF0
	copy(ccExpected[1:], cc0.data)
	ccExpected[len(cc0.data)+1] = cc0.csLow
	ccExpected[len(cc0.data)+2] = cc0.csHigh
	ccExpected[len(cc0.data)+3] = 0xF7

	latchExpected := make([]byte, msgLen)
	latchExpected[0] = 0xF0
	copy(latchExpected[1:], latch0.data)
	latchExpected[len(latch0.data)+1] = latch0.csLow
	latchExpected[len(latch0.data)+2] = latch0.csHigh
	latchExpected[len(latch0.data)+3] = 0xF7

	expected := append(ccExpected, latchExpected...)

	if !hexEq(hex.EncodeToString(data), hex.EncodeToString(expected)) {
		t.Errorf("BuildCCConfig(0,0,false,0):\n  got: %s\n  exp: %s",
			hex.EncodeToString(data), hex.EncodeToString(expected))
	}
}

// TestBuildDiscovery verifies discovery message.
func TestBuildDiscovery(t *testing.T) {
	testHex(t, "BuildDiscovery", "F0003245000000407FF7", BuildDiscovery())
}

// TestBuildReadSettings verifies read settings message.
func TestBuildReadSettings(t *testing.T) {
	testHex(t, "BuildReadSettings", "F000320D410000000200000000107E00000700F7", BuildReadSettings())
}

// TestSysexChecksumProperty verifies checksum properties with quick.
func TestSysexChecksumProperty(t *testing.T) {
	// The checksum should be deterministic: same input always gives the same output.
	fn := func(data []byte) bool {
		clamped := make([]byte, len(data))
		for i, b := range data {
			clamped[i] = b & 0x7F
		}
		l1, h1 := SysexChecksum(clamped)
		l2, h2 := SysexChecksum(clamped)
		return l1 == l2 && h1 == h2
	}
	if err := quick.Check(fn, nil); err != nil {
		t.Error(err)
	}
}
