package jack

import (
	"github.com/kikht/mix"

	"github.com/rkusa/gm/math32"
	"github.com/xthexder/go-jack"

	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
)

var (
	portCount   = 0
	client      *jack.Client
	streams     atomic.Value
	reaper      = make(chan *Stream, 10)
	outputPorts []string
)

type Stream struct {
	state   uint64
	sources [2]mix.Source
	ports   []*jack.Port
	end     chan struct{}
}

func init() {
	log.Println("jack.init()")
	var status int
	client, status = jack.ClientOpen("gomix", jack.NullOption|jack.NoStartServer)
	if status != 0 {
		log.Println("connecting client: ", jack.StrError(status))
		client = nil
		return
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGABRT,
		syscall.SIGQUIT, syscall.SIGHUP)
	go func() {
		<-sigs
		log.Println("Closing jack connection")
		client.Close()
	}()

	status = client.SetProcessCallback(process)
	if status != 0 {
		log.Println("set callback: ", jack.StrError(status))
		client.Close()
		client = nil
		return
	}
	outputPorts = client.GetPorts("", "", jack.PortIsPhysical|jack.PortIsInput)
	if outputPorts == nil {
		log.Println("Can not find output ports")
		client.Close()
		client = nil
		return
	}
	log.Println("jack output ports", outputPorts)
	streams.Store([]*Stream{})

	log.Println("jack.Play()")
	status = client.Activate()
	if status != 0 {
		client.Close()
		client = nil
		log.Println("activate jack client:", jack.StrError(status))
	}
}

const (
	stateBits = 1
	srcBit    = 1
)

func process(nframes uint32) int {
	chunkSize := mix.Tz(nframes)
	streamList := streams.Load().([]*Stream)
	for _, stream := range streamList {
		if stream == nil {
			continue
		}
		state := atomic.AddUint64(&stream.state, uint64(chunkSize<<stateBits))
		posAfter := mix.Tz(state >> stateBits)
		pos := posAfter - chunkSize
		src := stream.sources[state&srcBit]

		if src == nil {
			continue
		}

		end := src.Length()
		if pos < end && posAfter >= end {
			select {
			case stream.end <- struct{}{}:
			default:
			}
			continue
		}

		for c, port := range stream.ports {
			//TODO: get rid of copy, mix directly to buffer
			dstBuf := port.GetBuffer(nframes)
			srcBuf := src.Samples(c, pos, chunkSize)
			for i, v := range srcBuf {
				dstBuf[i] = jack.AudioSample(v / (1 + math32.Abs(v)))
			}
		}
	}
	return 0
}

func NewStream(numChannels int) (*Stream, error) {
	log.Println("jack.NewStream()")
	if client == nil {
		return nil, errors.New("Can not create stream without jack connection")
	}
	stream := &Stream{
		ports: make([]*jack.Port, numChannels),
		end:   make(chan struct{}, 1),
	}
	for i := range stream.ports {
		portCount++
		stream.ports[i] = client.PortRegister(fmt.Sprintf("out_%d", portCount),
			jack.DEFAULT_AUDIO_TYPE, jack.PortIsOutput, 0)
		if stream.ports[i] == nil {
			return nil, errors.New("Can not register jack port")
		}
		outName := outputPorts[i%len(outputPorts)]
		status := client.Connect(stream.ports[i].GetName(), outName)
		if status != 0 {
			return nil, fmt.Errorf("Can not connect %d to %s: %s", i,
				outName, jack.StrError(status))
		}
	}
	oldStreams := streams.Load().([]*Stream)
	newStreams := make([]*Stream, len(oldStreams)+1)
	copy(newStreams, oldStreams)
	newStreams[len(newStreams)-1] = stream
	streams.Store(newStreams)
	return stream, nil
}

func (s *Stream) Play(src mix.Source) {
	orig := atomic.LoadUint64(&s.state)
	//src bit must be changed only by controller thread
	s.sources[(orig&srcBit)^srcBit] = src
	for {
		log.Println("jack.Stream.Play() iteration", s.state)
		upd := orig ^ srcBit
		if atomic.CompareAndSwapUint64(&s.state, orig, upd) {
			break
		}
		orig = atomic.LoadUint64(&s.state)
	}
}

func (s *Stream) Switch(generator mix.SourceMutator) {
	var orig uint64
	for {
		log.Println("jack.Stream.Switch() iteration", s.state)
		orig = atomic.LoadUint64(&s.state)
		cur := s.sources[orig&srcBit]
		pos := mix.Tz(orig >> stateBits)
		s.sources[(orig&srcBit)^srcBit] = generator.Mutate(cur, pos)
		upd := orig ^ srcBit
		if atomic.CompareAndSwapUint64(&s.state, orig, upd) {
			break
		}
	}
}

func (s *Stream) End() <-chan struct{} {
	return s.end
}

func (s *Stream) SampleRate() mix.Tz {
	return mix.Tz(client.GetSampleRate())
}

func (s *Stream) ChunkSize() mix.Tz {
	return mix.Tz(client.GetBufferSize())
}

//TODO: stream destroy
