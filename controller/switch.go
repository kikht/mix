package controller

import (
	"github.com/kikht/mix"

	"time"
)

type SwitchController struct {
	Controller
	player mix.SwitchPlayer
}

func NewSwitchController(player mix.SwitchPlayer) *SwitchController {
	fade := mix.DurationToTz(100*time.Millisecond, player.SampleRate())
	return &SwitchController{
		Controller: NewController(fade, player),
		player:     player,
	}
}

func (c *SwitchController) Action(label string) error {
	gen, err := c.Controller.Action(label)
	if err != nil {
		return err
	}
	c.player.Switch(gen)
	return nil
}
