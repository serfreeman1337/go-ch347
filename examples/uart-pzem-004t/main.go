// The uart-pzem-004t command demonstrates how to achieve reads with timeouts
// using PZEM 004T Multimeter measurements as an example.
//
// Connect PZEM 004T like it shown below and then run the example.
//
//	CH347 UART1      PZEM 004T TTL
//	3.3V         ->  VCC
//	TX           ->  RX
//	RX           ->  TX
//	GND          ->  GND
package main

import (
	"fmt"
	"io"
	"time"

	"github.com/serfreeman1337/go-ch347"
	"github.com/sstallion/go-hid"
)

const (
	UART int = 0
	IO   int = 1
)

// DevPath returns CH347 hidraw path.
//
// Allowed ifaces:
//   - 0 - UART
//   - 1 - SPI+I2C+GPIO
func DevPath(iface int) string {
	var devPath string

	// Don't forget to allow access to hidraw:
	// sudo chmod 777 /dev/hidraw{5,6}
	// hidraw numbers can be checked with the `dmesg` command.

	// Locate HID device.
	// ID 1a86:55dc QinHeng Electronics
	var devInfos []*hid.DeviceInfo
	hid.Enumerate(0x1a86, 0x55dc, func(info *hid.DeviceInfo) error {
		devInfos = append(devInfos, info)
		return nil
	})

	for _, di := range devInfos {
		// InterfaceNbr == 0 - UART
		// InterfaceNbr == 1 - SPI+I2C+GPIO
		if di.ProductStr == "HID To UART+SPI+I2C" && di.InterfaceNbr == iface {
			devPath = di.Path
			break
		}
	}

	return devPath
}

// HIDWithTimeout overrides the Read method with the ReadWithTimeout one.
// Setting fixed 100ms timeout reads will prevent indefinite blocking when
// there is no response on UART.
type HIDWithTimeout struct {
	*hid.Device
}

// Read overrided with ReadWithTimeout.
func (d *HIDWithTimeout) Read(p []byte) (int, error) {
	return d.ReadWithTimeout(p, 100*time.Millisecond)
}

func main() {
	devPath := DevPath(UART)
	if len(devPath) == 0 {
		panic("no CH347 found")
	}

	fmt.Println("Opening", devPath)
	dev, err := hid.OpenPath(devPath)
	if err != nil {
		panic(err)
	}
	defer dev.Close()

	// Create CH347 device and set UART config.
	c := &ch347.UART{Dev: &HIDWithTimeout{dev}}
	err = c.Set(9600, ch347.UARTDataBits8, ch347.UARTParityNone, ch347.UARTStopBitOne)

	if err != nil {
		panic(err)
	}

	pzem := PZEM004{c}
	var r = PZEM004Reading{}

	// Reading loop.
	for {
		err := pzem.ReadAll(&r)

		if err != nil {
			fmt.Println("---------------", err, time.Now())
			continue
		}

		fmt.Printf("- %.01f V - %.03f A - %.01f W - %.0f Wh - %.02f Hz - %.02f Pf - %v\n",
			r.V, r.A, r.W, r.Wh, r.F, r.Pf,
			time.Now(),
		)

		// PZEM004 updates values every 1 second.
		// No point in reading faster than that.
		time.Sleep(1 * time.Second)
	}
}

type PZEM004Reading struct {
	V, A, W, Wh, F, Pf float32
}

type PZEM004 struct {
	dev io.ReadWriter
}

func (pzem *PZEM004) ReadAll(r *PZEM004Reading) error {
	const serverAddr uint8 = 0xf8 // Modbus server addr.
	const regAddr uint16 = 0x0000 // Modbus register address.
	const count uint16 = 0x09     // Number of regs.

	const rlen = count*2 + 5

	p := make([]byte, 0, rlen)

	// Modbus request payload.
	p = append(p,
		serverAddr,
		0x04,                  // Read input register.
		byte(regAddr>>8)&0xff, // Reg Addr MSB,
		byte(regAddr)&0xff,    // Reg Addr LSB.
		byte(count>>8)&0xff,   // Number of Reg MSB
		byte(count)&0xff,      // Number of Reg LSB
	)

	crc := crc16(p)
	p = append(p, byte(crc)&0xff, byte(crc>>8)&0xff)

	_, err := pzem.dev.Write(p)
	if err != nil {
		return err
	}

	// Modbus response payload.
	p = p[:rlen]
	_, err = pzem.dev.Read(p)
	if err != nil {
		return err
	}

	// Confirm response CRC.
	crc = crc16(p[:len(p)-2])

	if p[len(p)-2] != byte(crc&0xff) || p[len(p)-1] != byte(crc>>8)&0xff {
		return fmt.Errorf("crc check failed")
	}

	// I'm sorry.
	r.V = float32((uint32(p[3])<<8)|uint32(p[4])) / 10.0
	r.A = float32((((uint32(p[7])<<8)|uint32(p[8]))<<16)|((uint32(p[5])<<8)|uint32(p[6]))) / 1000.0
	r.W = float32((((uint32(p[11])<<8)|uint32(p[12]))<<16)|((uint32(p[9])<<8)|uint32(p[10]))) / 10.0
	r.Wh = float32((((uint32(p[15]) << 8) | uint32(p[16])) << 16) | ((uint32(p[13]) << 8) | uint32(p[14])))
	r.F = float32((uint32(p[17])<<8)|uint32(p[18])) / 10.0
	r.Pf = float32((uint32(p[19])<<8)|uint32(p[20])) / 100.0

	return nil
}

func crc16(p []byte) uint16 {
	crc := uint16(0xffff)

	for _, a := range p {
		crc ^= uint16(a)

		for i := 8; i != 0; i-- {
			if (crc & 0x0001) != 0 {
				crc >>= 1
				crc ^= 0xA001
			} else {
				crc >>= 1
			}
		}
	}

	return crc
}
