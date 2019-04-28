package session

import (
	"github.com/kikht/mix"

	"errors"
	"fmt"
	"log"
	"sort"
)

// Session mixes collection of Regions. Output is done in 32-bit float WAV.
// Session implements Source, so it could be nested.
type Session struct {
	sampleRate mix.Tz
	pos        mix.Tz
	length     mix.Tz
	forgetPast bool

	buffer [numChannels]mix.Buffer

	//TODO: separate regions collection & implement as tree
	regions []*preparedRegion
	rPos    int
	active  []*preparedRegion
}

// Region defines where and how Source audio (or its part) will be played.
type Region struct {
	Source          mix.Source // Audio to play.
	Begin           mix.Tz     // Time to begin playing in session samples.
	Offset, Length  mix.Tz     // Offset and Length in Source that will be played.
	Volume, Pan     float32    // Volume gain and stereo panning.
	FadeIn, FadeOut mix.Tz     // Length of fades.
}

const numChannels = 2

// NewSession creates Session with given sampleRate.
func NewSession(sampleRate mix.Tz, forgetPast bool) *Session {
	sess := &Session{
		sampleRate: sampleRate,
		forgetPast: forgetPast,
	}
	return sess
}

// Returns shallow copy of Session.
// Sources that are used in regions are not cloned.
func (s *Session) Clone() mix.Source {
	clone := *s
	clone.regions = make([]*preparedRegion, len(s.regions))
	copy(clone.regions, s.regions)
	clone.active = make([]*preparedRegion, len(s.active))
	copy(clone.active, s.active)
	return &clone
}

// AddRegion adds region to the Session mix.
func (s *Session) AddRegion(r Region) error {
	if r.Source.SampleRate() != s.sampleRate {
		return errors.New("Source sample rate is different from session")
	}
	if chans := r.Source.NumChannels(); chans < 1 || chans > 2 {
		return errors.New("Only mono and stereo sources are supported")
	}

	sLen := r.Source.Length()
	if r.Offset > sLen || r.Offset < 0 {
		return errors.New("Invalid offset")
	}
	if r.Length > sLen-r.Offset || r.Length < 0 {
		return errors.New("Invalid length")
	}
	if r.Length == 0 {
		r.Length = sLen - r.Offset
	}

	if r.FadeIn < 0 || r.FadeIn > r.Length {
		return errors.New("Invalid fadeIn")
	}
	if r.FadeOut < 0 || r.FadeOut > r.Length {
		return errors.New("Invalid fadeOut")
	}
	if r.FadeIn+r.FadeOut > r.Length {
		return errors.New("FadeIn + fadeOut > length")
	}

	end := r.Begin + r.Length
	if r.FadeIn > 0 {
		fi := preparedRegion{
			Src:    r.Source,
			Beg:    r.Begin,
			End:    r.Begin + r.FadeIn,
			Off:    r.Offset,
			VolBeg: 0,
			VolEnd: r.Volume,
			Pan:    r.Pan,
		}
		s.insertRegion(fi)
	}
	if r.Begin+r.FadeIn != r.Begin+r.Length-r.FadeOut {
		sr := preparedRegion{
			Src:    r.Source,
			Beg:    r.Begin + r.FadeIn,
			End:    end - r.FadeOut,
			Off:    r.Offset + r.FadeIn,
			VolBeg: r.Volume,
			VolEnd: r.Volume,
			Pan:    r.Pan,
		}
		s.insertRegion(sr)
	}
	if r.FadeOut > 0 {
		fo := preparedRegion{
			Src:    r.Source,
			Beg:    end - r.FadeOut,
			End:    end,
			Off:    r.Offset + r.Length - r.FadeOut,
			VolBeg: r.Volume,
			VolEnd: 0,
			Pan:    r.Pan,
		}
		s.insertRegion(fo)
	}

	if s.length < end {
		s.length = end
	}

	return nil
}

func (s *Session) insertRegion(r preparedRegion) {
	rLen := len(s.regions)
	rPos := sort.Search(rLen, func(i int) bool {
		return s.regions[i].Beg > r.Beg
	})
	//log.Println("insert: regions=", s.regions, "r=", r, "rPos=", rPos)
	if rPos < s.rPos || (rPos == s.rPos && r.Beg < s.pos) {
		s.rPos++
	}
	s.regions = append(s.regions, &r)
	if rPos < rLen {
		copy(s.regions[rPos+1:], s.regions[rPos:])
		s.regions[rPos] = &r
	}

	if s.pos > r.Beg && s.pos < r.End {
		//log.Println("insert: New active region", r)
		s.active = append(s.active, &r)
	}
}

