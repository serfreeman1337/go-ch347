// The spi-ssd1306-bad-apple command shows how to use the CH347 SPI interface
// by playing the "Bad Apple!!" video on the SSD1306 OLED display.
//
// SSD1306 SPI OLED Display connection as follows:
//
//	  CH347       SSD1306 SPI OLED 128x64 display
//	- 3.3V    ->  VCC
//	- GND     ->  GND
//	- SCK     ->  D0
//	- MOSI    ->  D1
//	- MISO    ->  DC
//	- CS0     ->  CS
//	- CS1     ->  RES
package main

//  1. Connect your other USB TTL as follows:
//
//     - CH347 UART1      USB to TTL converter
//
//     - TX           ->  RX
//
//     - RX           ->  TX
//
//     - GND          ->  GND

import (
	"fmt"
	"time"

	"github.com/kkdai/youtube/v2"
	"github.com/serfreeman1337/go-ch347"
	"github.com/sstallion/go-hid"
	ffmpeg "github.com/u2takey/ffmpeg-go"
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

	fmt.Println("Configuring SPI")

	// Create CH347 device and set SPI config.
	c := &ch347.IO{Dev: dev}
	err = c.SetSPI(ch347.SPIMode0, ch347.SPIClock0, ch347.SPIByteOrderMSB)

	if err != nil {
		panic(err)
	}

	// Get YouTube video stream reader.
	fmt.Println("Getting YouTube Video")
	videoID := "FtutLA63Cp8"
	yt := youtube.Client{}

	video, err := yt.GetVideo(videoID)
	if err != nil {
		panic(err)
	}

	fmt.Println("-", video.Title, video.Duration)

	format := video.Formats.FindByQuality("360p")
	r, _, err := yt.GetStream(video, format)

	if err != nil {
		panic(err)
	}
	defer r.Close()

	// Create SSD1306 stream writer.
	fmt.Println("Configuring SSD1306")
	w, err := NewSSD1306(c, format.FPS)
	if err != nil {
		panic(err)
	}
	defer w.Close()

	// Now encode streams with ffmpeg.
	fmt.Println("Playing")
	err = ffmpeg.Input("pipe:").
		Output("pipe:",
			ffmpeg.KwArgs{
				"filter:v": "scale=128:64",
				"format":   "rawvideo", "pix_fmt": "gray",
			}).
		WithInput(r).
		WithOutput(w).
		Run()

	if err != nil {
		panic(err)
	}
}

// SSD1306 implements WriteCloser interface.
type SSD1306 struct {
	c           *ch347.IO
	buf         []byte
	x, y        int
	nextFrameAt time.Time
	frameTime   time.Duration
}

// NewSSD1306 inits 128x64 SPI OLED display.
func NewSSD1306(c *ch347.IO, fps int) (*SSD1306, error) {
	// For 128x64.
	mux := byte(64 - 1)
	com_pins := byte(0x12)
	contrast := byte(0xff)

	const RST = ch347.GPIO5 // SCS1
	const DC = ch347.GPIO1  // MISO

	// Trigger RST sequence.
	c.WritePin(RST, true, true)
	time.Sleep(1 * time.Millisecond)

	c.WritePin(RST, true, false)
	time.Sleep(10 * time.Millisecond)

	c.WritePin(RST, true, true)

	// Init sequence.
	c.WritePin(DC, true, false) // Switch to cmd mode.
	w := []byte{
		0xae,       // SSD1306_CMD_DISPLAY_OFF
		0xd5, 0x80, // SSD1306_CMD_SET_DISPLAY_CLK_DIV // follow with 0x80
		0xa8, mux, // SSD1306_CMD_SET_MUX_RATIO //  follow with 0x3F = 64 MUX
		0xd3, 0x00, // SSD1306_CMD_SET_DISPLAY_OFFSET // // follow with 0x00
		0x40,       // SSD1306_CMD_SET_DISPLAY_START_LINE
		0x8D, 0x14, // SSD1306_CMD_SET_CHARGE_PUMP // follow with 0x14
		0x20, 0x00, // SSD1306_CMD_SET_MEMORY_ADDR_MODE // SSD1306_CMD_SET_HORI_ADDR_MODE
		0xa1,           // SSD1306_CMD_SET_SEGMENT_REMAP_1
		0xc8,           // SSD1306_CMD_SET_COM_SCAN_MODE
		0xda, com_pins, // SSD1306_CMD_SET_COM_PIN_MAP
		0x81, contrast, // SSD1306_CMD_SET_CONTRAST
		0xd9, 0xf1, // SSD1306_CMD_SET_PRECHARGE // follow with 0xF1
		0xd8, 0x40, // SSD1306_CMD_SET_VCOMH_DESELCT
		0xa4, // SSD1306_CMD_DISPLAY_RAM
		0xa6, // SSD1306_CMD_DISPLAY_NORMAL
		0xaf, // SSD1306_CMD_DISPLAY_ON

		//
		0x21, 0x00, 0x7f, // SSD1306_CMD_SET_COLUMN_RANGE // follow with 0x00 and 0x7F = COL127
		0x22, 0x00, 0x07, // SSD1306_CMD_SET_PAGE_RANGE // follow with 0x00 and 0x07 = PAGE7
	}

	c.SetCS(true)
	err := c.SPI(w, nil)
	c.SetCS(false)

	if err != nil {
		return nil, err
	}

	c.WritePin(DC, true, true) // Switch to data mode.

	// Calculate time between frames.
	var ft time.Duration
	if fps > 0 {
		eh := 1 / float32(fps)
		ft, _ = time.ParseDuration(fmt.Sprintf("%fs", eh))
	}

	return &SSD1306{
		c:         c,
		buf:       make([]byte, 128*8),
		frameTime: ft,
	}, nil
}

// Write performs conversion to SSD1306 format and displays buffer every 8192 bytes written.
func (w *SSD1306) Write(p []byte) (int, error) {
	var page, pageRow, pageCol int

	for _, a := range p {
		page = w.y / 8
		pageRow = w.y % 8
		pageCol = w.x

		// Set pixel bit.
		if a > 127 { // True. Threshold value. Set pixel bit if intensity of that pixel is greater than 127.
			w.buf[page*128+pageCol] |= (1 << pageRow)
		} else { // False.
			w.buf[page*128+pageCol] &= ^(1 << pageRow)
		}

		w.x++

		if w.x > 127 {
			w.x = 0
			w.y++
			if w.y > 63 {
				w.y = 0
				err := w.display()

				if err != nil {
					return 0, err
				}
			}
		}
	}

	return len(p), nil
}

// Close displays any remaining buffer.
func (w *SSD1306) Close() error {
	if w.x == 0 && w.y == 0 {
		return nil
	}
	return w.display()
}

func (w *SSD1306) display() error {
	if w.frameTime > 0 {
		if time.Now().Before(w.nextFrameAt) {
			time.Sleep(time.Until(w.nextFrameAt))
		}

		w.nextFrameAt = time.Now().Add(w.frameTime)
	}

	w.c.SetCS(true)
	err := w.c.SPI(w.buf, nil)
	w.c.SetCS(false)

	return err
}
