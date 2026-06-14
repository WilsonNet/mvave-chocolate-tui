package main

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestFindChecksumAlgo(t *testing.T) {
	t.Skip("Exploration test - run manually to find algorithm")

	// Candidate algorithms against known examples
	algorithms := []struct {
		name string
		fn   func([]byte) (byte, byte)
	}{
		{"0x400 - sum", func(data []byte) (byte, byte) {
			var sum uint16
			for _, b := range data {
				sum += uint16(b)
			}
			v := 0x400 - sum
			return byte(v & 0x7F), byte((v >> 7) & 0x7F)
		}},
		{"0x400 - sum of non-zero", func(data []byte) (byte, byte) {
			var sum uint16
			for _, b := range data {
				if b != 0 {
					sum += uint16(b)
				}
			}
			v := 0x400 - sum
			return byte(v & 0x7F), byte((v >> 7) & 0x7F)
		}},
		{"0x4000 - sum (14bit)", func(data []byte) (byte, byte) {
			var sum uint16
			for _, b := range data {
				sum += uint16(b)
			}
			v := 0x4000 - sum
			return byte(v & 0x7F), byte((v >> 7) & 0x7F)
		}},
		{"sum bytes after header", func(data []byte) (byte, byte) {
			var sum uint16
			for _, b := range data[5:] {
				sum += uint16(b)
			}
			v := 0x400 - sum
			return byte(v & 0x7F), byte((v >> 7) & 0x7F)
		}},
		{"two sum method", func(data []byte) (byte, byte) {
			var low, high uint16
			for i, b := range data {
				if i%2 == 0 {
					low += uint16(b)
				} else {
					high += uint16(b)
				}
			}
			l := 0x80 - byte(low&0x7F)
			h := 0x80 - byte(high&0x7F)
			return l & 0x7F, h & 0x7F
		}},
	}

	for _, ex := range knownExamples {
		for _, a := range algorithms {
			csLow, csHigh := a.fn(ex.data)
			if csLow == ex.csLow && csHigh == ex.csHigh {
				fmt.Printf("MATCH: %s for %s\n", a.name, ex.name)
			}
		}
	}
}

func TestChecksumAnalyzer(t *testing.T) {
	t.Skip("Analysis test - prints checksum patterns")

	for _, ex := range knownExamples {
		fmt.Printf("\n=== %s ===\n", ex.name)
		fmt.Printf("data: %s (%d bytes)\n", hex.EncodeToString(ex.data), len(ex.data))

		var sum uint16
		for _, b := range ex.data {
			sum += uint16(b)
		}

		expected := uint16(ex.csHigh)<<7 | uint16(ex.csLow)
		fmt.Printf("sum=0x%04X (%d), expected=0x%04X (%d)\n", sum, sum, expected, expected)

		if len(ex.data) >= 5 {
			fmt.Printf("bytes after cmd: %X\n", ex.data[5:])
			var s2 uint16
			for _, b := range ex.data[5:] {
				s2 += uint16(b)
			}
			fmt.Printf("sum after cmd: 0x%04X (%d)\n", s2, s2)
		}

		fmt.Printf("0x%X - sum = 0x%X\n", 0x400, uint16(0x400)-sum)
		fmt.Printf("0x%X - sum = 0x%X\n", 0x4000, uint16(0x4000)-sum)
		fmt.Printf("sum + expected = 0x%X\n", sum+expected)
	}
}
