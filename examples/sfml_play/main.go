package main

import (
	"github.com/kikht/mix/examples"
	"github.com/kikht/mix/sfml"
	"log"
)

func main() {
	sess := examples.SampleSession("examples/audio/")
	end, err := sfml.Play(sess)
	if err == nil {
		<-end
	} else {
		log.Println(err)
	}
}
