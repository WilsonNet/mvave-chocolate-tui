// Copyright (C) 2026 Wilson Neto and contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

// Backups and comparing reads.
//
// A backup is every byte a read returned, kept as JSON beside the config.
// Restoring one does NOT write raw bytes back: it decodes the backup with the
// same Device and loads it as edits, so the screen shows every value that
// would change, and writing goes through the same checks, confirmation and
// read-back as any edit. A restore can do exactly what an edit can.
//
// Comparing marks the current read; the next read is listed byte by byte
// against it, with what each byte means where the model knows. That is how an
// unknown setting gets mapped: mark, change it in the official app, read.

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"mvave-chocolate-tui/internal/sysex"
)

type backupFile struct {
	Product string    `json:"product"`
	Taken   time.Time `json:"taken"`
	// Image is the bytes from address 0 up, hex-encoded; Missing lists any
	// address inside that range the read did not cover.
	Image   string `json:"image"`
	Missing []int  `json:"missing,omitempty"`
}

func backupDir() string { return filepath.Join(filepath.Dir(configFilePath()), "backups") }

func imageBytes(img sysex.Image) (data []byte, missing []int) {
	top := -1
	for a := range img {
		if a > top {
			top = a
		}
	}
	data = make([]byte, top+1)
	for a := 0; a <= top; a++ {
		v, ok := img[a]
		if !ok {
			missing = append(missing, a)
		}
		data[a] = v
	}
	return data, missing
}

func (m *Model) exportBackup() {
	if m.cfg == nil {
		m.statusMsg = "Nothing read yet"
		return
	}
	data, missing := imageBytes(m.pedalImage)
	b := backupFile{Product: m.device.Product(), Taken: time.Now(), Image: hex.EncodeToString(data), Missing: missing}
	out, err := json.MarshalIndent(b, "", "  ")
	if err == nil {
		err = os.MkdirAll(backupDir(), 0o755)
	}
	name := filepath.Join(backupDir(), b.Taken.Format("2006-01-02_150405")+".json")
	if err == nil {
		err = os.WriteFile(name, out, 0o644)
	}
	if err != nil {
		m.statusMsg = "Backup failed: " + err.Error()
		return
	}
	m.statusMsg = "Backed up to " + name
}

// backups lists saved backups for this model, newest first.
func backups(product string) []string {
	files, _ := filepath.Glob(filepath.Join(backupDir(), "*.json"))
	var out []string
	for _, f := range files {
		if b, err := readBackup(f); err == nil && b.Product == product {
			out = append(out, f)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out
}

func readBackup(path string) (backupFile, error) {
	var b backupFile
	data, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(data, &b)
	}
	return b, err
}

func (b backupFile) image() (sysex.Image, error) {
	data, err := hex.DecodeString(b.Image)
	if err != nil {
		return nil, err
	}
	img := sysex.Image{}
	for a, v := range data {
		img[a] = v
	}
	for _, a := range b.Missing {
		delete(img, a)
	}
	return img, nil
}

// loadBackup turns a backup into edits.
func (m *Model) loadBackup(path string) {
	b, err := readBackup(path)
	var img sysex.Image
	if err == nil {
		img, err = b.image()
	}
	if err != nil {
		m.statusMsg = "Cannot read that backup: " + err.Error()
		return
	}
	if !m.device.Recognise(img) {
		m.statusMsg = "That backup is not a " + m.device.Product() + " read this tool can decode"
		return
	}
	cfg := m.device.Decode(img)
	if _, err := m.device.Changes(m.pedalImage, cfg); err != nil {
		m.statusMsg = "Cannot restore that backup: " + err.Error()
		return
	}
	m.pui.draft = &cfg
	m.statusMsg = "Backup from " + b.Taken.Format("02-01-2006 15:04") + " loaded as edits — w writes it"
}

// --- comparing reads --------------------------------------------------------

// describer is implemented by models that can name what a byte is.
type describer interface {
	Describe(addr int) string
}

func (m *Model) markForCompare() {
	if m.cfg == nil {
		m.statusMsg = "Nothing read yet"
		return
	}
	m.pui.mark = sysex.Image{}
	for a, v := range m.pedalImage {
		m.pui.mark[a] = v
	}
	m.pui.markAt = time.Now()
	m.statusMsg = "Marked. Change something on the pedal or in the official app, then r to read and compare"
}

func (m *Model) drawCompare() string {
	lines := []string{pMuted.Render("changed since " + m.pui.markAt.Format("15:04:05"))}
	var addrs []int
	for a, v := range m.pedalImage {
		if old, ok := m.pui.mark[a]; ok && old != v {
			addrs = append(addrs, a)
		}
	}
	sort.Ints(addrs)
	if len(addrs) == 0 {
		lines = append(lines, pFaintTxt.Render("  nothing yet — make a change, then r"))
	}
	d, _ := m.device.(describer)
	for i, a := range addrs {
		if i == 12 {
			lines = append(lines, pFaintTxt.Render(fmt.Sprintf("  … and %d more", len(addrs)-12)))
			break
		}
		what := ""
		if d != nil {
			what = d.Describe(a)
		}
		lines = append(lines, fmt.Sprintf("  %4d  %3d → %s  %s", a, m.pui.mark[a],
			pChanged.Render(fmt.Sprintf("%3d", m.pedalImage[a])), pMuted.Render(what)))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) drawBackups() string {
	lines := []string{pMuted.Render("restore a backup")}
	if len(m.pui.files) == 0 {
		lines = append(lines, pFaintTxt.Render("  none yet — x makes one"))
	}
	for i, f := range m.pui.files {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		if t, err := time.Parse("2006-01-02_150405", name); err == nil {
			name = t.Format("02-01-2006 15:04:05")
		}
		if i == m.pui.pick {
			lines = append(lines, "  "+pCursor.Render(name))
		} else {
			lines = append(lines, "  "+name)
		}
	}
	return strings.Join(lines, "\n")
}
