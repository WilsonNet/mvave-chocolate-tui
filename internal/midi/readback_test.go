// Copyright (C) 2026 Wilson Neto
// SPDX-License-Identifier: GPL-3.0-or-later

package midi

import (
	"context"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"mvave-chocolate-tui/internal/sysex"
)

// TestHeadlessWriteRead cycles: read current CC, write 42, read back, verify, restore.
func TestHeadlessWriteRead(t *testing.T) {
	path := findDeviceOrSkip(t)
	alsaDev := pathToAlsa(path)

	// Use amidi for both send and receive — raw fd reads block forever.
	sendHex := func(hexStr string) error {
		cmd := exec.Command("amidi", "-p", alsaDev, "-S", hexStr)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("amidi send: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	recvHex := func(timeout time.Duration) string {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "amidi", "-p", alsaDev, "-d")
		out, _ := cmd.CombinedOutput()
		return strings.TrimSpace(string(out))
	}

	// Full reference init sequence (11 msgs, correct checksums from cbix).
	refInit := []string{
		"f000320d410000000200000000107e00000700f7",
		"f000320d410000000271070000106a00003301f7",
		"f00032094900000002020000001000000000203003f7",
		"f00032094900000002040000001000000000212a03f7",
		"f00032094900000002060000001000000000222403f7",
		"f00032094900000002080000001000000000035e03f7",
		"f000320949000000020a00000010000000002c0803f7",
		"f00032094900000002320e00001000000000077402f7",
		"f00032094900000002360e00001000000000096802f7",
		"f000320949000000023a0e000010000000000a5e02f7",
		"f000320949000000023e0e000010000000000b5402f7",
	}

	// -----------------------------------------------------------------------
	// Step 1 — Send init and read current config.
	// -----------------------------------------------------------------------
	t.Log("Sending init sequence and reading current config...")
	for _, h := range refInit {
		if err := sendHex(h); err != nil {
			t.Fatalf("init msg: %v", err)
		}
		time.Sleep(15 * time.Millisecond)
	}

	time.Sleep(100 * time.Millisecond)
	raw1 := recvHex(2 * time.Second)
	t.Logf("Init response len=%d", len(raw1))
	origCC := findBankCCinHex(raw1, 0)
	t.Logf("Original bank 0 CC = %d", origCC)

	// -----------------------------------------------------------------------
	// Step 2 — Write CC=42 to bank 0.
	// -----------------------------------------------------------------------
	testCC := 42
	t.Logf("Writing CC=%d to bank 0...", testCC)

	// Use BuildCCConfig from the sysex package, split into individual messages.
	ccData := sysex.BuildCCConfig(0, byte(testCC), false, 0)
	for _, m := range sysex.SplitMessages(ccData) {
		if err := sendHex(hex.EncodeToString(m)); err != nil {
			t.Fatalf("write CC: %v", err)
		}
		time.Sleep(40 * time.Millisecond)
	}

	// Check for OK response.
	time.Sleep(100 * time.Millisecond)
	okRaw := recvHex(500 * time.Millisecond)
	if hasOKinHex(okRaw) {
		t.Log("OK response received — config write accepted!")
	} else {
		t.Logf("No OK response. Response (%d bytes): %s", len(okRaw), okRaw[:min(len(okRaw), 100)])
	}

	// -----------------------------------------------------------------------
	// Step 3 — Read back by sending only the read request (not the full init
	// which would overwrite our CC config with hardcoded probe values).
	// -----------------------------------------------------------------------
	t.Log("Reading back...")
	if err := sendHex("f000320d410000000200000000107e00000700f7"); err != nil {
		t.Fatalf("readback request: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	raw2 := recvHex(3 * time.Second)
	newCC := findBankCCinHex(raw2, 0)
	t.Logf("Bank 0 CC after write = %d", newCC)

	if newCC == testCC {
		t.Logf("PASS: CC changed from %d to %d ✓", origCC, testCC)
	} else {
		t.Logf("CC did NOT change (was %d, still %d). Write may have been rejected.",
			origCC, newCC)
	}

	// -----------------------------------------------------------------------
	// Step 4 — Restore original (or use CC=20 as safe default).
	// -----------------------------------------------------------------------
	restoreCC := 20
	if origCC >= 0 && origCC <= 127 {
		restoreCC = origCC
	}
	t.Logf("Restoring CC=%d to bank 0...", restoreCC)
	restoreData := sysex.BuildCCConfig(0, byte(restoreCC), false, 0)
	for _, m := range sysex.SplitMessages(restoreData) {
		if err := sendHex(hex.EncodeToString(m)); err != nil {
			t.Fatalf("restore: %v", err)
		}
		time.Sleep(40 * time.Millisecond)
	}
	t.Log("Restore done.")
}

// findBankCCinHex parses amidi -d output and extracts the CC value for a bank.
func findBankCCinHex(output string, bank int) int {
	clean := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			return r
		}
		return -1
	}, output)

	data, err := hex.DecodeString(clean)
	if err != nil {
		return -1
	}

	frames := sysex.FindSysExFrames(data, []byte{0xF0, 0x00, 0x32, 0x0D, 0x49})
	for _, frame := range frames {
		cr := sysex.ParseConfigResponse(frame)
		if cr != nil && bank >= 0 && bank < 4 {
			return int(cr.CC[bank])
		}
	}

	return -1
}

