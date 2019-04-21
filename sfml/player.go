package sfml

import (
	"errors"
	"github.com/kikht/mix"
	"github.com/kikht/mix/gosfml2"
	"github.com/rkusa/gm/math32"
	"math"
	"sync/atomic"
	"time"
)

const chunkSize = 2048

var (
	source   atomic.Value
	position int64
	buffer   []int16 = make([]int16, 2*chunkSize)
	end      chan struct{}
)

func Play(src mix.Source) (<-chan struct{}, error) {
	if src.NumChannels() != 2 {
		return nil, errors.New("only stereo sources are supported")
	}
	current := source.Load()
	if current == nil {
		stream, err := gosfml2.NewSoundStream(onGetData, onSeek,
			uint(src.NumChannels()), uint(src.SampleRate()), nil)
		if err != nil {
			return nil, err
		}
		source.Store(src.Clone())
		end = make(chan struct{})
		go func() {
			defer close(end)
			stream.Play()
			for stream.GetStatus() == gosfml2.SoundStatusPlaying {
				time.Sleep(100 * time.Millisecond)
			}
		}()
	} else {
		if current.(mix.Source).SampleRate() != src.SampleRate() {
			return nil, errors.New("sources have different sample rate")
		}
		source.Store(src.Clone())
	}
	return end, nil
}

// simple limiter
func norm(v float32) int16 {
	return int16(math.MaxInt16 * v / (1 + math32.Abs(v)))
}

func onGetData(data interface{}) (proceed bool, samples []int16) {
	src := source.Load().(mix.Source)
	var buf [2]mix.Buffer
	posAfter := atomic.AddInt64(&position, chunkSize)
	pos := mix.Tz(posAfter - chunkSize)
	if pos >= src.Length() {
		return false, buffer
	}
	buf[0] = src.Samples(0, pos, chunkSize)
	buf[1] = src.Samples(1, pos, chunkSize)
	for i := 0; i < chunkSize; i++ {
		buffer[2*i] = norm(buf[0][i])
		buffer[2*i+1] = norm(buf[1][i])
	}
	return true, buffer
}

func onSeek(time time.Duration, data interface{}) {
	src := source.Load().(mix.Source)
	atomic.StoreInt64(&position, int64(mix.DurationToTz(time, src.SampleRate())))
}
