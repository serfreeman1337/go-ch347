package ch347

import "errors"

var (
	ErrI2CRead  = errors.New("i2c read failed")
	ErrI2CWrite = errors.New("i2c write failed")
)

type I2CMode uint8

const (
	I2CMode0 I2CMode = iota // Low rate 20KHz.
	I2CMode1                // Standart rate 100KHz.
	I2CMode2                // Fast rate 400KHz.
	I2CMode3                // High rate 750KHz.
)

// SetI2C configures the interface with a specified mode.
//   - I2CMode0 - Low rate 20KHz.
//   - I2CMode1 - Standart rate 100KHz.
//   - I2CMode2 - Fast rate 400KHz.
//   - I2CMode3 - High rate 750KHz.
func (c *IO) SetI2C(mode I2CMode) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	p := []byte{0x03, 0x00, 0xaa, 0x60 | byte(mode), 0x00}
	_, err := c.Dev.Write(p)
	return err
}

// I2C performs write and read operations with device on given address.
//
// Example:
//
//	// Read all 4096 bytes from 24C32B chip
//	w := []byte{0x00, 0x00} // "Random read". Write page address first.
//	r := make([]byte, 4096) // Allocate buffer for 4096 bytes to be read.
//	err = c.I2C(0x57, w, r)
//
//	if err != nil {
//		return err
//	}
//
//	// Print result as a string
//	fmt.Println(string(r))
func (c *IO) I2C(addr uint16, w, r []byte) error {
	const (
		// The command package of the I2C interface, starting from the secondary byte, is the I2C command stream
		CmdI2CStream = 0xAA

		// Command flow of I2C interface: generate start bit
		CmdI2CStart = 0x74

		// Command flow of I2C interface: generate stop bit
		CmdI2CStop = 0x75

		// Command flow of I2C interface: output data, bit 5 - bit 0 is the length, subsequent bytes are data, and length 0 only sends one byte and returns an answer
		CmdI2CWrite = 0x80

		// I2C interface command flow: input data, bit 5 - bit 0 is the length, and 0 length only receives one byte and sends no response
		CmdI2CRead = 0xc0 // Note: a reads must be completed with one byte reading (0xc0), otherwise next operation will fail.
	)

	const maxLen = 63 // Max data length with 6 bits.

	p := make([]byte, 0, 512)

	// Counters to confirm writes or reads of I2C bytes.
	toWrite := 0
	toRead := 0
	rpos := 0
	hasRead := false

	write := func() error {
		p = append(p, 0x00) // End packet with 0x00.

		// Calucate and set length in the first 2 bytes.
		plen := len(p) - 2
		p[0] = byte(plen & 0xff)
		p[1] = byte((plen >> 8) & 0xff)

		c.mu.Lock()
		defer c.mu.Unlock()

		_, err := c.Dev.Write(p)
		if err != nil {
			return err
		}

		// Confirm I2C operation.
		if clen := (toWrite + toRead); clen > 0 {
			if hasRead {
				clen++
			}

			rlen := (2 + clen)
			p = p[:rlen]

			_, err = c.Dev.Read(p)
			if err != nil {
				return err
			}

			pos := 2 // Skip 2 bytes in begining.

			// Confirm writes.
			for toWrite > 0 {
				if p[pos] == 0x00 {
					// pos += toWrite
					// toWrite = 0
					// break
					return ErrI2CWrite
				}

				toWrite--
				pos++
			}

			// Confirm reads.
			if toRead > 0 {
				if hasRead { // Confirm read request.
					hasRead = false

					if p[pos] != 0x01 {
						// pos += toRead
						return ErrI2CRead
					}

					pos++
				}

				copy(r[rpos:rpos+toRead], p[pos:pos+toRead]) // Copy i2c bytes to reader buffer.

				rpos += toRead
				toRead = 0
			}
		}

		p = p[:0] // Start new packet.
		return nil
	}

	pack := func(elems ...byte) error {
		// First make it work, then make it better.
		if (len(p) + len(elems)) >= (maxPacketLen - 2) {
			if err := write(); err != nil {
				return err
			}

			// p = p[:0]
		}

		if len(p) == 0 {
			p = append(p, 0x00, 0x00)   // Every packet starts with length.
			p = append(p, CmdI2CStream) // CMD byte.
		}

		p = append(p, elems...)
		return nil
	}

	if wlen := len(w); wlen != 0 {
		err := pack(CmdI2CStart)
		if err != nil {
			return err
		}

		pos := 0
		d := []byte{CmdI2CWrite} // Start with length, will be calculated at the end.

		var dlen int
		for pos < wlen {
			// Calc potential write part length.
			dlen = (wlen - pos)

			if dlen > maxLen {
				dlen = maxLen
			}

			if pos == 0 && dlen == maxLen {
				dlen--
			}

			if pos == 0 { // Write addr, yes.
				d = append(d, byte(addr<<1))
			}

			// Write data.
			d = append(d, w[pos:pos+dlen]...)
			pos += dlen

			dlen = len(d) - 1
			if pos == 0 {
				dlen++ // Oh.
			}

			d[0] = CmdI2CWrite | byte(dlen) // Length in the begining.

			err = pack(d...)
			if err != nil {
				return err
			}

			d = d[:1] // Reset write part.

			toWrite += dlen
		}
	}

	if rlen := len(r); rlen != 0 {
		// Read request.
		d := []byte{
			CmdI2CStart, CmdI2CWrite | 1, byte(addr<<1) | 1,
		}
		hasRead = true

		// For some reason reading must end with one byte reading (0xc0).
		maxRLen := 64
		dlen := rlen
		send := false

		for rlen > 0 {
			dlen = rlen
			if dlen > maxLen {
				dlen = maxLen
			}

			if nlen := (2 + toWrite + toRead + dlen); nlen >= maxPacketLen {
				dlen -= (nlen - maxPacketLen)
				send = true

				if hasRead {
					toRead--
				}
			}

			if maxRLen == 63 {
				d = append(d, CmdI2CRead|byte(dlen))
			} else if dlen > 1 {
				// Account for extra byte (0xc0) that needs to be send to finish reading.
				d = append(d, CmdI2CRead|byte(dlen)-1)
			}

			if maxRLen == 64 {
				maxRLen = 63
			}

			toRead += dlen

			if send {
				send = false

				err := pack(d...)
				if err != nil {
					return err
				}

				err = write()
				if err != nil {
					return err
				}

				// p = p[:0]
				d = d[:0]
			}

			rlen -= dlen
		}

		// I have no idea anymore.
		if !hasRead {
			toRead++
		}

		d = append(d, CmdI2CRead)
		err := pack(d...)
		if err != nil {
			return err
		}
	}

	err := pack(CmdI2CStop)
	if err != nil {
		return err
	}

	err = write()
	if err != nil {
		return err
	}

	return nil
}
