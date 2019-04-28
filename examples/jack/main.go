package main

import (
	"github.com/kikht/mix/examples"
	"github.com/kikht/mix/jack"
	"log"
)

func main() {
	sess := examples.SampleSession("examples/audio/")
	stream, err := jack.NewStream(sess.NumChannels())
	if err == nil {
		stream.Play(sess)
		<-stream.End()
	} else {
		log.Println(err)
	}
}
