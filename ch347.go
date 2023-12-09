// Package ch347 provides methods to access UART and SPI+I2C+GPIO interfaces
// of the High-speed USB converter chip CH347 in HIDAPI mode (Mode 2).
//
// The ch347 package was built by examining USB packets of the official demonstration library.
//
// [github.com/sstallion/go-hid] can be used as HIDAPI interface.
package ch347

import (
	"io"
	"sync"
)

// IO implements methods to access CH347 SPI+I2C+GPIO.
//
// Pass second hidraw device to Dev.
type IO struct {
	mu  sync.Mutex
	Dev HIDDev
}

// UART implements ReadWriter interface to access CH347 UART.
//
// Pass first hidraw device.
type UART struct {
	Dev HIDDev
}

// # Note:
//
// It's advised to handle read timeouts and "Interrupted system call" errors.
// Otherwise, operations might error "invalid response" once an interrupt has occurred
// or block indefinitely.
//
// Example with the Read method override for [github.com/sstallion/go-hid]:
//
//	type HIDWithTimeout struct {
//		*hid.Device
//	}
//
//	// Read overridden with ReadWithTimeout and with "Interrupted system call" error handling.
//	func (d *HIDWithTimeout) Read(p []byte) (n int, err error) {
//		for {
//			n, err = d.Device.ReadWithTimeout(p, 1*time.Second)
//			if err == nil || err.Error() != "Interrupted system call" {
//				return
//			}
//		}
//	}
//
//	func main() {
//		dev, _ := hid.OpenPath("/dev/hidraw6")
//		c = &ch347.IO{Dev: &HIDWithTimeout{dev}}
//	}
type HIDDev interface {
	io.ReadWriter
	SendFeatureReport(p []byte) (int, error)
}

// CH347 receives and sends 512 bytes long packets.
const maxPacketLen = 512
