package mix_test

import (
	"github.com/kikht/mix"
	"os"
)

func Example() {
	const (
		sampleRate = 44100
		tempo      = 58
		bars       = 4
		whole      = sampleRate * 60 * 4 / tempo
		length     = bars * whole
	)
	sess := mix.NewSession(sampleRate)
	sess.SetOutput(os.Stdout)

	// It's only example. Handle your errors properly!
	kick, _ := mix.LoadSOX("examples/audio/kick.ogg")
	snare, _ := mix.LoadSOX("examples/audio/snare.ogg")
	hat, _ := mix.LoadSOX("examples/audio/hat.ogg")
	crash, _ := mix.LoadSOX("examples/audio/crash.ogg")
	guitar, _ := mix.LoadSOX("examples/audio/guitar.ogg")

	drums := mix.NewSession(sampleRate)
	drums.AddRegion(mix.Region{Source: crash, Begin: 0, Volume: 0.7, FadeOut: crash.Length()})
	for h := mix.Tz(whole / 16); h < whole; h += whole / 16 {
		drums.AddRegion(mix.Region{Source: hat, Begin: h, Volume: 0.5, Pan: -0.3})
	}

	kickPos := [...]mix.Tz{0, whole * 7 / 16, whole * 9 / 16}
	for _, k := range kickPos {
		drums.AddRegion(mix.Region{Source: kick, Begin: k, Volume: 1})
	}

	snarePos := [...]mix.Tz{whole / 4, whole * 3 / 4}
	for _, s := range snarePos {
		drums.AddRegion(mix.Region{Source: snare, Begin: s, Volume: 1, Pan: 0.1})
	}

	sess.AddRegion(mix.Region{Source: drums, Begin: 0, Volume: 1, FadeIn: whole})
	for b := mix.Tz(whole); b < (bars-1)*whole; b += whole {
		sess.AddRegion(mix.Region{Source: drums, Begin: b, Volume: 1})
	}
	sess.AddRegion(mix.Region{Source: drums, Begin: (bars - 1) * whole, Volume: 1, FadeOut: whole})

	sess.AddRegion(mix.Region{Source: guitar, Begin: 0, Volume: 1, FadeIn: whole})
	sess.AddRegion(mix.Region{Source: guitar, Begin: 2 * whole, Volume: 1, FadeOut: whole})

	for i := 0; i < bars; i++ {
		sess.Play(whole)
	}
}
