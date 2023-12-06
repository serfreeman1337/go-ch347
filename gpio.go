package ch347

import "fmt"

// Pin represents available pins for GPIO operations.
type Pin uint8

const (
	// CTS0/SCK/TCK
	GPIO0 Pin = iota

	// RTS0/MSIO/TDO
	GPIO1

	// DSR0/SCS0/TMS
	GPIO2

	// SCL
	GPIO3

	// ACT
	GPIO4

	// DTR0/TNOW0/SCS1/TRST
	GPIO5

	// CTS1
	GPIO6

	// RTS1
	GPIO7
)

// WritePin sets given pin operation mode.
//
// Example:
//
//	// Blink ACT led (GPIO4).
//	st := false
//	for {
//		err := WritePin(GPIO4, true, st)
//
//		if err != nil {
//			fmt.Println(err)
//			break;
//		}
//
//		if st {
//			st = false
//		} else {
//			st = true
//		}
//
//		time.Sleep(100*time.Miliseconds)
//	}
func (c *IO) WritePin(pin Pin, output bool, level bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	//		CMD	 LEN? 	PINS
	// 0b00  cc	08 00	c8 00 08 08 00 08 08 08
	// Pins:
	// 00 - 00000000 - ignore ?
	// 08 - 00001000 - disabled ?
	// c0 - 11000000 - enabled / input / ?
	// c8 - 11001000 - enabled / input / ?
	// f0 - 11110000 - enabled / output / off
	// f8 - 11111000 - enabled / output / on
	p := []byte{0x0b, 0x00, 0xcc, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	pos := 5 + pin

	if output {
		if level {
			p[pos] = 0xf8
		} else {
			p[pos] = 0xf0
		}
	} else {
		p[pos] = 0xc0
	}

	_, err := c.Dev.Write(p)
	if err != nil {
		return err
	}

	// Device returns whole gpio status.
	_, err = c.Dev.Read(p)
	if err != nil {
		return err
	}

	if p[0] != 0x0b || p[2] != 0xcc {
		return fmt.Errorf("invaid response. expected (0x%02x 0x%02x 0x%02x), got (0x%02x 0x%02x 0x%02x)",
			0x0b, 0x00, 0xcc,
			p[0], p[1], p[2],
		)
	}

	// 00 = 00000000 // input on ?
	// 40 = 01000000 // input off ?
	// 80 = 10000000 // output off
	// c0 = 11000000 // output on

	// Confirm pin state.
	err = nil
	if output {
		mask := byte(0x80) // Check bit 7 for output.
		if level {
			mask |= 0x40 // Check bit 6 for output level.
		}

		if p[pos]&mask == 0x00 {
			err = fmt.Errorf("gpio set as output failed, got 0x%02x", p[pos])
		}
	} else {
		if p[pos]&0x80 != 0x00 { // Bit 7 is still set (this pin is still output) ?
			err = fmt.Errorf("gpio set as input failed, got 0x%02x", p[pos])
		}
	}

	return err
}

// ReadPin returns given pin level.
//
// For output pin "true" means there is +3.3V on this pin.
//
// For input pin "true" means this pin is shorted to GND.
func (c *IO) ReadPin(pin Pin) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	p := []byte{0x0b, 0x00, 0xcc, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	_, err := c.Dev.Write(p)
	if err != nil {
		return false, err
	}

	_, err = c.Dev.Read(p)
	if err != nil {
		return false, err
	}

	if p[0] != 0x0b || p[2] != 0xcc {
		return false, fmt.Errorf("invaid response. expected (0x%02x 0x%02x 0x%02x), got (0x%02x 0x%02x 0x%02x)",
			0x0b, 0x00, 0xcc,
			p[0], p[1], p[2],
		)
	}

	pos := 5 + pin

	// 00 = 00000000 // input on ?
	// 40 = 01000000 // input off ?
	// 80 = 10000000 // output off
	// c0 = 11000000 // output on
	if p[pos]&0x80 != 0x00 { // Pin is output.
		if p[pos]&0x40 != 0x00 { // Pin level is high.
			return true, nil
		} else {
			return false, nil
		}
	} else { // Pin is input.
		if p[pos]&0x40 != 0x00 { // Pin level is low ?
			return false, nil
		} else {
			return true, nil
		}
	}
}
