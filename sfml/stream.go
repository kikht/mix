package sfml

/*
#cgo LDFLAGS: -lcsfml-audio -lcsfml-system
#include <SFML/Audio/SoundStream.h>
sfSoundStream* cgo_createStream(unsigned int channelCount,
                                unsigned int sampleRate,
                                void* obj);
*/
import "C"

import (
	"github.com/kikht/mix"

	"github.com/rkusa/gm/math32"

	"errors"
	"log"
	"math"
	"sync/atomic"
	"unsafe"
)

const (
	chunkSize   = 1 << 11
	maxStreams  = 1 << 7
	posMask     = ^uint64(chunkSize - 1)
	srcBit      = 1
	activeBit   = 2
	numChannels = 2
)

func init() {
	log.Printf("cgo_sfml.stream init chunkSize=%d maxStreams=%d posMask=%x\n",
		chunkSize, maxStreams, posMask)
}

type Stream struct {
	state      *uint64
	sampleRate mix.Tz
	handle     *C.sfSoundStream
	sources    [2]mix.Source
	buffer     []int16
	end        chan struct{}
}

var (
	stateArray [maxStreams]uint64
	streams    []*Stream
)

func NewStream(sampleRate mix.Tz) (*Stream, error) {
	id := len(streams)
	stream := &Stream{
		state:      &stateArray[id],
		sampleRate: sampleRate,
		buffer:     make([]int16, numChannels*chunkSize),
	}
	stream.handle = C.cgo_createStream(C.uint(numChannels), C.uint(sampleRate),
		unsafe.Pointer(stream.state))
	if stream.handle == nil {
		return nil, errors.New("Can not create sfml sound stream")
	}
	streams = append(streams, stream)
	*stream.state = 0
	//TODO: add destructor to finalize
	return stream, nil
}

//export onStreamChunk
func onStreamChunk(chunk *C.sfSoundStreamChunk, ptr unsafe.Pointer) C.sfBool {
	id := uintptr(ptr) - uintptr(unsafe.Pointer(&stateArray[0]))
	statePtr := (*uint64)(ptr)
	state := atomic.AddUint64(statePtr, chunkSize)
	stream := streams[id]

	chunk.samples = (*C.sfInt16)(unsafe.Pointer(&stream.buffer[0]))
	chunk.sampleCount = C.uint(numChannels * chunkSize)

	posAfter := mix.Tz(state & posMask)
	pos := posAfter - chunkSize
	src := stream.sources[state&srcBit]
	if pos >= src.Length() {
		log.Println("End of stream", statePtr, pos)
		atomic.StoreUint64(statePtr, state&srcBit)
		defer close(stream.end)
		return C.sfFalse
	}

	buf := [2]mix.Buffer{
		src.Samples(0, pos, chunkSize),
		src.Samples(1, pos, chunkSize)}
	for i := 0; i < chunkSize; i++ {
		stream.buffer[2*i] = norm(buf[0][i])
		stream.buffer[2*i+1] = norm(buf[1][i])
	}
	return C.sfTrue
}

//export onStreamSeek
func onStreamSeek(time C.sfTime, ptr unsafe.Pointer) {
	//id := data.(int)
	//log.Println("Stream seek")
	//newPos := int64(mix.DurationToTz(time, streams[id].sampleRate)) & posMask
	//state := &stateArray[id]
	//for {
	//	orig := atomic.LoadUint64(state)
	//	upd := newPos | (orig & stateMask)
	//	if atomic.CompareAndSwapUint64(state, orig, upd) {
	//		break
	//	}
	//}
}

func (s *Stream) End() <-chan struct{} {
	return s.end
}

func (s *Stream) Play(src mix.Source) {
	orig := atomic.LoadUint64(s.state)
	//src bit must be changed only by controller thread
	s.sources[(orig&srcBit)^srcBit] = src
	for {
		log.Println("Stream.Play() iteration", s.state)
		upd := (orig ^ srcBit) | activeBit
		if atomic.CompareAndSwapUint64(s.state, orig, upd) {
			break
		}
		orig = atomic.LoadUint64(s.state)
	}
	if (orig & activeBit) == 0 {
		log.Println("Start playing", s.state)
		s.end = make(chan struct{})
		C.sfSoundStream_play(s.handle)
	}
}

func (s *Stream) Switch(generator mix.SourceMutator) {
	var orig uint64
	for {
		log.Println("Stream.Switch() iteration", s.state)
		orig = atomic.LoadUint64(s.state)
		cur := s.sources[orig&srcBit]
		pos := mix.Tz(orig & posMask)
		s.sources[(orig&srcBit)^srcBit] = generator.Mutate(cur, pos)
		upd := (orig ^ srcBit) | activeBit
		if atomic.CompareAndSwapUint64(s.state, orig, upd) {
			break
		}
	}
	if (orig & activeBit) == 0 {
		log.Println("Start playing", s.state)
		s.end = make(chan struct{})
		C.sfSoundStream_play(s.handle)
	}
}

func (s *Stream) ChunkSize() mix.Tz {
	return chunkSize
}

func (s *Stream) State() (mix.Source, mix.Tz) {
	state := atomic.LoadUint64(s.state)
	return s.sources[state&srcBit], mix.Tz(state & posMask)
}

func (s *Stream) SampleRate() mix.Tz {
	return s.sampleRate
}

// simple limiter
func norm(v float32) int16 {
	return int16(math.MaxInt16 * v / (1 + math32.Abs(v)))
}
