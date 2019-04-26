package controller

import (
	"fmt"
	"time"

	"github.com/kikht/mix"
)

type Ambience mix.Source
type Effect mix.Source
type Music struct {
	mix.Source
	After string
}

type Controller struct {
	sampleRate  mix.Tz
	ahead, fade mix.Tz
	start       time.Time

	player mix.Player

	ambience map[string]Ambience
	music    map[string]Music
	effect   map[string]Effect

	mix *mix.Session
	cur string
}

func NewController(sampleRate mix.Tz, player mix.Player) *Controller {
	return &Controller{
		sampleRate: sampleRate,
		ahead:      mix.DurationToTz(300*time.Millisecond, sampleRate),
		fade:       mix.DurationToTz(100*time.Millisecond, sampleRate),
		player:     player,
		ambience:   make(map[string]Ambience),
		music:      make(map[string]Music),
		effect:     make(map[string]Effect),
	}
}

func (c *Controller) Mix() *mix.Session {
	return c.mix
}

func (c *Controller) AddAmbience(label string, sound mix.Source) {
	c.ambience[label] = Ambience(sound)
}

func (c *Controller) AddMusic(label string, sound mix.Source, after string) {
	c.music[label] = Music{sound, after}
}

func (c *Controller) AddEffect(label string, sound mix.Source) {
	c.effect[label] = Effect(sound)
}

func (c *Controller) pos() mix.Tz {
	if c.start.IsZero() {
		c.start = time.Now()
		return 0
	}
	return mix.DurationToTz(time.Now().Sub(c.start), c.sampleRate)
}

func (c *Controller) SetAmbience(label string) error {
	amb, ok := c.ambience[label]
	if !ok {
		return fmt.Errorf("Ambience %s is not found", label)
	}

	pos := c.pos() + c.ahead - c.fade
	oldMix := c.mix
	newMix := mix.NewSession(c.sampleRate)
	if oldMix != nil {
		newMix.AddRegion(mix.Region{
			Source:  oldMix,
			Begin:   0,
			Offset:  0,
			Volume:  1,
			Length:  pos,
			FadeOut: c.fade,
		})
	}
	newMix.AddRegion(mix.Region{
		Source: amb,
		Begin:  pos,
		Offset: pos,
		Volume: 1,
		FadeIn: c.fade,
	})

	c.cur = label
	c.mix = newMix
	c.player.Play(c.mix)
	return nil
}

func (c *Controller) PlayMusic(label string) error {
	mus, ok := c.music[label]
	if !ok {
		return fmt.Errorf("Music %s is not found", label)
	}
	ambLabel := mus.After
	if ambLabel == "" {
		ambLabel = c.cur
	}
	amb, ok := c.ambience[ambLabel]
	if !ok {
		return fmt.Errorf("Ambience %s after music %s is not found",
			ambLabel, label)
	}

	pos := c.pos() + c.ahead - c.fade
	oldMix := c.mix
	newMix := mix.NewSession(c.sampleRate)
	//Fade out of current ambience
	if oldMix != nil {
		newMix.AddRegion(mix.Region{
			Source:  oldMix,
			Begin:   0,
			Offset:  0,
			Volume:  1,
			Length:  pos,
			FadeOut: c.fade,
		})
	}
	//Music itself
	newMix.AddRegion(mix.Region{
		Source:  mus,
		Begin:   pos,
		Offset:  0,
		Volume:  1,
		FadeIn:  c.fade,
		FadeOut: c.fade,
	})
	//Next ambience with fade in
	eventEnd := pos + mus.Length() - c.fade
	newMix.AddRegion(mix.Region{
		Source: amb,
		Begin:  eventEnd,
		Offset: eventEnd,
		Volume: 1,
		FadeIn: c.fade,
	})

	c.mix = newMix
	c.player.Play(c.mix)
	return nil
}

func (c *Controller) FireEffect(label string) error {
	eff, ok := c.effect[label]
	if !ok {
		return fmt.Errorf("Music %s is not found", label)
	}

	pos := c.pos() + c.ahead - c.fade
	if c.mix == nil {
		c.mix = mix.NewSession(c.sampleRate)
	}
	c.mix.AddRegion(mix.Region{
		Source:  eff,
		Begin:   pos,
		Offset:  0,
		Volume:  1,
		FadeIn:  c.fade,
		FadeOut: c.fade,
	})

	c.player.Play(c.mix)
	return nil
}

func (c *Controller) Actions() [][]string {
	res := make([][]string, 3)
	for k := range c.effect {
		res[0] = append(res[0], k)
	}
	for k := range c.music {
		res[1] = append(res[1], k)
	}
	for k := range c.ambience {
		res[2] = append(res[2], k)
	}
	return res
}

func (c *Controller) Action(label string) error {
	if _, ok := c.effect[label]; ok {
		return c.FireEffect(label)
	} else if _, ok = c.music[label]; ok {
		return c.PlayMusic(label)
	} else {
		return c.SetAmbience(label)
	}
}
