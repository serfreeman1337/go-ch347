package ch347

import (
	"errors"
)

var (
	ErrInvalidResponse = errors.New("invalid response")
)

type SPIMode uint8

const (
	SPIMode0 SPIMode = iota
	SPIMode1
	SPIMode2
	SPIMode3
)

type SPIClock uint8

const (
	SPIClock0 SPIClock = iota // 60 MHz
	SPIClock1                 // 30 MHz
	SPIClock2                 // 15 MHz
	SPIClock3                 // 7.5 MHz
	SPIClock4                 // 3.75 Mhz
	SPIClock5                 // 1.875 MHz
	SPIClock6                 // 937.5 KHz
	SPIClock7                 // 468.75 KHz
)

type SPIByteOrder uint8

const (
	SPIByteOrderMSB SPIByteOrder = iota
	SPIByteOrderLSB
)

// SetSPI configures the interface with a specified mode, clock, and byte order.
//   - SPIClock0 - 60 MHz.
//   - SPIClock1 - 30 MHz.
//   - SPIClock2 - 15 MHz.
//   - SPIClock3 - 7.5 MHz.
//   - SPIClock4 - 3.75 Mhz.
//   - SPIClock5 - 1.875 MHz.
//   - SPIClock6 - 937.5 KHz.
//   - SPIClock7 - 468.75 KHz.
func (c *IO) SetSPI(mode SPIMode, clock SPIClock, byteOrder SPIByteOrder) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	p := make([]byte, 0, 29)

	p = append(p, 0x1d, 0x00)

	// byte 0 - CMD
	// bytes 1-8 - ??
	p = append(p, 0xc0, 0x1a, 0x00, 0x00, 0x00, 0x04, 0x01, 0x00, 0x00)

	// bytes 9-13 - SPI Mode
	switch mode {
	case SPIMode0:
		p = append(p, 0x00, 0x00, 0x00, 0x00)
	case SPIMode1:
		p = append(p, 0x00, 0x00, 0x01, 0x00)
	case SPIMode2:
		p = append(p, 0x02, 0x00, 0x00, 0x00)
	case SPIMode3:
		p = append(p, 0x02, 0x00, 0x01, 0x00)
	}

	// bytes 14-15 - ???
	p = append(p, 0x00, 0x02)

	// Bad Apple!! benchmark:
	//     60MHz	time: 5.01151691s		size: 6572 KiB		rate: 1311.3793923125763 KiB/s
	//     30MHz	time: 6.665416534s		size: 6572 KiB		rate: 985.9848917883096 KiB/s
	//     15MHz	time: 8.385592473s		size: 6572 KiB		rate: 783.7251835407671 KiB/s
	//    7.5MHz	time: 11.87835496s		size: 6572 KiB		rate: 553.2752659885153 KiB/s
	//   3.75MHz	time: 19.715254824s		size: 6572 KiB		rate: 333.3459323081991 KiB/s
	//  1.875MHz	time: 35.687317732s		size: 6572 KiB		rate: 184.1550561281617 KiB/s
	//  937.5KHz	time: 1m3.68030119s		size: 6572 KiB		rate: 103.203029464189 KiB/s
	// 468.75KHz	time: 2m2.683078174s	size: 6572 KiB		rate: 53.56891999953741 KiB/s

	// byte 16 - SPI Clock
	// - 60Mhz     - 00	   - 00000000
	// - 30Mhz     - 08    - 00001000
	// - 15Mhz     - 10    - 00010000
	// - 7.5Mhz    - 18    - 00011000
	// - 3.75MHz   - 20    - 00100000
	// - 1.875MHz  - 28    - 00101000
	// - 937.5KHz  - 30    - 00110000
	// - 468.75KHz - 38    - 00111000
	p = append(p, byte(clock<<3))

	// byte 17 - ???
	p = append(p, 0x00)

	// byte 18 - byte order
	// - LSB - 80 - 10000000
	// - MSB - 00 - 00000000
	p = append(p, byte(byteOrder)<<7)

	// 19-21 byte - ???
	p = append(p, 0x00, 0x07, 0x00)

	// 22-23 byte - read write interval
	// What ?
	p = append(p, 0x00, 0x00)

	// 24 byte - default data
	// Output MISO data during MOSI read ?
	p = append(p, 0xff)

	// 25 byte - CS Polarity
	// 0x80 - active high CS0
	// 0x40 - active high CS1
	p = append(p, 0x00)

	// 26-30
	p = append(p, 0x00, 0x00, 0x00, 0x00)

	_, err := c.Dev.Write(p)
	if err != nil {
		return err
	}

	// Read response.
	p = p[:6]
	// 0400 c0 01 00 00
	_, err = c.Dev.Read(p)
	if err != nil {
		return err
	}

	if p[2] != 0xc0 && p[3] != 0x01 {
		// return fmt.Errorf("invalid response. expected (0xc0 0x01), got (0x%02x 0x%02x)", p[2], p[3])
		return ErrInvalidResponse
	}

	return nil
}

