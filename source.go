package mix

import (
	"time"
)

// Source is the interface that represents audio data.
type Source interface {
	// Samples may return internal buffer.
	// You must Copy first, if you plan to modify it.
	Samples(channel int, offset, length Tz) Buffer
	SampleRate() Tz
	NumChannels() int
	Length() Tz
}

// Duration returns Length() of Source as time.Duration according to its SampleRate()
func SourceDuration(s Source) time.Duration {
	return time.Duration(s.Length() * Tz(time.Second) / s.SampleRate())
}

// DurationToTz converts time.Duration to number of samples with Source sample rate.
func DurationToTz(s Source, d time.Duration) Tz {
	return Tz(d * time.Duration(s.SampleRate()) / time.Second)
}

// MemSource is a Source that holds all necessary data in memory.
type MemSource struct {
	Data []Buffer
	Rate Tz
}

// Samples returns Buffer holding length samples from channel starting at offset.
// Will return internal buffer. Copy it, if you want to modify.
func (s MemSource) Samples(channel int, offset, length Tz) Buffer {
	return s.Data[channel][offset : offset+length]
}

// SampleRate returns number of samples per second in MemSource.
func (s MemSource) SampleRate() Tz {
	return s.Rate
}

// NumChannels returns number of channels in MemSource.
func (s MemSource) NumChannels() int {
	return len(s.Data)
}

// Length returns number of samples in MemSource.
func (s MemSource) Length() Tz {
	if len(s.Data) == 0 {
		return 0
	}
	return Tz(len(s.Data[0]))
}
