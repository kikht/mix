package sfml

import (
	"github.com/kikht/mix"
	"github.com/kikht/mix/gosfml2"
	"runtime"
	"sync/atomic"
	"time"
)

const (
	chunkPow   = 11
	chunkSize  = 1 << 11
	maxStreams = 1 << 10
	stateMask  = int64(chunkSize - 1)
	posMask    = ^int64(stateMask)
)

type Stream struct {
	state      *int64
	sampleRate mix.Tz
	handler    *gosfml2.SoundStream
	sources    [2]mix.Source
	buffer     []int16
	end        chan struct{}
}

var (
	stateArray [maxStreams]int64
	streams    []*Stream
)

func NewStream(numChannels int, sampleRate mix.Tz) (*Stream, error) {
	id := len(streams)
	state := &stateArray[id]
	handler, err := gosfml2.NewSoundStream(onStreamChunk, onStreamSeek,
		uint(numChannels), uint(sampleRate), id)
	if err != nil {
		return nil, err
	}
	res := Stream{
		state:      state,
		handler:    handler,
		sampleRate: sampleRate,
		buffer:     make([]int16, 2*chunkSize),
	}
	streams = append(streams, &res)
	*state = 0
	return &res, nil
}

func onStreamChunk(data interface{}) (proceed bool, samples []int16) {
	id := data.(int)
	state := atomic.AddInt64(&stateArray[id], chunkSize)

	stream := streams[id]
	src := stream.sources[state&1]

	posAfter := mix.Tz(state & posMask)
	pos := posAfter - chunkSize
	if pos >= src.Length() {
		return false, stream.buffer
	}

	var buf [2]mix.Buffer
	buf[0] = src.Samples(0, pos, chunkSize)
	buf[1] = src.Samples(1, pos, chunkSize)
	for i := 0; i < chunkSize; i++ {
		buffer[2*i] = norm(buf[0][i])
		buffer[2*i+1] = norm(buf[1][i])
	}
	return true, buffer
}

func onStreamSeek(time time.Duration, data interface{}) {
	id := data.(int)
	newPos := int64(mix.DurationToTz(time, streams[id].sampleRate)) & posMask
	state := &stateArray[id]
	for {
		orig := atomic.LoadInt64(state)
		upd := newPos | (orig & stateMask)
		if atomic.CompareAndSwapInt64(state, orig, upd) {
			break
		}
	}
}

func (s *Stream) Play(src mix.Source) (<-chan struct{}, error) {
	orig := atomic.LoadInt64(s.state)
	s.sources[(orig&1)^1] = src
	for !atomic.CompareAndSwapInt64(s.state, orig, orig^1) {
		runtime.Gosched()
		orig = atomic.LoadInt64(s.state)
	}
	if s.handler.GetStatus() != gosfml2.SoundStatusPlaying {
		s.end = make(chan struct{})
		go func() {
			defer close(s.end)
			s.handler.Play()
			for s.handler.GetStatus() == gosfml2.SoundStatusPlaying {
				time.Sleep(100 * time.Millisecond)
			}
		}()
	}
	return s.end, nil
}
