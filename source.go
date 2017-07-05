package mix

import (
	"errors"
	sox "github.com/krig/go-sox"
	"math"
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

// LoadSOX loads audio file using libsox.
// Current implementation loads all data into memory.
func LoadSOX(path string) (Source, error) {
	file := sox.OpenRead(path)
	if file == nil {
		return nil, errors.New("Sox can't open file: " + path)
	}
	defer file.Release()

	info := file.Signal()
	channels := int(info.Channels())
	res := MemSource{
		Rate: Tz(info.Rate()),
		Data: make([]Buffer, channels),
	}
	length := info.Length()
	if length > 0 {
		for i := 0; i < channels; i++ {
			res.Data[i] = NewBuffer(Tz(length))[0:0]
		}
	}

	const ChunkSize = 2048
	buffer := make([]sox.Sample, ChunkSize*channels)
	for {
		size := file.Read(buffer, uint(len(buffer)))
		if size == 0 || size == sox.EOF {
			break
		}

		for off := 0; off < int(size); off += channels {
			for c := 0; c < channels; c++ {
				res.Data[c] = append(res.Data[c], soxSample(buffer[off+c]))
			}
		}
	}
	return res, nil
}

// Duration returns Length() of Source as time.Duration according to its SampleRate()
func SourceDuration(s Source) time.Duration {
	return time.Duration(s.Length() * Tz(time.Second) / s.SampleRate())
}

func soxSample(s sox.Sample) float32 {
	const coef = 1.0 / (math.MaxInt32 + 1)
	return float32(s) * coef
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
