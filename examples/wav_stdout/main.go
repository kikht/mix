package main

import (
	"github.com/kikht/mix"
	"github.com/kikht/mix/examples"
	"os"
)

func main() {
	sess := examples.SampleSession("examples/audio/")
	sess.SetOutput(os.Stdout)
	const chunk = 1 << 16
	for i := mix.Tz(0); i < sess.Length(); i += chunk {
		sess.Play(chunk)
	}
}
