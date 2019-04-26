package session

import (
	"github.com/kikht/mix"
	"testing"
	"time"
)

const (
	rate   = 44100
	length = 100
)

func TestEmptySession(t *testing.T) {
	s := NewSession(rate, true)

	tz := mix.DurationToTz(1*time.Second, rate)
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

	buf := s.Samples(0, 0, length)
	for _, v := range buf {
		if v != 0 {
			t.Error("Non-zero samples from empty session")
			break
		}
	}

	endPos := s.Position()
	if endPos != length {
		t.Error("Invalid position at end of empty session play:", endPos)
	}
}

func TestAddRegion(t *testing.T) {
	s := NewSession(rate, true)
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
	s := NewSession(rate, false)
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
	s := NewSession(rate, true)

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

	buf := s.Samples(0, length, length)
	for _, v := range buf {
		if v != 0 {
			t.Error("Silent data is not zero")
			break
		}
	}
}

func TestMix(t *testing.T) {
	s := NewSession(rate, false)
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
	s := NewSession(rate, false)
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
	s := NewSession(rate, true)
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
		src mix.Source
		buf [2]mix.Buffer
	)
	const thres = 1e-7

	s = NewSession(rate, true)
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

	s = NewSession(rate, true)
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

	s = NewSession(rate, true)
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

	s = NewSession(rate, true)
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

func getTestSource(channels int) mix.Source {
	res := mix.MemSource{
		Rate: rate,
		Data: make([]mix.Buffer, channels),
	}
	for i := range res.Data {
		res.Data[i] = mix.NewBuffer(length)
		for j := range res.Data[i] {
			res.Data[i][j] = 1.0
		}
	}
	return res
}
