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

	cc := func(ch, num, val byte) PlusMessage {
		return PlusMessage{Type: PlusCC, Channel: ch, Number: num, Value: val}
	}
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

// Switch C's Bank B starts at byte 1008 (its first message's "in use" byte):
// the last, short byte of the first reply. Dropping a partial final byte
// loses it.
func TestTheShortFinalByteIsKept(t *testing.T) {
	img := loadImage(t, "plus-read1.syx")
	if _, ok := img[1008]; !ok {
		t.Fatal("byte 1008 missing")
	}
	if img[1008] != 1 {
		t.Errorf("byte 1008 (switch C bank B message 1 in use) = %d, want 1", img[1008])
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

// Adding a message, as the official app did on a real pedal: its "in use"
// byte (99) went 0 -> 1 and nothing else changed -- the rest was already the
// app's default, channel 1 PC 0 0. Here the flag must be written FIRST.
func TestAddingAMessageWritesItsFlagFirst(t *testing.T) {
	img := loadImage(t, "plus-read1.syx", "plus-read2.syx")
	want := img.DecodePlus()
	want.Switches[0].Banks[0] = append(want.Switches[0].Banks[0],
		PlusMessage{Type: PlusCC, Channel: 2, Number: 64, Value: 127})
	ch, err := img.PlusChanges(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(ch) == 0 || ch[0] != (Write{Addr: 99, Old: 0, New: 1}) {
		t.Fatalf("first write %+v; want the in-use flag at 99", ch)
	}
	got := map[int]byte{}
	for _, w := range ch[1:] {
		got[w.Addr] = w.New
	}
	if got[100] != 1 || got[101] != PlusCC || got[102] != 64 || got[103] != 127 {
		t.Errorf("fields %v; want ch 2 (stored 1), CC, 64, 127", got)
	}
}

// Removing the message again, as done on the real pedal: only after its
// fields are back to 0 does its flag go, so the slot ends as a never-used one.
func TestRemovingTheLastMessageClearsFieldsThenFlag(t *testing.T) {
	img := loadImage(t, "plus-read1.syx", "plus-read2.syx")
	img[99], img[100], img[101], img[102], img[103] = 1, 1, PlusCC, 64, 127 // a second message
	want := img.DecodePlus()
	if len(want.Switches[0].Banks[0]) != 2 {
		t.Fatalf("the second message was not decoded")
	}
	want.Switches[0].Banks[0] = want.Switches[0].Banks[0][:1]
	ch, err := img.PlusChanges(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(ch) != 5 || ch[len(ch)-1] != (Write{Addr: 99, Old: 1, New: 0}) {
		t.Errorf("writes %+v; want four fields cleared, then 99 -> 0", ch)
	}
}

func TestPlusChangesRefusesWhatItCannotDo(t *testing.T) {
	img := loadImage(t, "plus-read1.syx", "plus-read2.syx")

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
	sysex := img.DecodePlus()
	sysex.Switches[2].Banks[0][0].Type = PlusSysEx
	if _, err := img.PlusChanges(sysex); err == nil {
		t.Error("SysEx (unmapped) was allowed")
	}
	gapped := loadImage(t, "plus-read1.syx", "plus-read2.syx")
	gapped[104] = 1 // message 3 in use, message 2 not
	if _, err := gapped.PlusChanges(gapped.DecodePlus()); err == nil {
		t.Error("a bank with a gap was edited")
	}
}

// What the official app showed on a real pedal, set one at a time on switch A
// and read back: the stored bytes for each type, and channel 2.
func TestTheMappedMessageEncoding(t *testing.T) {
	img := loadImage(t, "plus-read1.syx", "plus-read2.syx")
	for _, tc := range []struct {
		ch, typ    byte
		at95, at96 byte
	}{
		{1, PlusPC, 0, 0}, {2, PlusCC, 1, 1}, {1, PlusNoteOn, 0, 2}, {1, PlusNoteOff, 0, 3},
	} {
		want := img.DecodePlus()
		want.Switches[0].Banks[0][0].Channel, want.Switches[0].Banks[0][0].Type = tc.ch, tc.typ
		ch, err := img.PlusChanges(want)
		if err != nil {
			t.Fatal(err)
		}
		got := map[int]byte{95: img[95], 96: img[96]}
		for _, w := range ch {
			got[w.Addr] = w.New
		}
		if got[95] != tc.at95 || got[96] != tc.at96 {
			t.Errorf("ch %d %s: 95=%d 96=%d, want %d %d", tc.ch, PlusTypeNames[tc.typ], got[95], got[96], tc.at95, tc.at96)
		}
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

// All five switch modes were set in the official app and read back.
func TestEverySwitchModeIsWritable(t *testing.T) {
	img := loadImage(t, "plus-read1.syx", "plus-read2.syx")
	for mode := byte(0); mode <= 4; mode++ {
		want := img.DecodePlus()
		want.Switches[0].Mode = mode
		ch, err := img.PlusChanges(want)
		if err != nil {
			t.Errorf("mode %d: %v", mode, err)
		}
		if mode != 1 && (len(ch) != 1 || ch[0].Addr != 93 || ch[0].New != mode) {
			t.Errorf("mode %d: writes %+v; want one, at 93", mode, ch)
		}
	}
}
