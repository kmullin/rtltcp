// This package provides a wrapper for the TCP protocol implemented by the rtl_tcp tool used with Realtek DVB-T based SDR's.
package rtltcp

import (
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/bemasher/rtltcp/si"
)

var dongleMagic = [...]byte{'R', 'T', 'L', '0'}

const DefaultAddress = "127.0.0.1:1234"

// Contains dongle information and an embedded tcp connection to the spectrum server
type SDR struct {
	net.Conn
	Config Config
	Info   DongleInfo
}

// Give an address of the form "<hostname or IP>:<port>", connects to the spectrum
// server at the given address or returns an error. The user is responsible
// for closing this connection. If address is an empty string, use "127.0.0.1:1234"
func (sdr *SDR) Connect(address string, timeout time.Duration) (err error) {
	if address == "" {
		address = DefaultAddress
	}
	sdr.Conn, err = net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return fmt.Errorf("Error connecting to spectrum server: %w", err)
	}

	// If we exit this function due to an error, close the connection.
	defer func() {
		if err != nil {
			sdr.Close()
		}
	}()

	err = binary.Read(sdr.Conn, binary.BigEndian, &sdr.Info)
	if err != nil {
		err = fmt.Errorf("Error getting dongle information: %w", err)
		return
	}

	if !sdr.Info.Valid() {
		err = fmt.Errorf("Invalid magic number: expected %q received %q", dongleMagic, sdr.Info.Magic)
	}

	return
}

type Config struct {
	CenterFreq     si.ScientificNotation // center frequency to receive on
	SampleRate     si.ScientificNotation // sample rate
	TunerGainMode  bool                  // enable/disable tuner gain
	TunerGain      float64               // set tuner gain in dB
	FreqCorrection int                   // frequency correction in PPM
	TestMode       bool                  // enable/disable test mode
	AgcMode        bool                  // enable/disable rtl agc
	DirectSampling bool                  // enable/disable direct sampling
	OffsetTuning   bool                  // enable/disable offset tuning
	RtlXtalFreq    uint                  // set rtl xtal frequency
	TunerXtalFreq  uint                  // set tuner xtal frequency
	GainByIndex    uint                  // set gain by index
}

// DefaultConfig returns a Config object with defaults to be used with SDR
func DefaultConfig() (config Config) {
	config.CenterFreq.Set("100M")
	config.SampleRate.Set("2.4M")
	return
}

func (sdr SDR) Configure() (err error) {
	fields := reflect.VisibleFields(reflect.TypeOf(sdr.Config))
	for _, field := range fields {
		switch field.Name {
		case "CenterFreq":
			err = sdr.SetCenterFreq(uint32(sdr.Config.CenterFreq))
		case "SampleRate":
			err = sdr.SetSampleRate(uint32(sdr.Config.SampleRate))
		case "TunerGainMode":
			err = sdr.SetGainMode(sdr.Config.TunerGainMode)
		case "TunerGain":
			err = sdr.SetGain(uint32(sdr.Config.TunerGain * 10.0))
		case "FreqCorrection":
			err = sdr.SetFreqCorrection(uint32(sdr.Config.FreqCorrection))
		case "TestMode":
			err = sdr.SetTestMode(sdr.Config.TestMode)
		case "AgcMode":
			err = sdr.SetAGCMode(sdr.Config.AgcMode)
		case "DirectSampling":
			err = sdr.SetDirectSampling(sdr.Config.DirectSampling)
		case "OffsetTuning":
			err = sdr.SetOffsetTuning(sdr.Config.OffsetTuning)
		case "RtlXtalFreq":
			err = sdr.SetRTLXtalFreq(uint32(sdr.Config.RtlXtalFreq))
		case "TunerXtalFreq":
			err = sdr.SetTunerXtalFreq(uint32(sdr.Config.TunerXtalFreq))
		case "GainByIndex":
			err = sdr.SetGainByIndex(uint32(sdr.Config.GainByIndex))
		default:
			err = fmt.Errorf("unknown configuration field: %v", field.Name)
		}

		if err != nil {
			return fmt.Errorf("err configuring sdr: %w", err)
		}
	}
	return nil
}