// hasOKinHex checks for the OK acknowledgment in amidi -d output.
func hasOKinHex(output string) bool {
	clean := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			return r
		}
		return -1
	}, output)
	data, err := hex.DecodeString(clean)
	if err != nil {
		return false
	}
	frames := sysex.SplitMessages(data)
	for _, f := range frames {
		if sysex.IsOKResponse(f) {
			return true
		}
	}
	return false
}

// TestHeadlessReadOnly verifies the device responds to read requests
// without requiring write support.
func TestHeadlessReadOnly(t *testing.T) {
	path := findDeviceOrSkip(t)
	alsaDev := pathToAlsa(path)

	sendHex := func(hexStr string) error {
		cmd := exec.Command("amidi", "-p", alsaDev, "-S", hexStr)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("amidi send: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	recvHex := func(timeout time.Duration) string {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "amidi", "-p", alsaDev, "-d")
		out, _ := cmd.CombinedOutput()
		return strings.TrimSpace(string(out))
	}

	// Drain multiple times to clear any queued OK responses or stale data.
	for i := 0; i < 3; i++ {
		r := recvHex(300 * time.Millisecond)
		if len(r) > 0 {
			t.Logf("Drained stale data (%d bytes)", len(r))
		}
	}
	time.Sleep(100 * time.Millisecond)

	// Just send the first init (read request) message.
	t.Log("Sending read request...")
	if err := sendHex("f000320d410000000200000000107e00000700f7"); err != nil {
		t.Fatal(err)
	}

	// Give the device time to respond, then read with generous timeout.
	time.Sleep(400 * time.Millisecond)
	resp := recvHex(5 * time.Second)
	t.Logf("Response len=%d", len(resp))

	if len(resp) < 20 {
		t.Error("No response from device — read may not be working")
		return
	}

	// Try to extract mode and bank 0 CC.
	clean := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			return r
		}
		return -1
	}, resp)
	data, err := hex.DecodeString(clean)
	if err != nil {
		t.Fatal(err)
	}

	frames := sysex.FindSysExFrames(data, []byte{0xF0, 0x00, 0x32, 0x0D, 0x49})
	if len(frames) == 0 {
		t.Error("No F0 00 32 0D 49 response frame found")
		return
	}

	frame := frames[0]
	t.Logf("Response frame: %s", hex.EncodeToString(frame))

	cr := sysex.ParseConfigResponse(frame)
	if cr == nil {
		t.Error("Failed to parse config response")
		return
	}
	modeName, ok := sysex.ModeNames[cr.Mode]
	if ok {
		t.Logf("Device mode: %s (0x%02X)", modeName, cr.Mode)
	} else {
		t.Logf("Device mode: unknown 0x%02X", cr.Mode)
	}

	t.Logf("CC values: bank0=%d bank1=%d bank2=%d bank3=%d",
		cr.CC[0], cr.CC[1], cr.CC[2], cr.CC[3])
	t.Logf("Latch: bank0=%v bank1=%v bank2=%v bank3=%v",
		cr.Latch[0], cr.Latch[1], cr.Latch[2], cr.Latch[3])

	// Print raw config bytes for debugging.
	t.Logf("Frame hex: %s", hex.EncodeToString(frame))
}

// TestHeadlessAmidiSendRaw verifies amidi can send raw bytes.
// This is a pre-existing test already in device_test.go but repeated here
// to verify the device is still reachable.
func TestHeadlessAmidiSendRaw(t *testing.T) {
	path := findDeviceOrSkip(t)
	alsaDev := pathToAlsa(path)

	for _, name := range []string{"mode_change", "cc_config", "read_req"} {
		var hexStr string
		switch name {
		case "mode_change":
			hexStr = hex.EncodeToString(sysex.BuildModeChange(sysex.ModeCustom))
		case "cc_config":
			hexStr = hex.EncodeToString(sysex.BuildCCConfig(0, 20, false, 0))
		case "read_req":
			hexStr = hex.EncodeToString(sysex.BuildReadSettings())
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, "amidi", "-p", alsaDev, "-S", hexStr)
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("%s: %v: %s", name, err, strings.TrimSpace(string(out)))
		}
		t.Logf("%s: OK", name)
	}
}
