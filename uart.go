package ch347

type UARTDataBits uint8
type UARTParity uint8
type UARTStopBit uint8

const (
	UARTDataBits5 UARTDataBits = iota
	UARTDataBits6
	UARTDataBits7
	UARTDataBits8
	UARTDataBits16
)

const (
	UARTParityNone UARTParity = iota
	UARTParityOdd
	UARTParityEven
	UARTParityMark
	UARTParitySpace
)

const (
	UARTStopBitOne UARTStopBit = iota
	UARTStopBitOneHalf
	UartStopBitTwo
)

func (c *UART) Set(baudRate uint32, dataBits UARTDataBits, parity UARTParity, stop UARTStopBit) error {
	// cmd		baud rate	?	stop bits	parity	data bits	timeout
	// cb0800	00c201		00	00			00		03			01
	p := []byte{
		0xcb, 0x08, 0x00, // cmd
		byte((baudRate >> 0) & 0xff), byte((baudRate >> 8) & 0xff), byte((baudRate >> 16) & 0xff),
		0x00,
		byte(stop), byte(parity), byte(dataBits), 0x00, /*timeout*/
	}

	_, err := c.Dev.SendFeatureReport(p)

	if err != nil {
		return err
	}

	return nil
}

// Read implementes reader interface.
func (c *UART) Read(b []byte) (int, error) {
	plen := len(b)

	// Maximum 510 bytes per reads.
	if plen > 510 {
		plen = 510
	}

	// 2 bytes length in the begining.
	p := make([]byte, plen+2)

	_, err := c.Dev.Read(p)
	if err != nil {
		return 0, err
	}

	n := (int(p[1]) << 8) | int(p[0])

	if n > len(b) {
		n = len(b)
	}

	copy(b[:n], p[2:])

	return n, nil
}

// Write implementes writer interface.
func (c *UART) Write(b []byte) (int, error) {
	plen := len(b)

	// Maximum 510 bytes per writes.
	if plen > 510 {
		plen = 510
	}
	p := make([]byte, plen+2)

	var pos, dlen, wlen int
	wlen = len(b)

	for pos < wlen {
		dlen = wlen - pos
		if dlen > plen {
			dlen = plen
		}

		p[0] = byte(dlen & 0xff)
		p[1] = byte((dlen >> 8) & 0xff)
		copy(p[2:], b[pos:pos+dlen])

		if dlen != plen {
			p = p[:2+dlen]
		}

		_, err := c.Dev.Write(p)
		if err != nil {
			return pos, err
		}

		pos += dlen
	}

	return pos, nil
}
