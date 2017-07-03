package mix

import "github.com/rkusa/gm/math32"

// Buffer of audio data.
type Buffer []float32

// NewBuffer creates buffer of length samples.
func NewBuffer(length Tz) Buffer {
	return make([]float32, length)
}

// Zero fills buffer with silence.
func (dst Buffer) Zero() {
	for i := range dst {
		dst[i] = 0
	}
}

// Clone buffer with contents.
func (src Buffer) Clone() Buffer {
	dst := make([]float32, len(src))
	copy(dst, src)
	return dst
}

// CopyGain copies samples from src into dst scaling by gain.
func (dst Buffer) CopyGain(src Buffer, gain float32) {
	n := copy(dst, src)
	dst[0:n].Gain(gain)
}

// Gain scales all samples by gain.
func (dst Buffer) Gain(gain float32) {
	for i := range dst {
		dst[i] *= gain
	}
}

// Mix puts sum of src and dst into dst.
func (dst Buffer) Mix(src Buffer) {
	n := len(dst)
	if len(src) < n {
		n = len(src)
	}
	for i := 0; i < n; i++ {
		dst[i] += src[i]
	}
}

// MixGain puts sum of src scaled by gain and dst into dst.
func (dst Buffer) MixGain(src Buffer, gain float32) {
	n := len(dst)
	if len(src) < n {
		n = len(src)
	}
	for i := 0; i < n; i++ {
		dst[i] += src[i] * gain
	}
}

// LinearRamp scales dst with linearly changing gain from initial to target.
func (dst Buffer) LinearRamp(initial, target float32) {
	delta := (target - initial) / float32(len(dst))
	for i := range dst {
		dst[i] *= initial
		initial += delta
	}
}

// SqrtRamp scales dst with "sqrt-linear" changing gain from sqrt(initial) to sqrt(target). Useful for equal-power crossfade.
func (dst Buffer) SqrtRamp(initial, target float32) {
	a := (target - initial) / float32(len(dst))
	b := initial
	for i, fi := 0, float32(0); i < len(dst); i, fi = i+1, fi+1 {
		dst[i] *= math32.Sqrt(a*fi + b)
	}
}

// Some useful info about panning in Ardour: http://lists.project-wombat.org/pipermail/ardour-dev-ardour.org/2005-August/009449.html
// Details of sincos implementation in golang: https://groups.google.com/forum/#!topic/golang-dev/gFJDX3mnjQU
// Anyway it's better to cache results of this in preparedRegion struct
func panStereoGain(pan float32) (l2l, l2r, r2l, r2r float32) {
	if pan > 1 {
		pan = 1
	} else if pan < -1 {
		pan = -1
	}

	w := pan
	if w < 0 {
		w = -w
	}
	w = 1 - w

	panL := (pan + 1 - w) / 2
	panR := (pan + 1 + w) / 2

	const coef = math32.Pi / 2
	l2r, l2l = math32.Sincos(panL * coef)
	r2r, r2l = math32.Sincos(panR * coef)
	return
}

func panMonoGain(pan float32) (l, r float32) {
	if pan > 1 {
		pan = 1
	} else if pan < -1 {
		pan = -1
	}
	const coef = math32.Pi / 4
	return math32.Sincos((1 - pan) * coef)
}
