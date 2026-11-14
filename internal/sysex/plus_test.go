package sysex

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// The plus-read*.syx fixtures are the two replies from a real Chocolate Plus
// whose settings were read off the official app's screens first:
//
//	A, B, C: single step, both banks ch1 CC107 = 1 / 2 / 3 (preset select)
//	D:       step short / step long, ch1 CC5 = 127 | ch1 CC6 = 127
//
// original-read*.syx are an original Chocolate's replies, from
// cbix/mvave-chocolate-sysex.

func loadImage(t *testing.T, names ...string) Image {
	t.Helper()
	img := Image{}
	for _, n := range names {
		frame, err := os.ReadFile("testdata/" + n)
		if err != nil {
			t.Fatal(err)
		}
		if !img.Add(frame) {
			t.Fatalf("%s is not a read reply", n)
		}
	}
	return img
}

func TestDecodeReadReplyAddressesAndLengths(t *testing.T) {
	for _, tc := range []struct {
		name       string
		addr, size int
	}{
		{"plus-read1.syx", 0, 1009},
		{"plus-read2.syx", 1009, 849},
	} {
		frame, _ := os.ReadFile("testdata/" + tc.name)
		addr, data, ok := DecodeReadReply(frame)
		if !ok || addr != tc.addr || len(data) != tc.size {
			t.Errorf("%s: ok=%v addr=%d len=%d, want addr=%d len=%d",
				tc.name, ok, addr, len(data), tc.addr, tc.size)
		}
	}
}

func TestPlusDecodesTheKnownConfiguration(t *testing.T) {
	img := loadImage(t, "plus-read1.syx", "plus-read2.syx")
	if !img.PlusComplete() {
		t.Fatal("image incomplete after both reads")
	}
	if !img.LooksLikePlus() {
		t.Fatal("a Plus read was not recognised as a Plus")
	}
	cfg := img.DecodePlus()
	if PlusModeNames[cfg.Mode] != "Advanced custom mode" {
		t.Errorf("device mode %d (%q), want Advanced custom mode", cfg.Mode, PlusModeNames[cfg.Mode])
	}

	cc := func(ch, num, val byte) PlusMessage { return PlusMessage{Type: 0, Channel: ch, Number: num, Value: val} }
	want := [4]PlusSwitch{
		{Mode: 1, Banks: [2][]PlusMessage{{cc(1, 107, 1)}, {cc(1, 107, 1)}}},
		{Mode: 1, Banks: [2][]PlusMessage{{cc(1, 107, 2)}, {cc(1, 107, 2)}}},
		{Mode: 1, Banks: [2][]PlusMessage{{cc(1, 107, 3)}, {cc(1, 107, 3)}}},
		{Mode: 4, Banks: [2][]PlusMessage{{cc(1, 5, 127)}, {cc(1, 6, 127)}}},
	}
	for s, w := range want {
		got := cfg.Switches[s]
		if got.Mode != w.Mode {
			t.Errorf("switch %c: mode %d, want %d", 'A'+s, got.Mode, w.Mode)
		}
		for b := 0; b < 2; b++ {
			if len(got.Banks[b]) != len(w.Banks[b]) {
				t.Errorf("switch %c bank %c: %d messages, want %d", 'A'+s, 'A'+b, len(got.Banks[b]), len(w.Banks[b]))
				continue
			}
			for i := range w.Banks[b] {
				if got.Banks[b][i] != w.Banks[b][i] {
					t.Errorf("switch %c bank %c msg %d: %+v, want %+v", 'A'+s, 'A'+b, i, got.Banks[b][i], w.Banks[b][i])
				}
			}
		}
	}
}

// Switch C's Bank B starts at byte 1008: the last, short byte of the first
// reply. Dropping a partial final byte loses it.
func TestTheShortFinalByteIsKept(t *testing.T) {
	img := loadImage(t, "plus-read1.syx")
	if _, ok := img[1008]; !ok {
		t.Fatal("byte 1008 missing")
	}
	if img[1008] != 1 {
		t.Errorf("byte 1008 (switch C bank B count) = %d, want 1", img[1008])
	}
}

func TestOneReadIsNotComplete(t *testing.T) {
	if loadImage(t, "plus-read1.syx").PlusComplete() {
		t.Error("switch D lives in the second reply; one read cannot be complete")
	}
}

func TestTheOriginalChocolateIsNotTakenForAPlus(t *testing.T) {
	if loadImage(t, "original-read1.syx", "original-read2.syx").LooksLikePlus() {
		t.Error("an original Chocolate's dump was recognised as a Plus")
	}
}