func (s *Session) mix(buffer [2]mix.Buffer) {
	length := mix.Tz(len(buffer[0]))
	if length == 0 {
		return
	}
	end := s.pos + length

	// Add new active regions
	for ; s.rPos < len(s.regions); s.rPos++ {
		r := s.regions[s.rPos]
		if r.Beg < end {
			//log.Println("mix: New active region", r)
			s.active = append(s.active, r)
			if s.forgetPast {
				s.regions[s.rPos] = nil
			}
		} else {
			break
		}
	}
	if s.forgetPast {
		if s.rPos > 0 {
			//log.Printf("Session: %p GC regions: %d active %d\n", s, s.rPos, len(s.regions))
		}
		s.regions = s.regions[s.rPos:]
		s.rPos = 0
	}

	// Mix active regions and filter completed
	lastActive := 0
	for _, r := range s.active {
		var rOff, bOff mix.Tz
		if r.Beg < s.pos {
			rOff = s.pos - r.Beg
		} else {
			bOff = r.Beg - s.pos
		}

		rEnd := r.End
		bEnd := end
		if r.End < end {
			bEnd = rEnd
		} else {
			rEnd = bEnd
		}
		rLen := rEnd - r.Beg - rOff
		bEnd -= s.pos

		if end < r.End {
			s.active[lastActive] = r
			lastActive++
		}

		if r.Src == nil {
			continue
		}

		var gain [numChannels][numChannels]float32
		schan := r.Src.NumChannels()
		switch schan {
		case 1:
			gain[0][0], gain[0][1] = mix.PanMonoGain(r.Pan)
		case 2:
			gain[0][0], gain[0][1], gain[1][0], gain[1][1] = mix.PanStereoGain(r.Pan)
		default:
			panic("Invalid number of channels")
		}

		//log.Printf("Mixing region %v, pos=%v end=%v rOff=%v bOff=%v rEnd=%v bEnd=%v rLen=%v gain=%v\n", r, s.pos, end, rOff, bOff, rEnd, bEnd, rLen, gain)

		for i := 0; i < schan; i++ {
			src := r.Src.Samples(i, r.Off+rOff, rLen)
			init, targ := r.VolBeg, r.VolEnd

			if init != targ {
				initsqr := init * init
				coef := (targ*targ - initsqr) / float32(r.End-r.Beg)
				init = initsqr + coef*float32(rOff)
				targ = initsqr + coef*float32(rOff+rLen)
			}

			for j := 0; j < numChannels; j++ {
				dst := buffer[j][bOff:bEnd]
				assert(len(src) == len(dst))
				if init == targ {
					g := init * gain[i][j]
					switch {
					case g == 1:
						dst.Mix(src)
					case g < 1e-8:
						//do nothing
					default:
						dst.MixGain(src, g)
					}
				} else {
					g := gain[i][j] * gain[i][j]
					dst.MixSqrtRamp(src, g*init, g*targ)
				}
			}
		}
	}
	s.active = s.active[0:lastActive]
	s.pos += length
}

// Length returns end of last region in Session
func (s *Session) Length() mix.Tz {
	return s.length
}

// NumChannels returns number of channels in Session
func (s *Session) NumChannels() int {
	return numChannels
}

func (s *Session) Samples(channel int, offset, length mix.Tz) mix.Buffer {
	// Fast-path for already mixed data
	if offset+length == s.pos &&
		len(s.buffer) > channel &&
		mix.Tz(len(s.buffer[channel])) == length {

		return s.buffer[channel]
	}

	s.SetPosition(offset)
	buf := s.allocateBuffer(length)
	s.mix(buf)
	return buf[channel]
}

// SetPosition sets current Session position.
func (s *Session) SetPosition(pos mix.Tz) {
	if s.pos == pos {
		return
	}
	if pos < s.pos && s.forgetPast {
		log.Fatalf("Rewind of forgetful session %d < %d", pos, s.pos)
	}
	s.pos = pos

	// Shrink buffer for fast path in Samples()
	for c := range s.buffer {
		s.buffer[c] = s.buffer[c][0:0]
	}

	//Simple linear algorithm, interval tree will do it in log(n)
	s.active = s.active[0:0]
	for s.rPos = 0; s.rPos < len(s.regions); s.rPos++ {
		r := s.regions[s.rPos]
		if pos <= r.Beg {
			break
		}
		if pos < r.End {
			s.active = append(s.active, r)
			//log.Println("SetPosition: New active region", r)
		}
	}
}

// Position returns current Session position.
func (s *Session) Position() mix.Tz {
	return s.pos
}

// SampleRate returns sample rate of Session.
func (s *Session) SampleRate() mix.Tz {
	return s.sampleRate
}

func (s *Session) allocateBuffer(length mix.Tz) [2]mix.Buffer {
	for i := 0; i < numChannels; i++ {
		if mix.Tz(cap(s.buffer[i])) >= length {
			s.buffer[i] = s.buffer[i][0:length]
			s.buffer[i].Zero()
		} else {
			s.buffer[i] = mix.NewBuffer(length)
		}
	}
	return s.buffer
}

// Immutable region info with precomputed values
type preparedRegion struct {
	Src                 mix.Source
	Beg, End, Off       mix.Tz
	VolBeg, VolEnd, Pan float32
}

func (r preparedRegion) String() string {
	return fmt.Sprintf("{Beg=%v End=%v Off=%v Vol=%4.2f:%4.2f Pan=%+5.2f}",
		r.Beg, r.End, r.Off, r.VolBeg, r.VolEnd, r.Pan)
}

func assert(b bool) {
	if !b {
		panic("assert failed")
	}
}