// Contains the Magic number, tuner information and the number of valid gain values.
type DongleInfo struct {
	Magic     [4]byte
	Tuner     Tuner
	GainCount uint32 // Useful for setting gain by index
}

func (d DongleInfo) String() string {
	return fmt.Sprintf("{Magic:%q Tuner:%s GainCount:%d}", d.Magic, d.Tuner, d.GainCount)
}

// Checks that the magic number received matches the expected byte string 'RTL0'.
func (d DongleInfo) Valid() bool {
	return d.Magic == dongleMagic
}

// Provides mapping of tuner value to tuner string.
type Tuner uint32

func (t Tuner) String() string {
	switch t {
	case 1:
		return "E4000"
	case 2:
		return "FC0012"
	case 3:
		return "FC0013"
	case 4:
		return "FC2580"
	case 5:
		return "R820T"
	case 6:
		return "R828D"
	}
	return "UNKNOWN"
}

func (sdr SDR) execute(cmd command) (err error) {
	return binary.Write(sdr.Conn, binary.BigEndian, cmd)
}

type command struct {
	command   uint8
	Parameter uint32
}

// Command constants defined in rtl_tcp.c
const (
	centerFreq = iota + 1
	sampleRate
	tunerGainMode
	tunerGain
	freqCorrection
	tunerIfGain
	testMode
	agcMode
	directSampling
	offsetTuning
	rtlXtalFreq
	tunerXtalFreq
	gainByIndex
)

// Set the center frequency in Hz.
func (sdr SDR) SetCenterFreq(freq uint32) (err error) {
	return sdr.execute(command{centerFreq, freq})
}

// Set the sample rate in Hz.
func (sdr SDR) SetSampleRate(rate uint32) (err error) {
	return sdr.execute(command{sampleRate, rate})
}

// Set gain in tenths of dB. (197 => 19.7dB)
func (sdr SDR) SetGain(gain uint32) (err error) {
	return sdr.execute(command{tunerGain, gain})
}

// Set the Tuner AGC, true to enable.
func (sdr SDR) SetGainMode(state bool) (err error) {
	if state {
		return sdr.execute(command{tunerGainMode, 0})
	}
	return sdr.execute(command{tunerGainMode, 1})
}

// Set gain by index, must be <= DongleInfo.GainCount
func (sdr SDR) SetGainByIndex(idx uint32) (err error) {
	if idx > sdr.Info.GainCount {
		return fmt.Errorf("invalid gain index: %d", idx)
	}
	return sdr.execute(command{gainByIndex, idx})
}

// Set frequency correction in ppm.
func (sdr SDR) SetFreqCorrection(ppm uint32) (err error) {
	return sdr.execute(command{freqCorrection, ppm})
}

// Set tuner intermediate frequency stage and gain.
func (sdr SDR) SetTunerIfGain(stage, gain uint16) (err error) {
	return sdr.execute(command{tunerIfGain, (uint32(stage) << 16) | uint32(gain)})
}

// Set test mode, true for enabled.
func (sdr SDR) SetTestMode(state bool) (err error) {
	if state {
		return sdr.execute(command{testMode, 1})
	}
	return sdr.execute(command{testMode, 0})
}

// Set RTL AGC mode, true for enabled.
func (sdr SDR) SetAGCMode(state bool) (err error) {
	if state {
		return sdr.execute(command{agcMode, 1})
	}
	return sdr.execute(command{agcMode, 0})
}

// Set direct sampling mode.
func (sdr SDR) SetDirectSampling(state bool) (err error) {
	if state {
		return sdr.execute(command{directSampling, 1})
	}
	return sdr.execute(command{directSampling, 0})
}

// Set offset tuning, true for enabled.
func (sdr SDR) SetOffsetTuning(state bool) (err error) {
	if state {
		return sdr.execute(command{offsetTuning, 1})
	}
	return sdr.execute(command{offsetTuning, 0})
}

// Set RTL xtal frequency.
func (sdr SDR) SetRTLXtalFreq(freq uint32) (err error) {
	return sdr.execute(command{rtlXtalFreq, freq})
}

// Set tuner xtal frequency.
func (sdr SDR) SetTunerXtalFreq(freq uint32) (err error) {
	return sdr.execute(command{tunerXtalFreq, freq})
}
