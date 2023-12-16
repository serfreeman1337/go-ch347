// The spi-flash command shows how to make simple flash programmer with CH347.
//
// Flash chip connection as follows:
//
//	  CH347       Flash chip
//	- SCS0    ->  CS
//	- MISO    ->  DO
//	- GND     ->  GND
//	- 3.3V    ->  VCC
//	- SCK     ->  CLK
//	- MOSI    ->  DI
package main

import (
	"flag"
	"fmt"
	"os"
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
	var isErase bool
	var toFile, fromFile string

	flag.BoolVar(&isErase, "e", false, "erase flash")
	flag.StringVar(&toFile, "r", "", "read flash contents to file")
	flag.StringVar(&fromFile, "w", "", "write flash contents from file")
	flag.Parse()

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

	fmt.Println("Configuring SPI")

	c := &ch347.IO{Dev: dev}
	// Note: Consult your flash chip datasheet for supported clocks.
	// In tests, W25Q32 worked only with 30Mhz, 1.875MHz and lower.
	err = c.SetSPI(ch347.SPIMode0, ch347.SPIClock1, ch347.SPIByteOrderMSB)

	if err != nil {
		panic(err)
	}

	flash := &Flash{c}
	size := flash.Capacity()
	if size == 0 {
		panic("No flash detected")
	}
	fmt.Println("Detected flash size:", size, "bytes")

	if isErase {
		fmt.Println("Erasing flash...")

		err = flash.Erase()
		if err != nil {
			panic(err)
		}

		fmt.Println("Done!")
		return
	}

	if toFile != "" {
		fmt.Println("Reading...")

		r := make([]byte, size)
		_, err = flash.Read(r)
		if err != nil {
			panic(err)
		}

		fmt.Println("Done!")

		err = os.WriteFile(toFile, r, 0666)
		if err != nil {
			panic(err)
		}
		return
	}

	if fromFile != "" {
		w, err := os.ReadFile(fromFile)
		if err != nil {
			panic(err)
		}

		fmt.Println("Writing...")

		_, err = flash.Write(w)
		if err != nil {
			panic(err)
		}

		fmt.Println("Done!")
		return
	}
}

type Flash struct {
	c *ch347.IO
}

// Capacity returns flash size by issuing JEDEC ID instruction 0x9f.
func (f *Flash) Capacity() int {
	w := []byte{0x9f} // JEDEC ID
	r := make([]byte, 3)

	f.c.SetCS(true)
	err := f.c.SPI(w, r)
	f.c.SetCS(false)

	if err != nil {
		return 0
	}

	size := 1
	for i := 0; i < int(r[2]); i++ {
		size *= 2
	}

	return size
}

// IsBusy checks status register 1 for busy flag.
func (f *Flash) IsBusy() bool {
	w := []byte{0x05} // Read status register.
	r := make([]byte, 1)

	f.c.SetCS(true)
	err := f.c.SPI(w, r)
	f.c.SetCS(false)

	if err != nil {
		return false
	}

	return r[0]&0x1 == 1
}

// WriteEnable issues write enable 0x06 or write disable 0x04 instruction.
func (f *Flash) WriteEnable(enable bool) {
	w := []byte{0x06} // Write Enable.

	if !enable {
		w[0] = 0x04 // Write Disable.
	}

	f.c.SetCS(true)
	f.c.SPI(w, nil)
	f.c.SetCS(false)
}

// Erase issues 0xc7 chip erase instruction and waits for it completion.
func (f *Flash) Erase() error {
	f.WriteEnable(true)

	w := []byte{0xc7} // Chip erase.

	f.c.SetCS(true)
	err := f.c.SPI(w, nil)
	f.c.SetCS(false)

	if err != nil {
		return err
	}

	for f.IsBusy() {
		time.Sleep(1 * time.Millisecond)
	}

	return nil
}

// Read reads flash contents starting from addr 0x000000.
func (f *Flash) Read(p []byte) (int, error) {
	addr := 0x00
	w := []byte{
		0x03,
		byte((addr >> 16) & 0xff),
		byte((addr >> 8) & 0xff),
		byte((addr) & 0xff),
	}

	f.c.SetCS(true)
	err := f.c.SPI(w, p)
	f.c.SetCS(false)

	if err != nil {
		return 0, err
	}

	return len(p), nil
}

// Write writes contents to flash by issuing page program instruction 0x02 starting from address 0x000000.
func (f *Flash) Write(p []byte) (int, error) {
	addr, dlen := 0, 256 // Up to 256 bytes can be programmed at a time using the Page Program instructions.

	w := make([]byte, 4+dlen)
	w[0] = 0x02 // Page program.

	for addr < len(p) {
		if (addr + dlen) > len(p) {
			dlen = len(p) - addr
		}

		w[1] = byte((addr >> 16) & 0xff)
		w[2] = byte((addr >> 8) & 0xff)
		w[3] = byte((addr) & 0xff)
		copy(w[4:], p[addr:addr+dlen])

		f.WriteEnable(true)

		f.c.SetCS(true)
		err := f.c.SPI(w, nil)
		f.c.SetCS(false)

		if err != nil {
			return addr, err
		}

		for f.IsBusy() {
			time.Sleep(1 * time.Millisecond)
		}

		addr += dlen
	}

	return addr, nil
}
