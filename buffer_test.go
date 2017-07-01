package mix

import (
	"testing"
)

var sink Buffer

func benchmarkMixGain(size Tz, b *testing.B) {
	src := NewBuffer(size)
	dst := NewBuffer(size)
	for n := 0; n < b.N; n++ {
		dst.MixGain(src, 1.0)
	}
	sink = dst
}

func BenchmarkMixGain32(b *testing.B)  { benchmarkMixGain(32, b) }
func BenchmarkMixGain64(b *testing.B)  { benchmarkMixGain(64, b) }
func BenchmarkMixGain128(b *testing.B) { benchmarkMixGain(128, b) }
func BenchmarkMixGain256(b *testing.B) { benchmarkMixGain(256, b) }
func BenchmarkMixGain512(b *testing.B) { benchmarkMixGain(512, b) }
func BenchmarkMixGain1k(b *testing.B)  { benchmarkMixGain(1024, b) }
func BenchmarkMixGain4k(b *testing.B)  { benchmarkMixGain(4096, b) }
func BenchmarkMixGain16k(b *testing.B) { benchmarkMixGain(16384, b) }
func BenchmarkMixGain64k(b *testing.B) { benchmarkMixGain(65536, b) }
