package controller

import (
	"github.com/kikht/mix"

	"fmt"
	"log"
)

type Ambience mix.Source
type Effect mix.Source
type Music struct {
	mix.Source
	After string
}

type Controller struct {
	fade, sampleRate mix.Tz

	ambience map[string]Ambience
	music    map[string]Music
	effect   map[string]Effect

	lastAmbience string
}

func NewController(fade, sampleRate mix.Tz) Controller {
	return Controller{
		fade:       fade,
		sampleRate: sampleRate,
		ambience:   make(map[string]Ambience),
		music:      make(map[string]Music),
		effect:     make(map[string]Effect),
	}
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

func (c *Controller) Ambience(label string) (mix.SourceMutator, error) {
	amb, ok := c.ambience[label]
	if !ok {
		return nil, fmt.Errorf("Ambience %s is not found", label)
	}
	c.lastAmbience = label

	mutator := func(cur mix.Source, pos mix.Tz) mix.Source {
		log.Println("Generating ambience", pos)
		//TODO: reuse session to prevent allocations
		next := mix.NewSession(c.sampleRate)
		if cur != nil {
			next.AddRegion(mix.Region{
				Source:  cur,
				Begin:   0,
				Offset:  0,
				Volume:  1,
				Length:  pos + c.fade,
				FadeOut: c.fade,
			})
		}
		next.AddRegion(mix.Region{
			Source: amb,
			Begin:  pos,
			Offset: pos,
			Volume: 1,
			FadeIn: c.fade,
		})
		return next
	}
	return mutator, nil
}

func (c *Controller) Music(label string) (mix.SourceMutator, error) {
	mus, ok := c.music[label]
	if !ok {
		return nil, fmt.Errorf("Music %s is not found", label)
	}
	ambLabel := mus.After
	if ambLabel == "" {
		ambLabel = c.lastAmbience
	}
	amb, ok := c.ambience[ambLabel]
	if !ok {
		return nil, fmt.Errorf("Ambience %s after music %s is not found",
			ambLabel, label)
	}
	c.lastAmbience = label

	mutator := func(cur mix.Source, pos mix.Tz) mix.Source {
		log.Println("Generating music", pos)
		//TODO: reuse session to prevent allocations
		next := mix.NewSession(c.sampleRate)
		//Fade out of current ambience
		if cur != nil {
			next.AddRegion(mix.Region{
				Source:  cur,
				Begin:   0,
				Offset:  0,
				Volume:  1,
				Length:  pos + c.fade,
				FadeOut: c.fade,
			})
		}
		//Music itself
		next.AddRegion(mix.Region{
			Source:  mus,
			Begin:   pos,
			Offset:  0,
			Volume:  1,
			FadeIn:  c.fade,
			FadeOut: c.fade,
		})
		//Next ambience with fade in
		eventEnd := pos + mus.Length() - c.fade
		next.AddRegion(mix.Region{
			Source: amb,
			Begin:  eventEnd,
			Offset: eventEnd,
			Volume: 1,
			FadeIn: c.fade,
		})
		return next
	}
	return mutator, nil
}

func (c *Controller) Effect(label string) (mix.SourceMutator, error) {
	eff, ok := c.effect[label]
	if !ok {
		return nil, fmt.Errorf("Music %s is not found", label)
	}

	mutator := func(cur mix.Source, pos mix.Tz) mix.Source {
		log.Println("Generating effect", pos)
		var next *mix.Session
		next, ok := cur.(*mix.Session)
		if ok {
			next = next.Clone().(*mix.Session)
		} else {
			next = mix.NewSession(c.sampleRate)
		}
		next.AddRegion(mix.Region{
			Source:  eff,
			Begin:   pos,
			Offset:  0,
			Volume:  1,
			FadeIn:  c.fade,
			FadeOut: c.fade,
		})
		return next
	}
	return mutator, nil
}

func (c *Controller) Action(label string) (mix.SourceMutator, error) {
	if _, ok := c.effect[label]; ok {
		return c.Effect(label)
	} else if _, ok = c.music[label]; ok {
		return c.Music(label)
	} else {
		return c.Ambience(label)
	}
}
