// Copyright (C) 2026 Wilson Neto
// SPDX-License-Identifier: GPL-3.0-or-later

// Package detect auto-discovers the M-Vave Chocolate MIDI device.
package detect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var DeviceNames = []string{"SINCO", "FootCtrl", "USB-Midi"}

func Find() (string, error) {
	data, err := os.ReadFile("/proc/asound/cards")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			for _, name := range DeviceNames {
				if strings.Contains(line, name) {
					cardNum := strings.TrimSpace(strings.Split(line, "[")[0])
					candidates, _ := filepath.Glob(fmt.Sprintf("/dev/snd/midiC%s*", cardNum))
					if len(candidates) > 0 {
						return candidates[0], nil
					}
					midiDev := fmt.Sprintf("/dev/midi%s", cardNum)
					if _, err := os.Stat(midiDev); err == nil {
						return midiDev, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("no M-Vave Chocolate found (SINCO/FootCtrl not in /proc/asound/cards)")
}
