package rtltcp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommand(t *testing.T) {
	testCases := []struct {
		name     string
		c        command
		expected []byte
	}{
		{"centerfreq", command{centerFreq, 1e+08}, []byte{0x01, 0x05, 0xf5, 0xe1, 0x00}},
	}

	var buf bytes.Buffer
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf.Truncate(0)

			err := binary.Write(&buf, binary.BigEndian, tc.c)
			if err != nil {
				t.Error("binary.Write failed:", err)
			}
			if !bytes.Equal(buf.Bytes(), tc.expected) {
				t.Errorf("expected %x, recieved %x", tc.expected, buf.Bytes())
			}
			t.Logf("command: %q (%d) value: %v, wire: %0 x",
				tc.name,
				tc.c.command,
				tc.c.Parameter,
				buf.Bytes(),
			)
		})
	}
}

func TestConnection(t *testing.T) {
	var sdr SDR
	var wg sync.WaitGroup
	var remote net.Conn
	remote, sdr.Conn = net.Pipe()

	wg.Add(1)
	go func() {
		// pretend we're the remote dongle
		di := DongleInfo{
			Magic:     dongleMagic,
			Tuner:     1,
			GainCount: 10,
		}
		err := binary.Write(remote, binary.BigEndian, di)
		if !assert.Nil(t, err, "writing dongle info") {
			t.FailNow()
		}

		b := make([]byte, 60)
		n, err := io.ReadFull(remote, b)
		assert.Nil(t, err, "readfull")

		expected := []byte{
			0x01, 0x05, 0xf5, 0xe1, 0x00, 0x02, 0x00, 0x24, 0x9f, 0x00, 0x03, 0x00, 0x00, 0x00, 0x01, 0x04,
			0x00, 0x00, 0x00, 0x00, 0x05, 0x00, 0x00, 0x00, 0x00, 0x07, 0x00, 0x00, 0x00, 0x00, 0x08, 0x00,
			0x00, 0x00, 0x00, 0x09, 0x00, 0x00, 0x00, 0x00, 0x0a, 0x00, 0x00, 0x00, 0x00, 0x0b, 0x00, 0x00,
			0x00, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x00, 0x0d, 0x00, 0x00, 0x00, 0x00,
		}

		// we read all the bytes from the config
		assert.Equal(t, 60, n, "did not read all the bytes")
		assert.Equal(t, expected, b)
		wg.Done()
	}()

	err := binary.Read(sdr.Conn, binary.BigEndian, &sdr.Info)
	if !assert.Nil(t, err, "reading dongle info") {
		t.FailNow()
	}
	if !assert.True(t, sdr.Info.Valid(), "sdr is invalid") {
		t.FailNow()
	}

	// lets try configuring now
	sdr.Config = DefaultConfig()
	t.Logf("sdr config: %v", sdr.Config)
	assert.Nil(t, sdr.Configure())
	wg.Wait()
}

func Example_SDR() {
	var sdr SDR

	// Connect to address and defer close.
	sdr.Connect("127.0.0.1:1234", 0)
	defer sdr.Close()

	// Print dongle info.
	fmt.Printf("%+v\n", sdr.Info)
	// Example: {Magic:"RTL0" Tuner:R820T GainCount:29}

	// Create an array of bytes for samples.
	buf := make([]byte, 16384)

	// Read the entire array. This is usually done in a loop.
	_, err := io.ReadFull(sdr, buf)
	if err != nil {
		log.Fatal("Error reading samples:", err)
	}

	// Do something with data in buf...
}
