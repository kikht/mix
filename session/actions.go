package session

import (
	"github.com/kikht/mix"
	"log"
)

type Ambience struct {
	*Session
	fade mix.Tz
}

func NewAmbience(next mix.Source, fade, chunkSize mix.Tz) mix.SourceMutator {
	res := &Session{
		sampleRate: next.SampleRate(),
		length:     next.Length(),
		forgetPast: true,
		regions: []*preparedRegion{
			&preparedRegion{
				VolBeg: 1,
				VolEnd: 0,
			},
			&preparedRegion{
				Src:    next,
				VolBeg: 0,
				VolEnd: 1,
			},
			&preparedRegion{
				Src:    next,
				End:    next.Length(),
				VolBeg: 1,
				VolEnd: 1,
			},
		},
		active: make([]*preparedRegion, 0, 4),
	}
	res.allocateBuffer(chunkSize)
	return Ambience{res, fade}
}

func (a Ambience) Mutate(cur mix.Source, pos mix.Tz) mix.Source {
	log.Println("Ambience.Mutate()", pos)
	var (
		fadeOut = a.regions[0]
		fadeIn  = a.regions[1]
		next    = a.regions[2]
	)

	fadeOut.Src = cur
	fadeOut.Beg = pos
	fadeOut.End = pos + a.fade
	fadeOut.Off = pos

	fadeIn.Beg = pos
	fadeIn.End = pos + a.fade
	fadeIn.Off = pos

	next.Beg = pos + a.fade
	next.Off = pos + a.fade

	a.pos = pos
	return a.Session
}

type Music struct {
	*Session
	fade mix.Tz
}

func NewMusic(mus, next mix.Source, fade, chunkSize mix.Tz) mix.SourceMutator {
	res := &Session{
		sampleRate: next.SampleRate(),
		length:     next.Length(),
		forgetPast: true,
		regions: []*preparedRegion{
			&preparedRegion{
				VolBeg: 1,
				VolEnd: 0,
			},
			&preparedRegion{
				Src:    mus,
				VolBeg: 0,
				VolEnd: 1,
			},
			&preparedRegion{
				Src:    mus,
				VolBeg: 1,
				VolEnd: 1,
			},
			&preparedRegion{
				Src:    mus,
				VolBeg: 1,
				VolEnd: 0,
			},
			&preparedRegion{
				Src:    next,
				VolBeg: 0,
				VolEnd: 1,
			},
			&preparedRegion{
				Src:    next,
				End:    next.Length(),
				VolBeg: 1,
				VolEnd: 1,
			},
		},
		active: make([]*preparedRegion, 0, 4),
	}
	res.allocateBuffer(chunkSize)
	return Music{res, fade}
}

func (m Music) Mutate(cur mix.Source, pos mix.Tz) mix.Source {
	log.Println("Music.Mutate()", pos)
	var (
		prevFadeOut = m.regions[0]
		musFadeIn   = m.regions[1]
		music       = m.regions[2]
		musFadeOut  = m.regions[3]
		nextFadeIn  = m.regions[4]
		next        = m.regions[5]
	)
	musLen := music.Src.Length()

	prevFadeOut.Src = cur
	prevFadeOut.Beg = pos
	prevFadeOut.End = pos + m.fade
	prevFadeOut.Off = pos

	musFadeIn.Beg = pos
	musFadeIn.End = pos + m.fade
	musFadeIn.Off = 0

	music.Beg = pos + m.fade
	music.End = pos + musLen - m.fade
	music.Off = m.fade

	musFadeOut.Beg = pos + musLen - m.fade
	musFadeOut.End = pos + musLen
	musFadeOut.Off = musLen - m.fade

	nextFadeIn.Beg = pos + musLen - m.fade
	nextFadeIn.End = pos + musLen
	nextFadeIn.Off = pos + musLen - m.fade

	next.Beg = pos + musLen
	next.Off = pos + musLen

	m.pos = pos
	return m.Session
}

type Effect struct {
	*Session
}
