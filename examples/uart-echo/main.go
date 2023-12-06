// The uart-echo command shows the ReadWriter interface implementation
// of the CH347 UART by doing a simple echo test.
//
// How to test on linux:
//
//  1. Connect your other USB TTL as follows:
//
//     - CH347 UART1      USB to TTL converter
//
//     - TX           ->  RX
//
//     - RX           ->  TX
//
//     - GND          ->  GND
//
//  2. On the first terminal, set baud rate and start reading to see the input:
//
//     stty -F /dev/ttyUSB0 115200 -echo
//
//     cat /dev/ttyUSB0
//
//  3. Start echo example with `go run` etc.
//
//  4. On the second terminal, send whatever you like to be seen on your first terminal:
//
//     echo "Whatever" > /dev/ttyUSB0
package main

import (
	"fmt"
	"io"

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

	// Print some welcome message.
	fmt.Fprintln(c, "You are about to enter an echo test. In this mode everything you say will be repeated back to you just as soon as it is received. The purpose of this test is to give you an audible sense of the latency between you and the machine that is running the echo test application. You may end the test by hanging up or by pressing the pound key.")

	fmt.Println("Now send something you want to recieve back over UART")

	// Now simply write back anything read.
	io.Copy(c, c)
}
