// Copyright (C) 2026 Wilson Neto
// SPDX-License-Identifier: GPL-3.0-or-later

package midi

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"mvave-chocolate-tui/internal/detect"
	"mvave-chocolate-tui/internal/sysex"
)

func TestHeadlessOpen(t *testing.T) {
	path := findDeviceOrSkip(t)
	start := time.Now()
	dev, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed after %v: %v", time.Since(start), err)
	}
	defer dev.Close()
	t.Logf("OK: opened in %v", time.Since(start))
}

func TestHeadlessAmidiSend(t *testing.T) {
	path := findDeviceOrSkip(t)
	alsaDev := pathToAlsa(path)

	msg := sysex.BuildModeChange(sysex.ModeCustom)
	ch := make(chan error, 1)
	go func() {
		cmd := exec.Command("amidi", "-p", alsaDev, "-S", hex.EncodeToString(msg))
		out, err := cmd.CombinedOutput()
		if err != nil {
			ch <- fmt.Errorf("amidi: %w: %s", err, strings.TrimSpace(string(out)))
		} else {
			ch <- nil
		}
	}()
	select {
	case err := <-ch:
		if err != nil {
			t.Fatal(err)
		}
		t.Log("OK: amidi send")
	case <-time.After(5 * time.Second):
		t.Fatal("TIMEOUT after 5s")
	}
}

func TestHeadlessFullFlow(t *testing.T) {
	path := findDeviceOrSkip(t)
	dev, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()

	steps := []struct {
		name string
		fn   func() error
	}{
		{"mode change", func() error { return dev.SendSysex(sysex.BuildModeChange(sysex.ModeCustom)) }},
		{"CC config", func() error { return dev.SendSysex(sysex.BuildCCConfig(0, 49, false, 0)) }},
		{"read settings", func() error { return dev.SendSysex(sysex.BuildReadSettings()) }},
	}

	for _, s := range steps {
		ch := make(chan error, 1)
		go func() { ch <- s.fn() }()
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("%s: %v", s.name, err)
			}
			t.Logf("OK: %s", s.name)
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: TIMEOUT", s.name)
		}
	}
}

func TestHeadlessAmidiThreeOps(t *testing.T) {
	path := findDeviceOrSkip(t)
	alsaDev := pathToAlsa(path)

	ops := []struct {
		name string
		hex  string
	}{
		{"mode", hex.EncodeToString(sysex.BuildModeChange(sysex.ModeCustom))},
		{"CC", hex.EncodeToString(sysex.BuildCCConfig(0, 49, false, 0))},
		{"read", hex.EncodeToString(sysex.BuildReadSettings())},
	}

	for _, op := range ops {
		ch := make(chan error, 1)
		go func() {
			cmd := exec.Command("amidi", "-p", alsaDev, "-S", op.hex)
			out, err := cmd.CombinedOutput()
			if err != nil {
				ch <- fmt.Errorf("%s: %w: %s", op.name, err, strings.TrimSpace(string(out)))
			} else {
				ch <- nil
			}
		}()
		select {
		case err := <-ch:
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("OK: %s", op.name)
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: TIMEOUT", op.name)
		}
	}
}

func TestAmidiWithReadFdHeld(t *testing.T) {
	path := findDeviceOrSkip(t)
	alsaDev := pathToAlsa(path)

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	t.Log("raw fd opened for read")

	go func() {
		buf := make([]byte, 64)
		_, _ = f.Read(buf)
	}()

	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	ch := make(chan error, 1)
	go func() {
		cmd := exec.Command("amidi", "-p", alsaDev, "-S",
			hex.EncodeToString(sysex.BuildModeChange(sysex.ModeCustom)))
		out, err := cmd.CombinedOutput()
		if err != nil {
			ch <- fmt.Errorf("amidi: %w: %s", err, strings.TrimSpace(string(out)))
		} else {
			ch <- nil
		}
	}()

	select {
	case err := <-ch:
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("OK: amidi completed in %v with raw read fd held", time.Since(start))
	case <-time.After(10 * time.Second):
		t.Fatal("TIMEOUT: amidi blocks when raw read fd is held!")
	}
}

func TestAmidiWithoutReadFd(t *testing.T) {
	path := findDeviceOrSkip(t)
	alsaDev := pathToAlsa(path)
	_ = path

	start := time.Now()
	cmd := exec.Command("amidi", "-p", alsaDev, "-S",
		hex.EncodeToString(sysex.BuildModeChange(sysex.ModeCustom)))
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("amidi failed after %v: %v: %s", elapsed, err, string(out))
	}
	t.Logf("OK: amidi without read fd completed in %v", elapsed)
}

func findDeviceOrSkip(t *testing.T) string {
	t.Helper()
	path, err := detect.Find()
	if err != nil || path == "" {
		t.Skipf("no SINCO device found: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("device %s not accessible: %v", path, err)
	}
	return path
}

func pathToAlsa(path string) string {
	if strings.HasPrefix(path, "/dev/snd/midiC") {
		rest := strings.TrimPrefix(path, "/dev/snd/midiC")
		parts := strings.SplitN(rest, "D", 2)
		dev := "0"
		if len(parts) > 1 {
			dev = parts[1]
		}
		return fmt.Sprintf("hw:%s,%s,0", parts[0], dev)
	}
	if strings.HasPrefix(path, "/dev/midi") {
		return fmt.Sprintf("hw:%s,0,0", strings.TrimPrefix(path, "/dev/midi"))
	}
	return "hw:5,0,0"
}
