package mix

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
	"math"
	"os"
	"sort"
	"syscall"
	"time"
)

// Number of samples
type Tz int64

type Session struct {
	sampleRate Tz
	pos        Tz
	output     io.Writer
	numOut     Tz

	buffer  [numChannels]Buffer
	regions []*preparedRegion
	rPos    int
	active  []*preparedRegion
	//TODO:
	//out sample type
	//data format
}

const (
	fmtSize       = 16
	bitsPerSample = 32
	numChannels   = 2
	sampleFormat  = 3 //for float, 1 for PCM
	blockAlign    = numChannels * bitsPerSample / 8

	dataSizeOff    = 40
	riffSizeOff    = 4
	riffHeaderSize = 36
)

func NewSession(sampleRate Tz) *Session {
	sess := &Session{
		sampleRate: sampleRate,
	}
	sess.SetOutput(ioutil.Discard)
	return sess
}

// Region defines where and how Source audio (or its part) will be played.
type Region struct {
	Source          Source  // Audio to play.
	Begin           Tz      // Time to begin playing in session samples.
	Offset, Length  Tz      // Offset and Length in Source that will be played.
	Volume, Pan     float32 // Volume gain and stereo panning.
	FadeIn, FadeOut Tz      // Length of fades.
}

// Add region to the mix.
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
	sr := preparedRegion{
		Src:    r.Source,
		Beg:    r.Begin + r.FadeIn,
		End:    r.Begin + r.Length - r.FadeOut,
		Off:    r.Offset + r.FadeIn,
		VolBeg: r.Volume,
		VolEnd: r.Volume,
		Pan:    r.Pan,
	}
	s.insertRegion(sr)
	if r.FadeOut > 0 {
		fo := preparedRegion{
			Src:    r.Source,
			Beg:    r.Begin + r.Length - r.FadeOut,
			End:    r.Begin + r.Length,
			Off:    r.Offset + r.Length - r.FadeOut,
			VolBeg: r.Volume,
			VolEnd: 0,
			Pan:    r.Pan,
		}
		s.insertRegion(fo)
	}

	return nil
}

func (s *Session) insertRegion(r preparedRegion) {
	rLen := len(s.regions)
	rPos := sort.Search(rLen, func(i int) bool {
		return s.regions[i].Beg > r.Beg
	})
	if rPos < s.rPos {
		s.rPos++
	}
	s.regions = append(s.regions, &r)
	if rPos < rLen {
		copy(s.regions[rPos+1:], s.regions[rPos:])
		s.regions[rPos] = &r
	}

	if s.pos > r.Beg && s.pos < r.End {
		s.active = append(s.active, &r)
	}

}

// Mix length samples and write them to output.
func (s *Session) Play(length Tz) error {
	if s.numOut == 0 {
		_, err := s.output.Write(s.wavHeader(-1))
		if err != nil {
			return err
		}
	}

	s.allocateBuffer(length)
	s.mix(length)
	s.pos += length
	s.numOut += length

	err := s.writeBuffer()
	if err != nil {
		return err
	}
	err = s.updateHeader()
	return err
}

func (s *Session) mix(length Tz) {
	end := s.pos + length

	// Add new active regions
	for ; s.rPos < len(s.regions); s.rPos++ {
		r := s.regions[s.rPos]
		if r.Beg < end {
			s.active = append(s.active, r)
		} else {
			break
		}
	}

	// Mix active regions and filter completed
	lastActive := 0
	for _, r := range s.active {
		var rOff, bOff Tz
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

		var gain [numChannels][numChannels]float32
		schan := r.Src.NumChannels()
		switch schan {
		case 1:
			gain[0][0], gain[0][1] = panMonoGain(r.Pan)
		case 2:
			gain[0][0], gain[0][1], gain[1][0], gain[1][1] = panStereoGain(r.Pan)
		default:
			panic("Invalid number of channels")
		}

		for i := 0; i < schan; i++ {
			src := r.Src.Samples(i, r.Off+rOff, rLen)
			var vol float32

			if r.VolBeg == r.VolEnd { //simple region fast path
				vol = r.VolBeg
			} else {
				vol = 1
				coef := (r.VolEnd*r.VolEnd - r.VolBeg*r.VolBeg) /
					float32(r.End-r.Beg)
				init := r.VolBeg*r.VolBeg + coef*float32(rOff)
				targ := r.VolBeg*r.VolBeg + coef*float32(rOff+rLen)
				src = src.Clone()
				src.SqrtRamp(init, targ)
			}

			for j := 0; j < numChannels; j++ {
				dst := s.buffer[j][bOff:bEnd]
				assert(len(src) == len(dst))
				dst.MixGain(src, gain[i][j]*vol)
			}
		}

		if end > r.End {
			s.active[lastActive] = r
			lastActive++
		}
	}
	s.active = s.active[0:lastActive]
}

