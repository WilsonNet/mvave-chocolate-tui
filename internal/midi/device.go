// Copyright (C) 2026 Wilson Neto
// SPDX-License-Identifier: GPL-3.0-or-later

// Package midi provides raw ALSA MIDI I/O for communicating with USB MIDI devices.
// Reads use a raw file descriptor (O_RDONLY). Writes use the amidi subprocess
// to avoid blocking on ALSA raw MIDI writes.
package midi

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Device struct {
	alsaDev  string // e.g., "hw:5,0,0"
	f        *os.File
	useAmidi bool
}

func Open(path string) (*Device, error) {
	alsaDev := ""

	base := path
	if strings.HasPrefix(base, "/dev/snd/midiC") {
		rest := strings.TrimPrefix(base, "/dev/snd/midiC")
		parts := strings.SplitN(rest, "D", 2)
		cardID := parts[0]
		devNum := "0"
		if len(parts) > 1 {
			devNum = parts[1]
		}
		alsaDev = fmt.Sprintf("hw:%s,%s,0", cardID, devNum)
	} else if strings.HasPrefix(base, "/dev/midi") {
		cardID := strings.TrimPrefix(base, "/dev/midi")
		alsaDev = fmt.Sprintf("hw:%s,0,0", cardID)
	} else {
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", path, err)
		}
		return &Device{f: f, useAmidi: false}, nil
	}

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	return &Device{alsaDev: alsaDev, f: f, useAmidi: true}, nil
}

func (d *Device) Read(buf []byte) (int, error) {
	return d.f.Read(buf)
}

func (d *Device) SendSysex(data []byte) error {
	if d.useAmidi {
		hexStr := hex.EncodeToString(data)
		cmd := exec.Command("amidi", "-p", d.alsaDev, "-S", hexStr)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("amidi: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	_, err := d.f.Write(data)
	return err
}

// SendReceiveSysex sends a SysEx message and returns raw bytes received within
// timeoutSecs seconds of inactivity. Uses amidi --timeout (seconds, not ms).
// Both send and receive happen in one port-open window so ACKs aren't lost.
func (d *Device) SendReceiveSysex(data []byte, timeoutSecs int) ([]byte, error) {
	if !d.useAmidi {
		return nil, fmt.Errorf("SendReceiveSysex: not an ALSA device")
	}
	if timeoutSecs < 1 {
		timeoutSecs = 1
	}
	hexStr := hex.EncodeToString(data)
	cmd := exec.Command("amidi", "-p", d.alsaDev,
		"-S", hexStr, "-d", "--timeout", fmt.Sprintf("%d", timeoutSecs))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("amidi: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func (d *Device) Close() error {
	if d.f != nil {
		return d.f.Close()
	}
	return nil
}
