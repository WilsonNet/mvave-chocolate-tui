// Copyright (C) 2026 Wilson Neto
// SPDX-License-Identifier: GPL-3.0-or-later

package midi

import (
	"context"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strconv"
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

// TestHeadlessAdvancedCustom48 is a diagnostic test for the CC=48 persistence bug.
//
// Two sub-tests:
//   - "direct": set Advanced Custom mode, write CC=48 (no init sequence), read back
//   - "with_init": set Advanced Custom, then send full init (which forces ModeCustom),
//     write CC=48, read back — exposes the mode-override bug
//
// Run: go test -v -run TestHeadlessAdvancedCustom48 ./internal/midi/
func TestHeadlessAdvancedCustom48(t *testing.T) {
	path := findDeviceOrSkip(t)
	alsaDev := pathToAlsa(path)

	stripHex := func(raw string) []byte {
		clean := strings.Map(func(r rune) rune {
			if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
				return r
			}
			return -1
		}, raw)
		data, _ := hex.DecodeString(clean)
		return data
	}

	// sendOnly sends a SysEx without waiting for response (fire-and-forget).
	sendOnly := func(hexStr string) error {
		cmd := exec.Command("amidi", "-p", alsaDev, "-S", hexStr)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("amidi -S: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	// sendRecv sends a SysEx AND listens for the response in one port-open window.
	// This avoids the race where the device's OK response arrives while the port
	// is closed between separate send and receive amidi invocations.
	// amidi --timeout is in SECONDS (inactivity), not milliseconds.
	sendRecv := func(hexStr string, timeout time.Duration) []byte {
		secs := int(timeout.Seconds())
		if secs < 1 {
			secs = 1
		}
		cmd := exec.Command("amidi", "-p", alsaDev,
			"-S", hexStr, "-d", "--timeout", strconv.Itoa(secs))
		raw, _ := cmd.CombinedOutput()
		t.Logf("    amidi raw: %q", strings.TrimSpace(string(raw)))
		return stripHex(string(raw))
	}

	// drain flushes any pending device output (OK responses from prior commands).
	// 1-second inactivity timeout is enough to clear stale bytes.
	drain := func() {
		cmd := exec.Command("amidi", "-p", alsaDev, "-d", "--timeout", "1")
		_, _ = cmd.CombinedOutput()
	}

	logFrames := func(t *testing.T, label string, data []byte) *sysex.ConfigResponse {
		t.Helper()
		frames := sysex.FindSysExFrames(data, []byte{0xF0, 0x00, 0x32, 0x0D, 0x49})
		t.Logf("%s: %d config frame(s), raw=%d bytes", label, len(frames), len(data))
		for i, f := range frames {
			t.Logf("  frame[%d]: %s", i, hex.EncodeToString(f))
			if cr := sysex.ParseConfigResponse(f); cr != nil {
				name := sysex.ModeNames[cr.Mode]
				t.Logf("  parsed: mode=0x%02X(%s) CC=[%d %d %d %d] latch=[%v %v %v %v]",
					cr.Mode, name, cr.CC[0], cr.CC[1], cr.CC[2], cr.CC[3],
					cr.Latch[0], cr.Latch[1], cr.Latch[2], cr.Latch[3])
				return cr
			}
		}
		return nil
	}

	hasOK := func(data []byte) bool {
		frames := sysex.FindSysExFrames(data, []byte{0xF0, 0x00, 0x32, 0x01, 0x08})
		return len(frames) > 0
	}

	const targetCC = 48

	// -----------------------------------------------------------------------
	// Part A — read initial state: what does the device have before any write?
	// Establishes baseline so we can tell if our writes change anything.
	// -----------------------------------------------------------------------
	t.Run("initial_read", func(t *testing.T) {
		drain()
		resp := sendRecv(hex.EncodeToString(sysex.BuildReadSettings()), 3*time.Second)
		logFrames(t, "initial device state", resp)
	})

	// -----------------------------------------------------------------------
	// Part B — Custom CC round-trip: switch to Custom CC, write CC=48, read back.
	// Confirms the basic send→readback path works before testing Advanced Custom.
	// -----------------------------------------------------------------------
	t.Run("custom_cc", func(t *testing.T) {
		drain()

		t.Log("Switching to Custom CC mode...")
		modeAck := sendRecv(hex.EncodeToString(sysex.BuildModeChange(sysex.ModeCustom)), 2*time.Second)
		if hasOK(modeAck) {
			t.Log("  OK for Custom CC mode change")
		} else {
			t.Fatalf("no OK for mode change — cannot proceed")
		}

		t.Log("Reading state before write...")
		before := sendRecv(hex.EncodeToString(sysex.BuildReadSettings()), 3*time.Second)
		crBefore := logFrames(t, "before write", before)

		t.Logf("Writing CC=%d to bank 0...", targetCC)
		ccData := sysex.BuildCCConfig(0, byte(targetCC), false, 0)
		for i, msg := range sysex.SplitMessages(ccData) {
			ack := sendRecv(hex.EncodeToString(msg), 2*time.Second)
			if hasOK(ack) {
				t.Logf("  msg[%d] OK", i)
			} else {
				t.Errorf("  msg[%d] no OK — write rejected", i)
			}
		}

		t.Log("Reading back...")
		after := sendRecv(hex.EncodeToString(sysex.BuildReadSettings()), 3*time.Second)
		crAfter := logFrames(t, "after write", after)
		if crAfter == nil {
			t.Fatal("no parseable config response after write")
		}

		if crBefore != nil {
			t.Logf("Bank 0 CC: %d → %d (expected %d)", crBefore.CC[0], crAfter.CC[0], targetCC)
		}
		if int(crAfter.CC[0]) == targetCC {
			t.Logf("PASS: Custom CC bank 0 CC=%d persists in readback", targetCC)
		} else {
			t.Errorf("FAIL: expected CC=%d, got %d in Custom CC mode", targetCC, crAfter.CC[0])
		}
	})

	// -----------------------------------------------------------------------
	// Part C — Advanced Custom write via standard subcmds (0x02/0x03).
	// Hypothesis: subcmd 0x02 writes target Custom CC config space, NOT Advanced
	// Custom's per-switch space. Write is accepted (OK) but readback shows no change.
	// -----------------------------------------------------------------------
	t.Run("advanced_custom_std_subcmd", func(t *testing.T) {
		drain()

		t.Log("Switching to Advanced Custom mode...")
		modeAck := sendRecv(hex.EncodeToString(sysex.BuildModeChange(sysex.ModeAdvancedCustom)), 2*time.Second)
		if hasOK(modeAck) {
			t.Log("  OK for Advanced Custom mode change")
		}

		t.Log("Reading state before write...")
		before := sendRecv(hex.EncodeToString(sysex.BuildReadSettings()), 3*time.Second)
		crBefore := logFrames(t, "before write", before)

		t.Logf("Writing CC=%d via subcmd 0x02 (Custom CC subcmd)...", targetCC)
		ccData := sysex.BuildCCConfig(0, byte(targetCC), false, 0)
		for i, msg := range sysex.SplitMessages(ccData) {
			ack := sendRecv(hex.EncodeToString(msg), 2*time.Second)
			if hasOK(ack) {
				t.Logf("  msg[%d] OK", i)
			} else {
				t.Logf("  msg[%d] no OK", i)
			}
		}

		t.Log("Reading back...")
		after := sendRecv(hex.EncodeToString(sysex.BuildReadSettings()), 3*time.Second)
		crAfter := logFrames(t, "after write", after)
		if crAfter == nil {
			t.Fatal("no config response")
		}
		if crBefore != nil {
			t.Logf("Bank 0 CC: %d → %d (expected %d)", crBefore.CC[0], crAfter.CC[0], targetCC)
		}
		if int(crAfter.CC[0]) == targetCC {
			t.Log("PASS: CC=48 persisted in Advanced Custom readback (subcmd 0x02 works here)")
		} else {
			t.Logf("NOTE: CC unchanged (%d). Subcmd 0x02 targets Custom CC space, not Advanced Custom. "+
				"Advanced Custom needs different write subcmds.", crAfter.CC[0])
		}
	})

	// -----------------------------------------------------------------------
	// Part D — byte-diff: write a distinctive CC value and compare full response
	// before/after to find WHERE in the 1173-byte dump the value lands.
	// Uses CC=99 (0x63) — unlikely to appear by chance in the response.
	// -----------------------------------------------------------------------
	t.Run("byte_diff", func(t *testing.T) {
		drain()

		// Baseline read.
		before := sendRecv(hex.EncodeToString(sysex.BuildReadSettings()), 3*time.Second)
		t.Logf("baseline: %d bytes", len(before))

		// Write distinctive CC=99 in Custom CC mode.
		if err := sendOnly(hex.EncodeToString(sysex.BuildModeChange(sysex.ModeCustom))); err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
		ccData := sysex.BuildCCConfig(0, 99, false, 0)
		for _, msg := range sysex.SplitMessages(ccData) {
			ack := sendRecv(hex.EncodeToString(msg), 2*time.Second)
			if hasOK(ack) {
				t.Log("CC write OK")
			}
		}

		// Read after write.
		after := sendRecv(hex.EncodeToString(sysex.BuildReadSettings()), 3*time.Second)
		t.Logf("after write: %d bytes", len(after))

		// Find all positions where before and after differ.
		changed := []int{}
		for i := 0; i < len(before) && i < len(after); i++ {
			if before[i] != after[i] {
				changed = append(changed, i)
			}
		}

		if len(changed) == 0 {
			t.Log("FINDING: no bytes changed in response after CC=99 write!")
			t.Log("  → The 0D41 read request does NOT reflect CC config writes")
			t.Log("  → Parser position 02/40 is FIXED data, not live CC value")
		} else {
			t.Logf("FINDING: %d byte(s) changed after CC=99 write:", len(changed))
			for _, pos := range changed {
				t.Logf("  [%d] 0x%02X → 0x%02X", pos, before[pos], after[pos])
			}
		}

		// Search for 0x63 (99) in the after response.
		t.Log("Positions containing 0x63 (CC=99) in after-write response:")
		found := false
		for i, b := range after {
			if b == 0x63 {
				t.Logf("  [%d] = 0x63", i)
				found = true
			}
		}
		if !found {
			t.Log("  → 0x63 not found anywhere — CC write did NOT update the config dump")
		}
	})

	// -----------------------------------------------------------------------
	// Part E — Full init sequence then CC write.
	// Init always sends BuildModeChange(ModeCustom). After the init, we re-assert
	// the user's mode (the fix applied to sendAllConfig). Verify CC+mode persist.
	// -----------------------------------------------------------------------
	t.Run("with_init_mode_reasserted", func(t *testing.T) {
		drain()

		t.Log("Sending full BuildInitSequence (forces ModeCustom internally)...")
		for i, msg := range sysex.BuildInitSequence() {
			if err := sendOnly(hex.EncodeToString(msg)); err != nil {
				t.Fatalf("init msg %d: %v", i, err)
			}
			time.Sleep(30 * time.Millisecond)
		}

		t.Log("Re-asserting Advanced Custom mode (the fix)...")
		modeAck := sendRecv(hex.EncodeToString(sysex.BuildModeChange(sysex.ModeAdvancedCustom)), 2*time.Second)
		if hasOK(modeAck) {
			t.Log("  OK for mode re-assertion")
		} else {
			t.Log("  no OK for mode re-assertion")
		}

		t.Logf("Writing CC=%d...", targetCC)
		ccData := sysex.BuildCCConfig(0, byte(targetCC), false, 0)
		for i, msg := range sysex.SplitMessages(ccData) {
			ack := sendRecv(hex.EncodeToString(msg), 2*time.Second)
			if hasOK(ack) {
				t.Logf("  msg[%d] OK", i)
			} else {
				t.Logf("  msg[%d] no OK", i)
			}
		}

		t.Log("Reading back...")
		resp := sendRecv(hex.EncodeToString(sysex.BuildReadSettings()), 3*time.Second)
		cr := logFrames(t, "readback", resp)
		if cr == nil {
			t.Fatal("no config response")
		}
		t.Logf("Mode: 0x%02X (%s), Bank0 CC: %d", cr.Mode, sysex.ModeNames[cr.Mode], cr.CC[0])
		if int(cr.CC[0]) == targetCC {
			t.Log("PASS: CC=48 persists after init+mode-reassert+write")
		} else {
			t.Logf("FAIL: CC=%d expected %d", cr.CC[0], targetCC)
		}
	})
}

// TestHeadlessProbeReadback explores which read commands return live CC data.
//
// Sub-tests:
//   - "mode_liveness": checks if mode byte in 0D41 response reflects live mode changes
//   - "init_msg2": sends the second init sequence read request (0D41 variant with 0x71 subcmd)
//     and captures its response — may return live CC data
//   - "advcustom_subcmds": tries sending CC writes with subcmds in the 0x20-0x3F range
//     that the init sequence uses for per-switch type init, to see if those affect readback
func TestHeadlessProbeReadback(t *testing.T) {
	path := findDeviceOrSkip(t)
	alsaDev := pathToAlsa(path)

	stripHex := func(raw string) []byte {
		clean := strings.Map(func(r rune) rune {
			if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
				return r
			}
			return -1
		}, raw)
		data, _ := hex.DecodeString(clean)
		return data
	}

	sendRecv := func(hexStr string, timeout time.Duration) []byte {
		secs := int(timeout.Seconds())
		if secs < 1 {
			secs = 1
		}
		cmd := exec.Command("amidi", "-p", alsaDev,
			"-S", hexStr, "-d", "--timeout", strconv.Itoa(secs))
		raw, _ := cmd.CombinedOutput()
		t.Logf("    amidi raw (%d chars): %q", len(raw), strings.TrimSpace(string(raw))[:min(len(strings.TrimSpace(string(raw))), 200)])
		return stripHex(string(raw))
	}

	sendOnly := func(hexStr string) error {
		cmd := exec.Command("amidi", "-p", alsaDev, "-S", hexStr)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("amidi -S: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	drain := func() {
		cmd := exec.Command("amidi", "-p", alsaDev, "-d", "--timeout", "1")
		_, _ = cmd.CombinedOutput()
	}

	// modeByteAt finds the mode byte (first byte after 10 7E 00 00 marker) in response.
	modeByteAt := func(data []byte) (byte, bool) {
		frames := sysex.FindSysExFrames(data, []byte{0xF0, 0x00, 0x32, 0x0D, 0x49})
		for _, f := range frames {
			cr := sysex.ParseConfigResponse(f)
			if cr != nil {
				return cr.Mode, true
			}
		}
		return 0, false
	}

	// Sub-test 1: does the mode byte reflect live mode changes?
	t.Run("mode_liveness", func(t *testing.T) {
		drain()

		t.Log("Setting Custom CC mode (0x07)...")
		r1 := sendRecv(hex.EncodeToString(sysex.BuildModeChange(sysex.ModeCustom)), 2*time.Second)
		t.Logf("  mode-change ACK bytes: %d", len(r1))

		t.Log("Reading with BuildReadSettings...")
		r2 := sendRecv(hex.EncodeToString(sysex.BuildReadSettings()), 3*time.Second)
		mode07, ok07 := modeByteAt(r2)
		t.Logf("  After Custom CC: mode byte=0x%02X found=%v", mode07, ok07)

		drain()
		t.Log("Setting Advanced Custom mode (0x09)...")
		r3 := sendRecv(hex.EncodeToString(sysex.BuildModeChange(sysex.ModeAdvancedCustom)), 2*time.Second)
		t.Logf("  mode-change ACK bytes: %d", len(r3))

		t.Log("Reading with BuildReadSettings...")
		r4 := sendRecv(hex.EncodeToString(sysex.BuildReadSettings()), 3*time.Second)
		mode09, ok09 := modeByteAt(r4)
		t.Logf("  After Adv Custom: mode byte=0x%02X found=%v", mode09, ok09)

		if ok07 && ok09 {
			if mode07 != mode09 {
				t.Logf("FINDING: mode byte IS live (0x%02X → 0x%02X). CC data is elsewhere in response.", mode07, mode09)
			} else {
				t.Logf("FINDING: mode byte is STATIC (both reads return 0x%02X). 0D41 response is fully cached/flash.", mode07)
			}
		} else {
			t.Log("Could not parse mode from one or both responses.")
		}
	})

	// Sub-test 2: send init message 2 (the other 0D41 variant) and capture response.
	// Init msg 2: F0 00 32 0D 41 00 00 00 02 71 07 00 00 10 6A 00 00 33 01 F7
	// This might return live CC data vs the static 1173-byte dump.
	t.Run("init_msg2", func(t *testing.T) {
		drain()

		// Standard read first (baseline).
		t.Log("Standard read (BuildReadSettings):")
		r1 := sendRecv(hex.EncodeToString(sysex.BuildReadSettings()), 3*time.Second)
		t.Logf("  standard: %d bytes", len(r1))

		drain()
		// Init message 2 - the alternate 0D41 read variant.
		initMsg2 := "f000320d410000000271070000106a00003301f7"
		t.Logf("Sending init msg2 (%s):", initMsg2)
		r2 := sendRecv(initMsg2, 3*time.Second)
		t.Logf("  init_msg2 response: %d bytes", len(r2))
		if len(r2) > 0 {
			t.Logf("  hex: %s", hex.EncodeToString(r2)[:min(len(hex.EncodeToString(r2)), 120)])
		}

		// Parse any 0D 49 frames.
		frames := sysex.FindSysExFrames(r2, []byte{0xF0, 0x00, 0x32, 0x0D, 0x49})
		t.Logf("  0D49 frames found: %d", len(frames))
		for i, f := range frames {
			cr := sysex.ParseConfigResponse(f)
			if cr != nil {
				t.Logf("  frame[%d]: mode=0x%02X CC=[%d %d %d %d]", i, cr.Mode, cr.CC[0], cr.CC[1], cr.CC[2], cr.CC[3])
			} else {
				t.Logf("  frame[%d]: %d bytes (unparseable)", i, len(f))
			}
		}
		// Look for other frame types.
		okFrames := sysex.FindSysExFrames(r2, []byte{0xF0, 0x00, 0x32, 0x01, 0x08})
		t.Logf("  01 08 (OK) frames: %d", len(okFrames))
	})

	// Sub-test 3: write CC=48 with init msg2 THEN read, compare bytes changed.
	// If init_msg2 response reflects CC writes, some bytes will differ.
	t.Run("init_msg2_diff", func(t *testing.T) {
		drain()

		initMsg2 := "f000320d410000000271070000106a00003301f7"
		before := sendRecv(initMsg2, 3*time.Second)
		t.Logf("before: %d bytes", len(before))

		// Write CC=99 (distinctive).
		if err := sendOnly(hex.EncodeToString(sysex.BuildModeChange(sysex.ModeCustom))); err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
		for _, msg := range sysex.SplitMessages(sysex.BuildCCConfig(0, 99, false, 0)) {
			ack := sendRecv(hex.EncodeToString(msg), 2*time.Second)
			if len(sysex.FindSysExFrames(ack, []byte{0xF0, 0x00, 0x32, 0x01, 0x08})) > 0 {
				t.Log("  CC write ACK'd")
			}
		}

		drain()
		after := sendRecv(initMsg2, 3*time.Second)
		t.Logf("after: %d bytes", len(after))

		changed := 0
		for i := 0; i < len(before) && i < len(after); i++ {
			if before[i] != after[i] {
				t.Logf("  CHANGED [%d] 0x%02X → 0x%02X", i, before[i], after[i])
				changed++
			}
		}
		if changed == 0 {
			t.Log("FINDING: init_msg2 response also STATIC — does not reflect CC writes")
		} else {
			t.Logf("FINDING: %d byte(s) changed in init_msg2 response after CC write!", changed)
		}

		// Check for 0x63 (CC=99).
		found99 := false
		for i, b := range after {
			if b == 0x63 {
				t.Logf("  0x63 (CC=99) at [%d]", i)
				found99 = true
			}
		}
		if !found99 {
			t.Log("  0x63 (CC=99) not found in init_msg2 response")
		}
	})
}

// TestHeadlessAdvCustomSubcmds probes which subcmds control per-switch CC in Advanced Custom mode.
//
// In Advanced Custom (0x09), each switch is configured independently. Init msgs 9-12 use
// subcmds 0x32/0x36/0x3A/0x3E (stride 4) to set switch type. The CC# for each switch
// might use the adjacent subcmd (0x33/0x37/0x3B/0x3F) or a different range.
//
// This test:
//  1. Sets Advanced Custom mode
//  2. Sends subcmds 0x30-0x3F with value=48 (one at a time) checking for ACK
//  3. Reads back and checks if mode byte or any other byte changed
func TestHeadlessAdvCustomSubcmds(t *testing.T) {
	path := findDeviceOrSkip(t)
	alsaDev := pathToAlsa(path)

	sendRecv := func(hexStr string, timeout time.Duration) []byte {
		secs := int(timeout.Seconds())
		if secs < 1 {
			secs = 1
		}
		cmd := exec.Command("amidi", "-p", alsaDev,
			"-S", hexStr, "-d", "--timeout", strconv.Itoa(secs))
		raw, _ := cmd.CombinedOutput()
		return []byte(raw)
	}

	sendOnly := func(hexStr string) error {
		cmd := exec.Command("amidi", "-p", alsaDev, "-S", hexStr)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("amidi -S: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	drain := func() {
		cmd := exec.Command("amidi", "-p", alsaDev, "-d", "--timeout", "1")
		_, _ = cmd.CombinedOutput()
	}

	stripHex := func(raw []byte) []byte {
		clean := strings.Map(func(r rune) rune {
			if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
				return r
			}
			return -1
		}, string(raw))
		data, _ := hex.DecodeString(clean)
		return data
	}

	hasOK := func(data []byte) bool {
		for _, f := range sysex.FindSysExFrames(data, []byte{0xF0, 0x00, 0x32, 0x01, 0x08}) {
			if sysex.IsOKResponse(f) {
				return true
			}
		}
		return false
	}

	// Build a 09 49 message with custom subcmd and value, byte[10] variant.
	// Standard BuildCCConfig has byte[10]=0x00 and byte[8]=0x02.
	// Per-switch init msgs have byte[10]=0x0E. Try both variants.
	buildMsg := func(subcmd, val, byte10 byte) []byte {
		msg := []byte{
			0xF0, 0x00, 0x32, 0x09, 0x49,
			0x00, 0x00, 0x00, 0x02,
			subcmd, byte10, 0x00, 0x00,
			0x10,
			0x00, 0x00, 0x00,
			val,
			0x00, 0x00,
			0xF7,
		}
		cs1, cs2 := sysex.Checksum(msg[1 : len(msg)-3])
		msg[len(msg)-3] = cs1
		msg[len(msg)-2] = cs2
		return msg
	}

	// Set Advanced Custom mode.
	drain()
	if err := sendOnly(hex.EncodeToString(sysex.BuildModeChange(sysex.ModeAdvancedCustom))); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	// Baseline read.
	baseline := stripHex(sendRecv(hex.EncodeToString(sysex.BuildReadSettings()), 3*time.Second))
	t.Logf("Baseline: %d bytes, mode byte: 0x%02X", len(baseline), func() byte {
		frames := sysex.FindSysExFrames(baseline, []byte{0xF0, 0x00, 0x32, 0x0D, 0x49})
		for _, f := range frames {
			if cr := sysex.ParseConfigResponse(f); cr != nil {
				return cr.Mode
			}
		}
		return 0xFF
	}())

	// Try subcmds 0x30-0x3F with value=48 and byte10=0x00 and byte10=0x0E.
	type result struct {
		subcmd byte
		byte10 byte
		acked  bool
		changed []int
	}
	var results []result

	for subcmd := byte(0x30); subcmd <= byte(0x3F); subcmd++ {
		for _, b10 := range []byte{0x00, 0x0E} {
			drain()
			msg := buildMsg(subcmd, 48, b10)
			raw := sendRecv(hex.EncodeToString(msg), 2*time.Second)
			data := stripHex(raw)
			acked := hasOK(data)

			if acked {
				// Check if readback changed.
				after := stripHex(sendRecv(hex.EncodeToString(sysex.BuildReadSettings()), 3*time.Second))
				var changed []int
				for i := 0; i < len(baseline) && i < len(after); i++ {
					if baseline[i] != after[i] {
						changed = append(changed, i)
					}
				}
				results = append(results, result{subcmd, b10, true, changed})
				t.Logf("ACK: subcmd=0x%02X b10=0x%02X → %d bytes changed in readback", subcmd, b10, len(changed))
				for _, pos := range changed {
					t.Logf("  [%d] 0x%02X → 0x%02X", pos, baseline[pos], after[pos])
				}
				baseline = after // update baseline
			} else {
				results = append(results, result{subcmd, b10, false, nil})
			}
		}
	}

	t.Log("\n=== SUMMARY ===")
	for _, r := range results {
		if r.acked {
			t.Logf("ACK subcmd=0x%02X b10=0x%02X  changed=%d bytes", r.subcmd, r.byte10, len(r.changed))
		}
	}
	ackedCount := 0
	for _, r := range results {
		if r.acked {
			ackedCount++
		}
	}
	if ackedCount == 0 {
		t.Log("NO subcmds in 0x30-0x3F returned ACK in Advanced Custom mode")
	}
}

// TestHeadlessAdvCustomCC48 verifies the full Advanced Custom CC=48 flow:
// 1. Set Advanced Custom mode
// 2. Send per-switch CC config via BuildAdvCustomCC (subcmds 0x30+n*4, byte10=0x0E)
// 3. Verify device ACKs each message
// 4. Read back mode (should stay 0x09)
func TestHeadlessAdvCustomCC48(t *testing.T) {
	path := findDeviceOrSkip(t)
	alsaDev := pathToAlsa(path)

	sendRecv := func(hexStr string, timeout time.Duration) []byte {
		secs := int(timeout.Seconds())
		if secs < 1 {
			secs = 1
		}
		cmd := exec.Command("amidi", "-p", alsaDev,
			"-S", hexStr, "-d", "--timeout", strconv.Itoa(secs))
		raw, _ := cmd.CombinedOutput()
		return []byte(raw)
	}

	drain := func() {
		cmd := exec.Command("amidi", "-p", alsaDev, "-d", "--timeout", "1")
		_, _ = cmd.CombinedOutput()
	}

	stripHex := func(raw []byte) []byte {
		clean := strings.Map(func(r rune) rune {
			if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
				return r
			}
			return -1
		}, string(raw))
		data, _ := hex.DecodeString(clean)
		return data
	}

	drain()

	// Step 1: set Advanced Custom mode, verify ACK.
	t.Log("Setting Advanced Custom mode...")
	modeRaw := sendRecv(hex.EncodeToString(sysex.BuildModeChange(sysex.ModeAdvancedCustom)), 2*time.Second)
	modeData := stripHex(modeRaw)
	modeACKd := len(sysex.FindSysExFrames(modeData, []byte{0xF0, 0x00, 0x32, 0x01, 0x08})) > 0
	t.Logf("  mode change ACK: %v (%d bytes)", modeACKd, len(modeData))

	// Step 2: send BuildAdvCustomCC for switches 0-3 (first bank), CC=48.
	targetCC := byte(48)
	ackCount := 0
	failCount := 0
	for sw := 0; sw < 4; sw++ {
		msgs := sysex.SplitMessages(sysex.BuildAdvCustomCC(sw, targetCC, false, 0))
		t.Logf("  switch %d: sending %d messages (CC=%d)", sw, len(msgs), targetCC)
		for j, msg := range msgs {
			raw := sendRecv(hex.EncodeToString(msg), 2*time.Second)
			data := stripHex(raw)
			acked := len(sysex.FindSysExFrames(data, []byte{0xF0, 0x00, 0x32, 0x01, 0x08})) > 0
			t.Logf("    msg[%d] subcmd=0x%02X byte10=0x%02X val=0x%02X ACK=%v",
				j, msg[9], msg[10], msg[17], acked)
			if acked {
				ackCount++
			} else {
				failCount++
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Step 3: verify mode is still Advanced Custom.
	drain()
	readRaw := sendRecv(hex.EncodeToString(sysex.BuildReadSettings()), 3*time.Second)
	readData := stripHex(readRaw)
	frames := sysex.FindSysExFrames(readData, []byte{0xF0, 0x00, 0x32, 0x0D, 0x49})
	modeOK := false
	for _, f := range frames {
		if cr := sysex.ParseConfigResponse(f); cr != nil {
			t.Logf("  readback mode=0x%02X (%s)", cr.Mode, sysex.ModeNames[cr.Mode])
			if cr.Mode == sysex.ModeAdvancedCustom {
				modeOK = true
			}
		}
	}

	t.Logf("Result: %d ACKs, %d no-ACK, mode_correct=%v", ackCount, failCount, modeOK)
	if failCount > 0 {
		t.Errorf("FAIL: %d BuildAdvCustomCC messages not ACK'd by device", failCount)
	}
	if !modeACKd {
		t.Error("FAIL: mode change to Advanced Custom not ACK'd")
	}
	if !modeOK {
		t.Error("FAIL: mode after write is not Advanced Custom")
	}
	if failCount == 0 && modeOK {
		t.Log("PASS: all Advanced Custom CC=48 messages ACK'd and mode persists")
		t.Log("NOTE: to verify CC=48 actually sends, press a switch and observe MIDI output")
	}
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

// TestHeadlessAdvCustomFlashWriteProbe probes whether AdvCustom writes with byte[10]=0x00
// (BuildAdvCustomCCFlash) update the 0D41 config dump, establishing a flash persistence path.
//
// Two writes are compared:
//   - RAM write (byte[10]=0x0E, CC=77): immediate effect, unknown persistence
//   - Flash write (byte[10]=0x00, CC=99): unknown effect, potential persistence
//
// If "Flash write" changes the 0D41 dump, dual-path writes (0x0E + 0x00) would
// give both immediate effect AND persistence after USB reconnect.
//
// Run: go test -v -run TestHeadlessAdvCustomFlashWriteProbe -timeout 60s ./internal/midi/
func TestHeadlessAdvCustomFlashWriteProbe(t *testing.T) {
	path := findDeviceOrSkip(t)
	alsaDev := pathToAlsa(path)

	stripHex := func(raw []byte) []byte {
		clean := strings.Map(func(r rune) rune {
			if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
				return r
			}
			return -1
		}, string(raw))
		data, _ := hex.DecodeString(clean)
		return data
	}

	sendRecv := func(hexStr string, timeout time.Duration) []byte {
		secs := int(timeout.Seconds())
		if secs < 1 {
			secs = 1
		}
		cmd := exec.Command("amidi", "-p", alsaDev,
			"-S", hexStr, "-d", "--timeout", strconv.Itoa(secs))
		raw, _ := cmd.CombinedOutput()
		return stripHex(raw)
	}

	drain := func() {
		cmd := exec.Command("amidi", "-p", alsaDev, "-d", "--timeout", "1")
		_, _ = cmd.CombinedOutput()
	}

	hasOK := func(data []byte) bool {
		return len(sysex.FindSysExFrames(data, []byte{0xF0, 0x00, 0x32, 0x01, 0x08})) > 0
	}

	readDump := func() []byte {
		drain()
		return sendRecv(hex.EncodeToString(sysex.BuildReadSettings()), 3*time.Second)
	}

	diffBytes := func(before, after []byte, label string) int {
		changes := 0
		for i := 0; i < len(before) && i < len(after); i++ {
			if before[i] != after[i] {
				changes++
				t.Logf("  %s: byte[%d] 0x%02X → 0x%02X", label, i, before[i], after[i])
			}
		}
		return changes
	}

	// Ensure we're in Advanced Custom mode.
	drain()
	modeRaw := sendRecv(hex.EncodeToString(sysex.BuildModeChange(sysex.ModeAdvancedCustom)), 2*time.Second)
	if !hasOK(modeRaw) {
		t.Fatal("mode change to AdvancedCustom not ACKd")
	}
	t.Log("Mode: AdvancedCustom set")

	// Baseline: 0D41 dump before any AdvCustom CC writes.
	baseline := readDump()
	t.Logf("Baseline dump: %d bytes", len(baseline))

	// --- RAM write: byte[10]=0x0E, CC=77 ---
	t.Log("\n=== RAM write (byte[10]=0x0E, CC=77) ===")
	ramMsgs := sysex.SplitMessages(sysex.BuildAdvCustomCC(0, 77, false, 0))
	ramACKs := 0
	for _, msg := range ramMsgs {
		raw := sendRecv(hex.EncodeToString(msg), 2*time.Second)
		if hasOK(raw) {
			ramACKs++
			t.Logf("  subcmd=0x%02X byte10=0x%02X val=%d ACK=true", msg[9], msg[10], msg[17])
		} else {
			t.Logf("  subcmd=0x%02X byte10=0x%02X val=%d ACK=false", msg[9], msg[10], msg[17])
		}
	}
	afterRAM := readDump()
	ramChanges := diffBytes(baseline, afterRAM, "RAM")
	t.Logf("RAM write (0x0E): %d/%d msgs ACKd, %d bytes changed in 0D41 dump",
		ramACKs, len(ramMsgs), ramChanges)

	// --- Flash write: byte[10]=0x00, CC=99 ---
	t.Log("\n=== Flash write (byte[10]=0x00, CC=99) ===")
	flashMsgs := sysex.SplitMessages(sysex.BuildAdvCustomCCFlash(0, 99, false, 0))
	flashACKs := 0
	for _, msg := range flashMsgs {
		raw := sendRecv(hex.EncodeToString(msg), 2*time.Second)
		if hasOK(raw) {
			flashACKs++
			t.Logf("  subcmd=0x%02X byte10=0x%02X val=%d ACK=true", msg[9], msg[10], msg[17])
		} else {
			t.Logf("  subcmd=0x%02X byte10=0x%02X val=%d ACK=false", msg[9], msg[10], msg[17])
		}
	}
	afterFlash := readDump()
	flashChanges := diffBytes(afterRAM, afterFlash, "Flash")
	t.Logf("Flash write (0x00): %d/%d msgs ACKd, %d bytes changed in 0D41 dump",
		flashACKs, len(flashMsgs), flashChanges)

	// Scan afterFlash dump for AdvCustom CC subcmd 0x30 with value 99.
	found99 := false
	for i := 0; i < len(afterFlash)-1; i++ {
		if afterFlash[i] == sysex.AdvCustomSubcmdBase && afterFlash[i+1] == 99 {
			t.Logf("  FOUND: subcmd=0x30 value=99 at byte[%d] in 0D41 dump after flash write", i)
			found99 = true
		}
	}

	t.Log("\n=== RESULT ===")
	if flashACKs > 0 && flashChanges > 0 {
		t.Log("DUAL-PATH CONFIRMED: byte[10]=0x00 IS ACKd and DOES update 0D41 dump.")
		t.Log("  → Update sendAllConfig to send both BuildAdvCustomCC (RAM) + BuildAdvCustomCCFlash (flash)")
		t.Log("  → Update ParseConfigResponse to parse AdvCustom subcmds (0x30+) from dump")
	} else if flashACKs > 0 && flashChanges == 0 {
		t.Log("FLASH ACKd but NO dump change: byte[10]=0x00 accepted but doesn't update 0D41.")
		t.Log("  → AdvCustom may need a different save-to-flash mechanism")
	} else {
		t.Log("FLASH NOT ACKd: byte[10]=0x00 rejected for AdvCustom subcmds.")
		t.Log("  → Only byte[10]=0x0E works; config is RAM-only (lost on power cycle)")
		t.Log("  → Use local config file for TUI state persistence")
	}
	if found99 {
		t.Log("  + CC=99 subcmd found in dump → ParseConfigResponse can read AdvCustom CC values")
	}
}

// TestHeadlessAdvCustomPersistenceRoundtrip tests the full user journey:
// configure CC=48 in Advanced Custom mode → close fd → reopen → verify mode persists.
//
// Run: go test -v -run TestHeadlessAdvCustomPersistenceRoundtrip -timeout 90s ./internal/midi/
func TestHeadlessAdvCustomPersistenceRoundtrip(t *testing.T) {
	path := findDeviceOrSkip(t)
	alsaDev := pathToAlsa(path)

	stripHex := func(raw []byte) []byte {
		clean := strings.Map(func(r rune) rune {
			if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
				return r
			}
			return -1
		}, string(raw))
		data, _ := hex.DecodeString(clean)
		return data
	}

	sendRecv := func(hexStr string, timeout time.Duration) []byte {
		secs := int(timeout.Seconds())
		if secs < 1 {
			secs = 1
		}
		cmd := exec.Command("amidi", "-p", alsaDev,
			"-S", hexStr, "-d", "--timeout", strconv.Itoa(secs))
		raw, _ := cmd.CombinedOutput()
		return stripHex(raw)
	}

	drain := func() {
		cmd := exec.Command("amidi", "-p", alsaDev, "-d", "--timeout", "1")
		_, _ = cmd.CombinedOutput()
	}

	hasOK := func(data []byte) bool {
		return len(sysex.FindSysExFrames(data, []byte{0xF0, 0x00, 0x32, 0x01, 0x08})) > 0
	}

	readMode := func(label string) (mode byte, isAdvCustom bool, rawDump []byte) {
		drain()
		raw := sendRecv(hex.EncodeToString(sysex.BuildReadSettings()), 3*time.Second)
		frames := sysex.FindSysExFrames(raw, []byte{0xF0, 0x00, 0x32, 0x0D, 0x49})
		for _, f := range frames {
			if cr := sysex.ParseConfigResponse(f); cr != nil {
				name := sysex.ModeNames[cr.Mode]
				t.Logf("%s: mode=0x%02X (%s)", label, cr.Mode, name)
				return cr.Mode, cr.Mode == sysex.ModeAdvancedCustom, raw
			}
		}
		t.Logf("%s: no config frame in response (%d bytes)", label, len(raw))
		return 0, false, raw
	}

	// === SESSION 1: set AdvCustom + CC=48 for switch 0 ===
	t.Log("=== Session 1: configure CC=48 in Advanced Custom mode ===")
	drain()

	modeRaw := sendRecv(hex.EncodeToString(sysex.BuildModeChange(sysex.ModeAdvancedCustom)), 2*time.Second)
	if !hasOK(modeRaw) {
		t.Fatal("mode change not ACKd")
	}

	ackCount := 0
	for _, msg := range sysex.SplitMessages(sysex.BuildAdvCustomCC(0, 48, false, 0)) {
		raw := sendRecv(hex.EncodeToString(msg), 2*time.Second)
		if hasOK(raw) {
			ackCount++
		}
	}
	t.Logf("CC=48 for switch 0: %d/3 ACKd", ackCount)
	if ackCount != 3 {
		t.Fatalf("expected 3 ACKs, got %d — device not responding correctly", ackCount)
	}

	_, ok1, _ := readMode("Session 1 readback")
	if !ok1 {
		t.Error("Session 1: mode not AdvancedCustom after write")
	}

	// === "CLOSE" — simulate TUI close (amidi subprocesses already closed) ===
	t.Log("\n=== Simulating TUI close (500ms pause) ===")
	time.Sleep(500 * time.Millisecond)

	// === SESSION 2: fresh read, no re-send ===
	t.Log("=== Session 2: read without re-sending config ===")
	mode2, ok2, rawDump2 := readMode("Session 2 readback")

	// Scan raw 0D41 dump for AdvCustom subcmd 0x30 + value 48.
	found48 := false
	for i := 0; i < len(rawDump2)-1; i++ {
		if rawDump2[i] == sysex.AdvCustomSubcmdBase && rawDump2[i+1] == 48 {
			t.Logf("  FOUND: subcmd=0x30 value=48 at raw byte[%d] — CC=48 in dump!", i)
			found48 = true
		}
	}

	t.Log("\n=== RESULT ===")
	if ok2 {
		t.Log("PASS: mode=AdvancedCustom persists after simulated close")
	} else {
		t.Logf("INFO: mode after close=0x%02X (not AdvancedCustom) — mode may be RAM-only too", mode2)
	}

	if found48 {
		t.Log("PASS: CC=48 found in 0D41 dump → device persists AdvCustom CC in dump/flash")
	} else {
		t.Log("INFO: CC=48 not found in 0D41 dump")
		t.Log("  → Run TestHeadlessAdvCustomFlashWriteProbe to probe byte[10]=0x00 path")
		t.Log("  → Physical test: press switch 0, if CC=48 arrives: device uses RAM that persists over close")
		t.Log("  → If CC=48 NOT sent after USB reconnect: re-send config on every TUI connect")
	}
}
