package sox

import (
	"errors"
	"github.com/kikht/mix"
	sox "github.com/krig/go-sox"
	"math"
)

// LoadSOX loads audio file using libsox.
// Current implementation loads all data into memory.
func Load(path string) (mix.Source, error) {
	file := sox.OpenRead(path)
	if file == nil {
		return nil, errors.New("Sox can't open file: " + path)
	}
	defer file.Release()

	info := file.Signal()
	channels := int(info.Channels())
	res := mix.MemSource{
		Rate: mix.Tz(info.Rate()),
		Data: make([]mix.Buffer, channels),
	}
	length := info.Length()
	if length > 0 {
		for i := 0; i < channels; i++ {
			res.Data[i] = mix.NewBuffer(mix.Tz(length))[0:0]
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
