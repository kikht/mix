# Mix

[![GoDoc](https://godoc.org/github.com/kikht/mix?status.svg)](https://godoc.org/github.com/kikht/mix) [![Build Status](https://travis-ci.org/kikht/mix.svg?branch=master)](https://travis-ci.org/kikht/mix) [![Go Report Card](https://goreportcard.com/badge/github.com/kikht/mix)](https://goreportcard.com/report/github.com/kikht/mix)

Audio mixer for golang. Inspired by https://github.com/go-mix/mix but has following differences:
- All audio operations are vectorized.
- No forced compression on whole mix. Optional compressors are planned, but not implemented yet.
- Fade-in & fade-out on audio regions.
- Float32 for audio samples - more than enough for audio.
- Most time calculations are in number of samples. Converters to time.Duration are provided.

## Demo

```
go run examples/main.go | aplay
```

## Dependencies 

- github.com/rkusa/gm/math32 - math functions for float32
- github.com/krig/go-sox - cgo bindings to [SoX](http://sox.sourceforge.net/) for audio input
- github.com/xthexder/go-jack - cgo bindings to [jackd](http://jackaudio.org)
- sfml package requires [csfml 2.4](https://www.sfml-dev.org)
