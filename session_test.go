package mix

import (
	"encoding/binary"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

const (
	rate   = 44100
	length = 100
)

func TestEmptySession(t *testing.T) {
	s := NewSession(rate)

	tz := s.DurationToTz(1 * time.Second)
	if tz != rate {
		t.Errorf("Invalid Tz conversion. Expected %v, got: %v", rate, tz)
	}

	pos := s.Position()
	if pos != 0 {
		t.Error("Invalid initial position:", pos)
	}

	realRate := s.SampleRate()
	if realRate != rate {
		t.Error("Invalid session sample rate:", realRate)
	}

	err := s.Play(length)
	if err != nil {
		t.Error("Error while playing empty session:", err)
	}

	endPos := s.Position()
	if endPos != length {
		t.Error("Invalid position at end of empty session play:", endPos)
	}
}

func TestAddRegion(t *testing.T) {
	s := NewSession(rate)
	src := getTestSource(1)
	var err error

	err = s.AddRegion(Region{Source: src, Begin: length, Volume: 1})
	if err != nil {
		t.Error("error while adding region", err)
	}
	if len(s.regions) != 1 {
		t.Error("invalid number of regions", len(s.regions))
	}
	if len(s.active) != 0 {
		t.Error("invalid number of active regions", len(s.active))
	}
	if s.rPos != 0 {
		t.Error("invalid rPos", s.rPos)
	}

	err = s.AddRegion(Region{Source: src, Begin: -length, Volume: 1})
	if err != nil {
		t.Error("error while adding region", err)
	}
	if len(s.regions) != 2 {
		t.Error("invalid number of regions", len(s.regions))
	}
	if len(s.active) != 0 {
		t.Error("invalid number of active regions", len(s.active))
	}
	if s.rPos != 1 {
		t.Error("invalid rPos", s.rPos)
	}

	err = s.AddRegion(Region{Source: src, Begin: 0, Volume: 1})
	if err != nil {
		t.Error("error while adding region", err)
	}
	if len(s.regions) != 3 {
		t.Error("invalid number of regions", len(s.regions))
	}
	if len(s.active) != 0 {
		t.Error("invalid number of active regions", len(s.active))
	}
	if s.rPos != 1 {
		t.Error("invalid rPos", s.rPos)
	}

	err = s.AddRegion(Region{Source: src, Begin: -length / 2, Volume: 1})
	if err != nil {
		t.Error("error while adding region", err)
	}
	if len(s.regions) != 4 {
		t.Error("invalid number of regions", len(s.regions))
	}
	if len(s.active) != 1 {
		t.Error("invalid number of active regions", len(s.active))
	}
	if s.rPos != 2 {
		t.Error("invalid rPos", s.rPos)
	}

	prev := s.regions[0].Beg
	for _, r := range s.regions[1:] {
		cur := r.Beg
		if cur < prev {
			t.Error("regions are not sorted", s.regions)
			break
		}
		prev = cur
	}
}

func TestSetPosition(t *testing.T) {
	s := NewSession(rate)
	src := getTestSource(1)
	s.AddRegion(Region{Source: src, Begin: 0, Volume: 1})

	s.SetPosition(length)
	if len(s.active) != 0 {
		t.Error("invalid number of active regions", len(s.active))
	}
	if s.rPos != 1 {
		t.Error("invalid rPos", s.rPos)
	}

	s.SetPosition(0)
	if len(s.active) != 0 {
		t.Error("invalid number of active regions", len(s.active))
	}
	if s.rPos != 0 {
		t.Error("invalid rPos", s.rPos)
	}

	s.SetPosition(length / 2)
	if len(s.active) != 1 {
		t.Error("invalid number of active regions", len(s.active))
	}
	if s.rPos != 1 {
		t.Error("invalid rPos", s.rPos)
	}
}

func TestSilentSession(t *testing.T) {
	s := NewSession(rate)

	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal("Can't open temp file:", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()
	s.SetOutput(file)

	s.AddRegion(Region{
		Source: getTestSource(1),
		Begin:  0,
		Volume: 1,
	})
	s.SetPosition(length)
	s.AddRegion(Region{
		Source: getTestSource(1),
		Begin:  2 * length,
		Volume: 1,
	})

	err = s.Play(length)
	if err != nil {
		t.Error("Error while playing silent session:", err)
	}

	expectDataSize := uint32(length * blockAlign)
	expectRiffSize := uint32(riffHeaderSize + expectDataSize)
	expectLen := int(expectRiffSize + 8)

	data, err := ioutil.ReadFile(file.Name())
	if err != nil {
		t.Fatal("Can't read temp file")
	}
	actualLen := len(data)
	if actualLen != expectLen {
		t.Errorf("Invalid output length. Expected: %v, got: %v", expectLen, actualLen)
	}

	actualRiffSize := binary.LittleEndian.Uint32(data[riffSizeOff : riffSizeOff+4])
	if actualRiffSize != expectRiffSize {
		t.Errorf("Invalid RIFF size. Expected: %v, got: %v",
			expectRiffSize, actualRiffSize)
	}

	actualDataSize := binary.LittleEndian.Uint32(data[dataSizeOff : dataSizeOff+4])
	if actualDataSize != expectDataSize {
		t.Errorf("Invalid data size. Expected: %v, got: %v",
			expectDataSize, actualDataSize)
	}

	for _, b := range data[riffHeaderSize+8:] {
		if b != 0 {
			t.Error("Silent data is not zero")
			break
		}
	}
}

func TestMix(t *testing.T) {
	s := NewSession(rate)
	s.AddRegion(Region{Source: getTestSource(1), Begin: 0, Volume: 1})

	t.Log("mix1")
	buf := s.allocateBuffer(length)
	s.mix(buf)
	if len(s.active) != 0 {
		t.Error("invalid active list", s.active)
	}
	for i, c := range buf {
		val := c[0]
		if val == 0 {
			t.Error("invalid mix data", val)
		}
		for j, v := range c {
			if v != val {
				t.Error("invalid mix data", v, "at", i, j)
				break
			}
		}
	}

	t.Log("mix2")
	s.SetPosition(-length / 2)
	buf = s.allocateBuffer(length)
	s.mix(buf)
	if len(s.active) != 1 {
		t.Error("invalid active list", s.active)
	}
	for i, c := range buf {
		for j := 0; j < length/2; j++ {
			if c[j] != 0 {
				t.Error("invalid mix data", c, "at", i, j)
				break
			}
		}

		val := c[length/2]
		if val == 0 {
			t.Error("invalid mix data", val)
		}
		for j := length / 2; j < length; j++ {
			if c[j] != val {
				t.Error("invalid mix data", c, "at", i, j)
				break
			}
		}
	}

	t.Log("mix3")
	s.SetPosition(length / 2)
	buf = s.allocateBuffer(length)
	s.mix(buf)
	if len(s.active) != 0 {
		t.Error("invalid active list", s.active)
	}
	for i, c := range buf {
		val := c[0]
		if val == 0 {
			t.Error("invalid mix data", val)
		}
		for j := 0; j < length/2; j++ {
			if c[j] != val {
				t.Error("invalid mix data", c, "at", i, j)
				break
			}
		}

		for j := length / 2; j < length; j++ {
			if c[j] != 0 {
				t.Error("invalid mix data", c, "at", i, j)
				break
			}
		}
	}
}

func TestMixStereo(t *testing.T) {
	s := NewSession(rate)
	s.AddRegion(Region{Source: getTestSource(2), Begin: 0, Volume: 1})

	t.Log("mix1")
	buf := s.allocateBuffer(length)
	s.mix(buf)
	for i, c := range buf {
		val := c[0]
		if val == 0 {
			t.Error("invalid mix data", val)
		}
		for j, v := range c {
			if v != val {
				t.Error("invalid mix data", v, "at", i, j)
				break
			}
		}
	}

	t.Log("mix2")
	s.SetPosition(-length / 2)
	buf = s.allocateBuffer(length)
	s.mix(buf)
	for i, c := range buf {
		for j := 0; j < length/2; j++ {
			if c[j] != 0 {
				t.Error("invalid mix data", c, "at", i, j)
				break
			}
		}

		val := c[length/2]
		if val == 0 {
			t.Error("invalid mix data", val)
		}
		for j := length / 2; j < length; j++ {
			if c[j] != val {
				t.Error("invalid mix data", c, "at", i, j)
				break
			}
		}
	}

	t.Log("mix3")
	s.SetPosition(length / 2)
	buf = s.allocateBuffer(length)
	s.mix(buf)
	for i, c := range buf {
		val := c[0]
		if val == 0 {
			t.Error("invalid mix data", val)
		}
		for j := 0; j < length/2; j++ {
			if c[j] != val {
				t.Error("invalid mix data", c, "at", i, j)
				break
			}
		}

		for j := length / 2; j < length; j++ {
			if c[j] != 0 {
				t.Error("invalid mix data", c, "at", i, j)
				break
			}
		}
	}
}

func TestFade(t *testing.T) {
	s := NewSession(rate)
	s.AddRegion(Region{
		Source:  getTestSource(2),
		Begin:   0,
		Volume:  1,
		FadeIn:  length / 2,
		FadeOut: length / 2,
	})

	if len(s.regions) != 2 {
		t.Error("Invalid number of regions")
	}

	buf := s.allocateBuffer(length)
	s.mix(buf)
	for i, c := range buf {
		prev := float32(0)
		for j := 0; j < length/2; j++ {
			cur := c[j]
			if cur < prev {
				t.Error("Invalid mix data", c, "at", i, j)
				break
			}
			prev = cur
		}
		prev = float32(1)
		for j := length / 2; j < length; j++ {
			cur := c[j]
			if cur > prev {
				t.Error("Invalid mix data", c, "at", i, j)
				break
			}
			prev = cur

		}
	}
}

func TestPan(t *testing.T) {
	var (
		s   *Session
		src Source
		buf []Buffer
	)
	const thres = 1e-7

	s = NewSession(rate)
	src = getTestSource(1)
	s.AddRegion(Region{Source: src, Begin: 0, Volume: 1, Pan: -1})
	buf = s.allocateBuffer(length)
	s.mix(buf)
	for j, v := range buf[1] {
		if v > thres {
			t.Error("Invalid mix data", buf[1], "at", j)
			break
		}
	}

	s = NewSession(rate)
	src = getTestSource(1)
	s.AddRegion(Region{Source: src, Begin: 0, Volume: 1, Pan: +1})
	buf = s.allocateBuffer(length)
	s.mix(buf)
	for j, v := range buf[0] {
		if v > thres {
			t.Error("Invalid mix data", buf[0], "at", j)
			break
		}
	}

	s = NewSession(rate)
	src = getTestSource(2)
	s.AddRegion(Region{Source: src, Begin: 0, Volume: 1, Pan: -1})
	buf = s.allocateBuffer(length)
	s.mix(buf)
	for j, v := range buf[1] {
		if v > thres {
			t.Error("Invalid mix data", buf[1], "at", j)
			break
		}
	}

	s = NewSession(rate)
	src = getTestSource(2)
	s.AddRegion(Region{Source: src, Begin: 0, Volume: 1, Pan: +1})
	buf = s.allocateBuffer(length)
	s.mix(buf)
	for j, v := range buf[0] {
		if v > thres {
			t.Error("Invalid mix data", buf[0], "at", j)
			break
		}
	}
}

func getTestSource(channels int) Source {
	res := MemSource{
		Rate: rate,
		Data: make([]Buffer, channels),
	}
	for i := range res.Data {
		res.Data[i] = NewBuffer(length)
		for j := range res.Data[i] {
			res.Data[i][j] = 1.0
		}
	}
	return res
}
