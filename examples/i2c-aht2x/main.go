// The i2c-aht2x command shows how to use CH347 I2C interface by
// reading temperatue and humidity from AHT2X sensor.
package main

import (
	"fmt"
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

func main() {
	devPath := DevPath(IO)
	if len(devPath) == 0 {
		panic("no CH347 found")
	}

	fmt.Println("Opening", devPath)
	dev, err := hid.OpenPath(devPath)
	if err != nil {
		panic(err)
	}
	defer dev.Close()

	// Create CH347 device and set I2C config.
	c := &ch347.IO{Dev: dev}
	err = c.SetI2C(ch347.I2CMode1)
	if err != nil {
		panic(err)
	}

	const addr = 0x38             // AHT2X I2C Address.
	w := []byte{0xac, 0x33, 0x00} // Trigger mesurements.
	r := make([]byte, 7)          // Read 7 bytes of measurements data.

	var tRaw, hRaw uint32
	var t, h float32

	for {
		time.Sleep(100 * time.Millisecond)

		// First write measurements command.
		err = c.I2C(addr, w, nil)
		if err != nil {
			fmt.Println("---", err, "-", time.Now())
			continue
		}

		// Then wait 80ms for measurements to be completed.
		time.Sleep(80 * time.Millisecond)

		// Then read them.
		err = c.I2C(addr, nil, r)
		if err != nil {
			fmt.Println("---", err, time.Now())
			continue
		}

		// Check the crc because why not?
		if r[6] != crc8(r[:6]) {
			fmt.Println("--- crc check failed", "-", time.Now())
			continue
		}

		if r[0] != 0x1c {
			fmt.Println("--- device is busy", r[0], "-", time.Now())
			continue
		}

		// Perform conversion.
		tRaw = ((uint32(r[3]) & 0x0f) << 16) | (uint32(r[4]) << 8) | uint32(r[5])
		hRaw = (uint32(r[1]) << 12) | (uint32(r[2]) << 4) | (uint32(r[3]) & 0xf0)

		t = (float32(tRaw)/0x100000)*200 - 50
		h = (float32(hRaw) / 0x100000) * 100

		fmt.Printf("--- %.02f°C - %.02f %% - %v\n", t, h, time.Now())
	}
}

func crc8(p []byte) uint8 {
	crc := uint8(0xff)

	for _, a := range p {
		crc ^= a

		for i := 8; i > 0; i-- {
			if crc&0x80 != 0x00 {
				crc = (crc << 1) ^ 0x31
			} else {
				crc = (crc << 1)
			}
		}
	}

	return crc
}