func hexBytes(t *testing.T, s string) []byte {
	t.Helper()
	var out []byte
	for _, f := range strings.Fields(s) {
		var b byte
		if _, err := fmt.Sscanf(f, "%02X", &b); err != nil {
			t.Fatalf("bad hex %q", f)
		}
		out = append(out, b)
	}
	return out
}

// Writes a real Chocolate Plus ACCEPTED, byte for byte, on 14-11-2026.
func TestPlusWritesTheDeviceAccepted(t *testing.T) {
	for _, tc := range []struct {
		addr  int
		value byte
		want  string
	}{
		{97, 107, "F0 00 32 09 49 00 00 00 02 61 00 00 00 10 00 00 00 6B 5C 00 F7"},
		{178, 1, "F0 00 32 09 49 00 00 00 02 32 01 00 00 10 00 00 00 01 0E 01 F7"},
		{178, 4, "F0 00 32 09 49 00 00 00 02 32 01 00 00 10 00 00 00 04 08 01 F7"},
	} {
		got, err := BuildWrite(tc.addr, tc.value)
		if err != nil {
			t.Fatal(err)
		}
		if want := hexBytes(t, tc.want); string(got) != string(want) {
			t.Errorf("write %d=%d:\n got % X\nwant % X", tc.addr, tc.value, got, want)
		}
	}
}

// The old formula's checksum for 178=1 (397 = 0x0D 0x03). The device ignored
// it; nothing may build it again.
func TestTheRejectedChecksumIsNotBuilt(t *testing.T) {
	got, _ := BuildWrite(178, 1)
	if got[18] == 0x0D && got[19] == 0x03 {
		t.Error("built the checksum the device refused")
	}
}

func TestPlusWriteRefusesWhatCannotBeSent(t *testing.T) {
	if _, err := BuildWrite(1<<14, 0); err == nil {
		t.Error("an address wider than two 7-bit bytes was accepted")
	}
	if _, err := BuildWrite(0, 0x80); err == nil {
		t.Error("a value over 127 was accepted")
	}
}

func TestChangesWritesOnlyWhatDiffers(t *testing.T) {
	img := loadImage(t, "plus-read1.syx", "plus-read2.syx")
	want := img.DecodePlus()
	if ch, err := img.PlusChanges(want); err != nil || len(ch) != 0 {
		t.Fatalf("no edits should mean no writes; got %v, %v", ch, err)
	}
	// The exact change made on the real pedal: A's Bank B value 1 -> 4.
	want.Switches[0].Banks[1][0].Value = 4
	ch, err := img.PlusChanges(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(ch) != 1 || ch[0] != (Write{Addr: 178, Old: 1, New: 4}) {
		t.Errorf("got %+v, want one write 178: 1 -> 4", ch)
	}
}

func TestChangesRefusesWhatIsUntested(t *testing.T) {
	img := loadImage(t, "plus-read1.syx", "plus-read2.syx")

	add := img.DecodePlus()
	add.Switches[0].Banks[0] = append(add.Switches[0].Banks[0], PlusMessage{Channel: 1, Number: 1})
	if _, err := img.PlusChanges(add); err == nil {
		t.Error("adding a message (a count change, untested) was allowed")
	}

	mode := img.DecodePlus()
	mode.Switches[1].Mode = 9
	if _, err := img.PlusChanges(mode); err == nil {
		t.Error("an unknown switch mode was allowed")
	}

	ch0 := img.DecodePlus()
	ch0.Switches[2].Banks[0][0].Channel = 0
	if _, err := img.PlusChanges(ch0); err == nil {
		t.Error("channel 0 was allowed")
	}
}

// Detection must not hang on this pedal's particular settings: change the
// header values the old check relied on, and the CCs, and it is still a Plus.
func TestAPlusWithOtherSettingsIsStillAPlus(t *testing.T) {
	img := loadImage(t, "plus-read1.syx", "plus-read2.syx")
	for a := 2; a <= 11; a++ {
		img[a] = 99
	}
	img[97], img[1348] = 64, 80
	if !img.LooksLikePlus() {
		t.Error("a Plus with different settings was not recognised")
	}
}

// Named from M-VAVE's manual is not the same as checked on a pedal: a mode
// with a name but no hardware check is shown, never written.
func TestAManualOnlySwitchModeIsNotWritten(t *testing.T) {
	img := loadImage(t, "plus-read1.syx", "plus-read2.syx")
	want := img.DecodePlus()
	want.Switches[0].Mode = 2 // press / release
	if _, err := img.PlusChanges(want); err == nil {
		t.Error("an unchecked switch mode would be written")
	}
	if PlusSwitchModeNames[2] != "press / release" {
		t.Errorf("mode 2 named %q", PlusSwitchModeNames[2])
	}
}