// Convert time.Duration to number of samples with Session sample rate.
func (s *Session) DurationToTz(d time.Duration) Tz {
	return Tz(d * time.Duration(s.sampleRate) / time.Second)
}

// Set playback position.
func (s *Session) SetPosition(pos Tz) {
	s.pos = pos

	//Simple linear algorithm, interval tree will do it in log(n)
	s.active = s.active[0:0]
	var r *preparedRegion
	for s.rPos, r = range s.regions {
		if pos <= r.Beg {
			break
		}
		if pos < r.End {
			s.active = append(s.active, r)
		}
	}
}

// Get current playback position.
func (s *Session) Position() Tz {
	return s.pos
}

// Get session sample rate.
func (s *Session) SampleRate() Tz {
	return s.sampleRate
}

// Set io.Writer to output mixed data.
func (s *Session) SetOutput(output io.Writer) {
	s.output = output
	s.numOut = 0
}

func (s *Session) allocateBuffer(length Tz) {
	for i := 0; i < numChannels; i++ {
		if Tz(cap(s.buffer[i])) >= length {
			s.buffer[i] = s.buffer[i][0:length]
			s.buffer[i].Zero()
		} else {
			s.buffer[i] = make([]float32, length)
		}
	}
}

type preparedRegion struct {
	Src                 Source
	Beg, End, Off       Tz
	VolBeg, VolEnd, Pan float32
}

// WAV functions

func (s *Session) wavSizes(numSamples Tz) (riffSize, dataSize uint32) {
	if numSamples < 0 {
		riffSize = math.MaxUint32
		dataSize = riffSize - 36
	} else {
		dataSize = uint32(numSamples * s.sampleRate * blockAlign)
		riffSize = dataSize + riffHeaderSize
	}
	return
}

func (s *Session) wavHeader(numSamples Tz) []byte {
	var (
		byteRate           = s.sampleRate * blockAlign
		riffSize, dataSize = s.wavSizes(numSamples)
	)

	//  0  4 "RIFF"
	//  4  4 riffSize = 36 + samples * byteRate (or just maximum possible)
	//  8  4 "WAVE"
	// 12  4 "fmt "
	// 16  4 fmtSize = 16
	// 20  2 smplFmt
	// 22  2 numChan
	// 24  4 smpRate
	// 28  4 byteRate
	// 30  2 block
	// 32  2 bits
	// 36  4 "data"
	// 40  4 dataSize = samples * byteRate
	// 44  ...

	buf := new(bytes.Buffer)
	buf.Write([]byte("RIFF"))
	binary.Write(buf, binary.LittleEndian, uint32(riffSize))
	buf.Write([]byte("WAVE"))
	buf.Write([]byte("fmt "))
	binary.Write(buf, binary.LittleEndian, uint32(fmtSize))
	binary.Write(buf, binary.LittleEndian, uint16(sampleFormat))
	binary.Write(buf, binary.LittleEndian, uint16(numChannels))
	binary.Write(buf, binary.LittleEndian, uint32(s.sampleRate))
	binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	binary.Write(buf, binary.LittleEndian, uint16(blockAlign))
	binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))
	buf.Write([]byte("data"))
	binary.Write(buf, binary.LittleEndian, uint32(dataSize))

	return buf.Bytes()
}

func (s *Session) writeBuffer() error {
	length := len(s.buffer[0])
	if len(s.buffer[1]) != length {
		return errors.New("invalid buffer")
	}

	out := bufio.NewWriter(s.output)
	b := make([]byte, 8)
	for i := 0; i < length; i++ {
		l := math.Float32bits(s.buffer[0][i])
		r := math.Float32bits(s.buffer[1][i])
		binary.LittleEndian.PutUint32(b[0:4], l)
		binary.LittleEndian.PutUint32(b[4:8], r)
		out.Write(b)
	}
	return out.Flush()
}

func (s *Session) updateHeader() error {
	if w, ok := s.output.(io.WriterAt); ok {
		var (
			buf                = make([]byte, 4)
			riffSize, dataSize = s.wavSizes(s.numOut)
			err                error
		)
		binary.LittleEndian.PutUint32(buf, riffSize)
		_, err = w.WriteAt(buf, riffSizeOff)
		if err != nil && !isPipeErr(err) {
			return err
		}
		binary.LittleEndian.PutUint32(buf, dataSize)
		_, err = w.WriteAt(buf, dataSizeOff)
		if err != nil {
			return err
		}
	}
	return nil
}

func isPipeErr(err error) bool {
	if perr, ok := err.(*os.PathError); ok {
		err = perr
	}
	if err == syscall.ESPIPE {
		return true
	}
	return false
}

func assert(b bool) {
	if !b {
		panic("assert failed")
	}
}
