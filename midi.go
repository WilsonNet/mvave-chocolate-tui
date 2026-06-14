// Copyright (C) 2026 Wilson Neto
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type MidiDevice struct {
	alsaDev  string // e.g., "hw:5,0,0"
	f        *os.File
	useAmidi bool
}

func OpenMidiDevice(path string) (*MidiDevice, error) {
	// Convert device path to ALSA hw:X,0,0 format
	alsaDev := ""
	cardID := ""

	base := path
	if strings.HasPrefix(base, "/dev/snd/midiC") {
		rest := strings.TrimPrefix(base, "/dev/snd/midiC")
		parts := strings.SplitN(rest, "D", 2)
		cardID = parts[0]
		devNum := "0"
		if len(parts) > 1 {
			devNum = parts[1]
		}
		alsaDev = fmt.Sprintf("hw:%s,%s,0", cardID, devNum)
	} else if strings.HasPrefix(base, "/dev/midi") {
		cardID = strings.TrimPrefix(base, "/dev/midi")
		alsaDev = fmt.Sprintf("hw:%s,0,0", cardID)
	} else {
		// Non-standard path (e.g., test temp file) - use raw file I/O only
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", path, err)
		}
		return &MidiDevice{f: f, useAmidi: false}, nil
	}

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	return &MidiDevice{alsaDev: alsaDev, f: f, useAmidi: true}, nil
}

func (d *MidiDevice) Read(buf []byte) (int, error) {
	return d.f.Read(buf)
}

func (d *MidiDevice) SendSysex(data []byte) error {
	if d.useAmidi {
		hexStr := hex.EncodeToString(data)
		cmd := exec.Command("amidi", "-p", d.alsaDev, "-S", hexStr)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("amidi: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	// Fallback: raw file write (for tests)
	_, err := d.f.Write(data)
	return err
}

func (d *MidiDevice) Close() error {
	if d.f != nil {
		return d.f.Close()
	}
	return nil
}
