package controller

import (
	"github.com/kikht/mix"

	"time"
)

type AheadController struct {
	Controller
	ahead  mix.Tz
	player mix.Player
	start  time.Time
	mix    mix.Source
}

func NewAheadController(sampleRate mix.Tz, player mix.Player) *AheadController {
	fade := mix.DurationToTz(100*time.Millisecond, sampleRate)
	return &AheadController{
		Controller: NewController(fade, sampleRate, player.ChunkSize()),
		ahead:      mix.DurationToTz(300*time.Millisecond, sampleRate),
		player:     player,
	}
}

func (c *AheadController) Action(label string) error {
	gen, err := c.Controller.Action(label)
	if err != nil {
		return err
	}
	c.mix = gen.Mutate(c.mix, c.pos())
	c.player.Play(c.mix)
	return nil
}

func (c *AheadController) Mix() mix.Source {
	return c.mix
}

func (c *AheadController) pos() mix.Tz {
	if c.start.IsZero() {
		c.start = time.Now()
		return c.ahead
	}
	return mix.DurationToTz(time.Now().Sub(c.start), c.sampleRate) + c.ahead
}
