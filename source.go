package mix

import (
	"errors"
	sox "github.com/krig/go-sox"
	"math"
)

type Source interface {
	Samples(channel int, offset, length Tz) Buffer
	SampleRate() Tz
	NumChannels() int
	Length() Tz
}

// Load audio file into memory using libsox.
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

func soxSample(s sox.Sample) float32 {
	const coef = 1.0 / (math.MaxInt32 + 1)
	return float32(s) * coef
}

// Source that holds all necessary data in memory
type MemSource struct {
	Data []Buffer
	Rate Tz
}

func (s MemSource) Samples(channel int, offset, length Tz) Buffer {
	return s.Data[channel][offset : offset+length]
}

func (s MemSource) SampleRate() Tz {
	return s.Rate
}

func (s MemSource) NumChannels() int {
	return len(s.Data)
}

func (s MemSource) Length() Tz {
	if len(s.Data) == 0 {
		return 0
	}
	return Tz(len(s.Data[0]))
}
