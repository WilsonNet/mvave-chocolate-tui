package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var deviceNames = []string{"SINCO", "FootCtrl", "USB-Midi"}

func findMidiDevice() (string, error) {
	data, err := os.ReadFile("/proc/asound/cards")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			for _, name := range deviceNames {
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
