package main

import (
	"github.com/kikht/mix/examples"
	"github.com/kikht/mix/sfml"
	"log"
)

func main() {
	sess := examples.SampleSession("examples/audio/")
	stream, err := sfml.NewStream(sess.NumChannels(), sess.SampleRate())
	if err == nil {
		stream.Play(sess)
		<-stream.End()
	} else {
		log.Println(err)
	}
}
