// Package mix implements golang audio sequencer.
//
// Demo:
//	go run examples/main.go | aplay
package mix

import "time"

// Tz represents time in number of samples
type Tz int64

// DurationToTz converts time.Duration to number of samples for specified sample rate.
func DurationToTz(d time.Duration, sampleRate Tz) Tz {
	return Tz(d * time.Duration(sampleRate) / time.Second)
}
