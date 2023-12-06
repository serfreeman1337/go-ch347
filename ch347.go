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

type HIDDev interface {
	io.ReadWriter
	SendFeatureReport(p []byte) (int, error)
}

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

// CH347 receives and sends 512 bytes long packets.
const maxPacketLen = 512
