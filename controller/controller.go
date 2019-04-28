package controller

import (
	"github.com/kikht/mix"
	"github.com/kikht/mix/session"

	"fmt"
	"log"
	"sort"
)

type Ambience mix.Source
type Effect mix.Source
type Music struct {
	mix.Source
	After string
}

type Controller struct {
	fade   mix.Tz
	player mix.PlayerState

	ambience map[string]Ambience
	music    map[string]Music
	effect   map[string]Effect

	lastAmbience string
}

func NewController(fade mix.Tz, player mix.PlayerState) Controller {
	return Controller{
		fade:     fade,
		ambience: make(map[string]Ambience),
		music:    make(map[string]Music),
		effect:   make(map[string]Effect),
		player:   player,
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
	sort.Strings(res[0])
	for k := range c.music {
		res[1] = append(res[1], k)
	}
	sort.Strings(res[1])
	for k := range c.ambience {
		res[2] = append(res[2], k)
	}
	sort.Strings(res[2])
	return res
}

func (c *Controller) Ambience(label string) (mix.SourceMutator, error) {
	amb, ok := c.ambience[label]
	if !ok {
		return nil, fmt.Errorf("Ambience %s is not found", label)
	}
	c.lastAmbience = label
	return session.NewAmbience(amb, c.fade, c.player.ChunkSize()), nil
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
	return session.NewMusic(mus, amb, c.fade, c.player.ChunkSize()), nil
}

func (c *Controller) Effect(label string) (mix.SourceMutator, error) {
	eff, ok := c.effect[label]
	if !ok {
		return nil, fmt.Errorf("Music %s is not found", label)
	}

	//TODO: fix when it is the first action
	mutator := func(cur mix.Source, pos mix.Tz) mix.Source {
		log.Println("Generating effect", pos)
		var next *session.Session
		next, ok := cur.(*session.Session)
		if ok && next.Length() > pos {
			next = next.Clone().(*session.Session)
		} else {
			next = session.NewSession(c.player.SampleRate(), true)
		}
		next.AddRegion(session.Region{
			Source:  eff,
			Begin:   pos,
			Offset:  0,
			Volume:  1,
			FadeIn:  c.fade,
			FadeOut: c.fade,
		})
		return next
	}
	return mix.SourceMutatorFunc(mutator), nil
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
