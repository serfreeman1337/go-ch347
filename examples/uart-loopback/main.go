// The uart-loopback command shows reads and writes
// of the CH347 UART.
//
// To do the test, connect UART1 RX TX pins between each other and
// then run the example.
package main

import (
	"fmt"
	"io"
	"net/http"
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
	// Get path to the ch347 uart hidraw device.
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
	c := &ch347.UART{Dev: dev}
	err = c.Set(115200, ch347.UARTDataBits8, ch347.UARTParityNone, ch347.UARTStopBitOne)

	if err != nil {
		panic(err)
	}

	// Get some long lorem ipsum text from the 'lorem ipsum' generator that doesn't suck.
	fmt.Println("Getting some text")
	r, err := http.Get("https://loripsum.net/api/10/verylong/plaintext")
	if err != nil {
		panic(err)
	}

	lorem, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	// Send it over UART in a separate goroutine.
	go func() {
		fmt.Println("Sending it over UART")

		_, err := c.Write(lorem)
		if err != nil {
			fmt.Println(err)
		}
	}()

	fmt.Println("Receiving it over UART")

	start := time.Now()

	// Add receive all of it.
	rbuf := make([]byte, len(lorem))
	n, err := io.ReadAtLeast(c, rbuf, len(rbuf))

	took := time.Since(start)

	if err != nil {
		panic(err)
	}

	fmt.Println("Read", n, "bytes:")
	fmt.Println("------")
	fmt.Println(string(rbuf))
	fmt.Println("------")
	fmt.Println("time:", took, "effective speed:", float64(len(rbuf)*8)/took.Seconds(), "bps")

	// Let's confirm it just in case.
	for i, a := range rbuf {
		if lorem[i] != a {
			panic(fmt.Sprintf("Byte %d doesn't match!!! %c != %c", i, lorem[i], a))
		}
	}

	fmt.Println("Success!")
}
