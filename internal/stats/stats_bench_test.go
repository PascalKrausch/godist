package stats

import (
	"math/rand/v2"
	"testing"
)

func BenchmarkDescribe(b *testing.B) {
	for _, n := range []int{100, 10_000, 1_000_000} {
		rng := rand.New(rand.NewPCG(42, 0))
		data := make([]float64, n)
		for i := range data {
			data[i] = rng.ExpFloat64()
		}
		b.Run(map[int]string{100: "n100", 10_000: "n10k", 1_000_000: "n1M"}[n], func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				Describe(data)
			}
		})
	}
}