// SPI performs write and read operations.
func (c *IO) SPI(w, r []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(r) != 0 { // Sorry, I don't have any available devices to test reads.
		return errors.ErrUnsupported
	}

	const CmdSPIWrite byte = 0xc4

	wlen := len(w)
	p := make([]byte, 0, 512)

	write := func() error {
		if len(p) <= 2 { // Nothing to write.
			return nil
		}

		// Set length in the first 2 bytes.
		plen := len(p) - 2
		p[0] = byte(plen & 0xff)
		p[1] = byte((plen >> 8) & 0xff)

		_, err := c.Dev.Write(p)
		if err != nil {
			return err
		}

		// Confirm write.
		p = p[:5]
		_, err = c.Dev.Read(p)
		if err != nil {
			return err
		}
		if p[2] != 0xc4 && p[3] != 0x01 {
			// return fmt.Errorf("invalid response. expected (0x%02x 0x%02x %02x 0x%02x). got (0x%02x 0x%02x %02x 0x%02x)",
			// 	0x03, 0x00, 0xc4, 0x01,
			// 	p[0], p[1], p[2], p[3],
			// )
			return ErrInvalidResponse
		}

		p = p[:2]
		return nil
	}

	const maxDataLen = 509
	const maxOpLen = 65535 // Max data length of single SPI Write (0xc4) operation.

	var pos, plen, nlen, olen, dlen int

	for pos < wlen {
		if olen == 0 {
			nlen = (wlen - pos)
			if nlen > maxOpLen {
				nlen = maxOpLen
			}

			// Start new packet.
			p = append(p, 0x00, 0x00, CmdSPIWrite, byte(nlen)&0xff, byte(nlen>>8)&0xff)
		}

		// Calculate data length within packet.
		dlen = wlen - pos
		if plen = len(p); (plen + dlen) > maxDataLen {
			dlen = maxDataLen - plen
		}

		// Calculate data length within single write operation.
		if nlen = (olen + dlen); nlen > maxOpLen {
			dlen = maxOpLen - olen
		}

		p = append(p, w[pos:pos+dlen]...)

		// Send packet.
		if len(p) >= maxDataLen {
			err := write()
			if err != nil {
				return err
			}
		}

		pos += dlen
		olen += dlen

		// Finish single write operation and start new one.
		if olen == maxOpLen {
			err := write()
			if err != nil {
				return err
			}

			p = p[:0]
			olen = 0
		}
	}

	return write()
}

// SetCS asserts CS0 pin.
func (c *IO) SetCS(enable bool) error {
	return c.setCS(0, enable)
}

// SetCS1 asserts CS1 pin.
func (c *IO) SetCS1(enable bool) error {
	return c.setCS(1, enable)
}

func (c *IO) setCS(cs int, enable bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	const CmdSPICS byte = 0xc1

	p := []byte{
		0x0d, 0x00, CmdSPICS, 0x0a, 0x00,
		0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00,
		0x00, 0x00, 0x00, 0x00,
	}

	pos := 5 + 5*cs

	if enable {
		p[pos] = 0x80
	} else {
		p[pos] = 0xc0
	}

	_, err := c.Dev.Write(p)
	return err
}
