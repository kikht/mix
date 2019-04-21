package controller

import (
	"errors"
	"log"
	"time"

	"github.com/kikht/mix"
)

type Controller struct {
	sampleRate  mix.Tz
	ahead, fade mix.Tz
	start       time.Time

	listener ControllerListener

	ambience map[string]Ambience
	event    map[string]Event

	mix *mix.Session
	cur string
}

type ControllerListener func(source mix.Source)

type Ambience struct {
	mix.Source
	Label string
}

type Event struct {
	mix.Source
	Label string
	Over  bool
	After string
}

func NewController(sampleRate mix.Tz, listener ControllerListener) *Controller {
	return &Controller{
		sampleRate: sampleRate,
		ahead:      mix.DurationToTz(300*time.Millisecond, sampleRate),
		fade:       mix.DurationToTz(100*time.Millisecond, sampleRate),
		listener:   listener,
		ambience:   make(map[string]Ambience),
		event:      make(map[string]Event),
	}
}

func (c *Controller) Mix() *mix.Session {
	return c.mix
}

func (c *Controller) AddAmbience(a Ambience) {
	log.Println("adding ambience", a.Label)
	c.ambience[a.Label] = a
}

func (c *Controller) AddEvent(e Event) {
	log.Println("adding event", e.Label)
	c.event[e.Label] = e
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
		return errors.New("ambience not found")
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
	c.listener(c.mix)
	return nil
}

func (c *Controller) FireEvent(label string) error {
	ev, ok := c.event[label]
	if !ok {
		return errors.New("event not found")
	}
	ambLabel := ev.After
	if ambLabel == "" {
		ambLabel = c.cur
	}
	amb, ok := c.ambience[ambLabel]
	if !ok {
		return errors.New("ambience not found")
	}

	pos := c.pos() + c.ahead - c.fade
	oldMix := c.mix
	var newMix *mix.Session
	if ev.Over {
		if oldMix != nil {
			newMix = oldMix
		} else {
			newMix = mix.NewSession(c.sampleRate)
		}

	} else {
		newMix = mix.NewSession(c.sampleRate)
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
		//Next ambience with fade in
		eventEnd := pos + ev.Length() - c.fade
		newMix.AddRegion(mix.Region{
			Source: amb,
			Begin:  eventEnd,
			Offset: eventEnd,
			Volume: 1,
			FadeIn: c.fade,
		})
	}
	//Event sound itself
	newMix.AddRegion(mix.Region{
		Source:  ev,
		Begin:   pos,
		Offset:  0,
		Volume:  1,
		FadeIn:  c.fade,
		FadeOut: c.fade,
	})

	c.mix = newMix
	c.listener(c.mix)
	return nil
}

func (c *Controller) Ambience() []string {
	var res []string
	for k := range c.ambience {
		res = append(res, k)
	}
	return res
}

func (c *Controller) Events() []string {
	var res []string
	for k := range c.event {
		res = append(res, k)
	}
	return res
}

func (c *Controller) Actions() []string {
	var res []string
	for k := range c.event {
		res = append(res, k)
	}
	for k := range c.ambience {
		res = append(res, k)
	}
	return res
}

func (c *Controller) Action(label string) error {
	_, ok := c.event[label]
	if ok {
		return c.FireEvent(label)
	} else {
		return c.SetAmbience(label)
	}
}
