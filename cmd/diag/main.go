package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	path := "/dev/snd/midiC5D0"
	fmt.Printf("Opening %s...\n", path)
	start := time.Now()
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		fmt.Printf("ERR open: %v (took %v)\n", err, time.Since(start))
		return
	}
	defer f.Close()
	fmt.Printf("Opened in %v\n", time.Since(start))

	// Send a simple CC message (not Sysex - just 3 bytes)
	msg := []byte{0xB0, 0x14, 0x7F}
	fmt.Printf("Writing 3 CC bytes...\n")
	start = time.Now()
	n, err := f.Write(msg)
	if err != nil {
		fmt.Printf("ERR write: %v (took %v)\n", err, time.Since(start))
	} else {
		fmt.Printf("Wrote %d bytes in %v\n", n, time.Since(start))
	}

	// Send a SysEx message
	msg2 := []byte{
		0xF0, 0x00, 0x32, 0x09, 0x49,
		0x00, 0x00, 0x00, 0x02,
		0x00, 0x00, 0x00, 0x00,
		0x10,
		0x00, 0x00, 0x00, 0x07,
		0x66, 0x03,
		0xF7,
	}
	fmt.Printf("Writing 21 SysEx bytes...\n")
	start = time.Now()
	n, err = f.Write(msg2)
	if err != nil {
		fmt.Printf("ERR sysex write: %v (took %v)\n", err, time.Since(start))
	} else {
		fmt.Printf("Wrote %d SyEx bytes in %v\n", n, time.Since(start))
	}

	// Try a read
	fmt.Println("Reading (1s timeout)...")
	buf := make([]byte, 64)
	done := make(chan struct{})
	go func() {
		n, err := f.Read(buf)
		fmt.Printf("Read returned: n=%d err=%v\n", n, err)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		fmt.Println("Read timed out (expected)")
	}
}
