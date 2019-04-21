package mix

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"sort"
	"syscall"
	"time"
)

// Session mixes collection of Regions. Output is done in 32-bit float WAV.
// Session implements Source, so it could be nested.
type Session struct {
	sampleRate Tz
	pos        Tz
	length     Tz

	//TODO: separate wav (or other format) writer
	output io.Writer
	numOut Tz

	buffer []Buffer

	//TODO: separate regions collection & implement as tree
	regions []*preparedRegion
	rPos    int
	active  []*preparedRegion
}

const numChannels = 2

// NewSession creates Session with given sampleRate.
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

// Play mixes length samples, writes them to output and advances currernt position by length.
func (s *Session) Play(length Tz) error {
	if length < 0 {
		return errors.New("Can't play length < 0")
	}

	buf := s.allocateBuffer(length)
	s.mix(buf)

	if s.numOut == 0 {
		_, err := s.output.Write(s.wavHeader(-1))
		if err != nil {
			return err
		}
	}
	s.numOut += length
	err := s.writeBuffer(buf)
	if err != nil {
		return errors.New("error while writing audio buffer: " + err.Error())
	}
	err = s.updateHeader()
	if err != nil {
		return errors.New("error while updating WAV header: " + err.Error())
	}
	return nil
}

func (s *Session) mix(buffer []Buffer) {
	if len(buffer) != numChannels {
		panic("invalid buffer")
	}
	length := Tz(len(buffer[0]))
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
		bEnd -= s.pos

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

		//log.Printf("Mixing region %v, pos=%v end=%v rOff=%v bOff=%v rEnd=%v bEnd=%v rLen=%v gain=%v\n", r, s.pos, end, rOff, bOff, rEnd, bEnd, rLen, gain)

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
				dst := buffer[j][bOff:bEnd]
				assert(len(src) == len(dst))
				dst.MixGain(src, gain[i][j]*vol)
			}
		}

		if end < r.End {
			s.active[lastActive] = r
			lastActive++
		}
	}
	s.active = s.active[0:lastActive]
	s.pos += length
}

// DurationToTz converts time.Duration to number of samples with Session sample rate.
func (s *Session) DurationToTz(d time.Duration) Tz {
	return DurationToTz(s, d)
}

// Length returns end of last region in Session
func (s *Session) Length() Tz {
	return s.length
}

// NumChannels returns number of channels in Session
func (s *Session) NumChannels() int {
	return numChannels
}

func (s *Session) Samples(channel int, offset, length Tz) Buffer {
	// Fast-path for already mixed data
	if offset+length == s.pos &&
		len(s.buffer) > channel &&
		Tz(len(s.buffer[channel])) == length {

		return s.buffer[channel]
	}

	s.SetPosition(offset)
	buf := s.allocateBuffer(length)
	s.mix(buf)
	return buf[channel]
}

// SetPosition sets current Session position.
func (s *Session) SetPosition(pos Tz) {
	if s.pos == pos {
		return
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
func (s *Session) Position() Tz {
	return s.pos
}

// SampleRate returns sample rate of Session.
func (s *Session) SampleRate() Tz {
	return s.sampleRate
}

// SetOutput redirects session output to given io.Writer.
func (s *Session) SetOutput(output io.Writer) {
	s.output = output
	s.numOut = 0
}

func (s *Session) allocateBuffer(length Tz) []Buffer {
	if len(s.buffer) != numChannels {
		s.buffer = make([]Buffer, numChannels)
	}
	for i := 0; i < numChannels; i++ {
		if Tz(cap(s.buffer[i])) >= length {
			s.buffer[i] = s.buffer[i][0:length]
			s.buffer[i].Zero()
		} else {
			s.buffer[i] = NewBuffer(length)
		}
	}
	return s.buffer
}

type preparedRegion struct {
	Src                 Source
	Beg, End, Off       Tz
	VolBeg, VolEnd, Pan float32
}

func (r preparedRegion) String() string {
	return fmt.Sprintf("{Beg=%v End=%v Off=%v Vol=%4.2f:%4.2f Pan=%+5.2f}",
		r.Beg, r.End, r.Off, r.VolBeg, r.VolEnd, r.Pan)
}

// WAV functions

const (
	bitsPerSample      = 32
	sampleFormat       = 3 //for float, 1 for PCM
	sampleFormatSuffix = "\x00\x00\x00\x00\x10\x00\x80\x00\x00\xAA\x00\x38\x9B\x71"
	blockAlign         = numChannels * bitsPerSample / 8

	extSize        = 2 + 4 + 16
	fmtSize        = 2 + 2 + 4 + 4 + 2 + 2 + 2 + extSize
	riffSizeOff    = 4
	riffHeaderSize = 4 + 4 + 4 + fmtSize + 4 + 4
	dataSizeOff    = riffHeaderSize + 4
)

func (s *Session) wavSizes(numSamples Tz) (riffSize, dataSize uint32) {
	if numSamples < 0 {
		riffSize = math.MaxUint32
		dataSize = riffSize - riffHeaderSize
	} else {
		dataSize = uint32(numSamples * blockAlign)
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
	//  4  4 riffSize = header + samples * byteRate (or just maximum possible)
	//  8  4 "WAVE"
	// 12  4 "fmt "
	// 16  4 fmtSize
	// 20  2 smplFmt
	// 22  2 numChan
	// 24  4 smpRate
	// 28  4 byteRate
	// 32  2 block
	// 34  2 bits
	// 36  2 extSize
	// 38  2 validBits
	// 40  4 channelMask
	// 44 16 format
	// 60  4 "data"
	// 64  4 dataSize = samples * byteRate
	// 68  ...

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
	binary.Write(buf, binary.LittleEndian, uint16(extSize))
	binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))
	binary.Write(buf, binary.LittleEndian, uint32(0))
	binary.Write(buf, binary.LittleEndian, uint16(sampleFormat))
	buf.Write([]byte(sampleFormatSuffix))
	buf.Write([]byte("data"))
	binary.Write(buf, binary.LittleEndian, uint32(dataSize))

	return buf.Bytes()
}

func (s *Session) writeBuffer(buffer []Buffer) error {
	if len(buffer) != numChannels {
		return errors.New("Only stereo buffers are supported")
	}
	length := len(buffer[0])
	if len(buffer[1]) != length {
		return errors.New("invalid buffer")
	}

	out := bufio.NewWriter(s.output)
	b := make([]byte, 8)
	for i := 0; i < length; i++ {
		l := math.Float32bits(buffer[0][i])
		r := math.Float32bits(buffer[1][i])
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
		if err != nil {
			if isPipeErr(err) {
				return nil
			}
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
		err = perr.Err
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
