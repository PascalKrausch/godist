package distribution

import (
	"fmt"
	"math"
	"testing"
)

// benchSizes sind die Stichprobengrößen für alle Benchmarks; evaluation/bench.py nutzt dieselben.
var benchSizes = []int{100, 10_000, 1_000_000}

// benchFitters enthält dieselben neun Verteilungen wie evaluation/compare.py.
var benchFitters = []Fitter{
	BetaFitter{},
	ExponentialFitter{},
	NormalFitter{},
	GumbelFitter{},
	GammaFitter{},
	LogNormalFitter{},
	WeibullFitter{},
	LaplaceFitter{},
	RayleighFitter{},
}

// benchData erzeugt exponentialverteilte Daten aus SplitMix64 (Seed 42) per Inversionsmethode.
// evaluation/bench.py (gen_data) implementiert dieselbe Erzeugung, sodass beide Seiten identische Eingaben fitten.
func benchData(n int) []float64 {
	const (
		seed        uint64 = 42
		goldenGamma uint64 = 0x9E3779B97F4A7C15
	)
	data := make([]float64, n)
	state := seed
	for i := range data {
		state += goldenGamma
		z := state
		z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
		z = (z ^ (z >> 27)) * 0x94D049BB133111EB
		z ^= z >> 31
		u := (float64(z>>11) + 0.5) / (1 << 53)
		data[i] = -math.Log(u)
	}
	return data
}

func TestBenchDataReference(t *testing.T) {
	// Referenzwerte aus evaluation/bench.py (gen_data); sichern die Übereinstimmung mit der Python-Seite.
	want := []float64{0.29899262457387227, 1.8331416651510681, 1.2779741607346153}
	got := benchData(len(want))
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-12 {
			t.Fatalf("benchData[%d] = %.17g, want %.17g", i, got[i], want[i])
		}
	}
}

func BenchmarkFit(b *testing.B) {
	for _, n := range benchSizes {
		data := benchData(n)
		for _, f := range benchFitters {
			b.Run(fmt.Sprintf("%s/n=%d", f.Name(), n), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, err := f.Fit(data); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// BenchmarkSelectAllModels entspricht Fit plus Gütemetriken für alle Modelle; Vergleich zu AllModels in SciPy.
func BenchmarkSelectAllModels(b *testing.B) {
	for _, n := range benchSizes {
		data := benchData(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, _, err := SelectAllModels(data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// selectAllModelsSequential ist die sequenzielle Referenzimplementierung für den Crossover-Benchmark.
func selectAllModelsSequential(data []float64) ([]FitResult, []FitFailure, error) {
	fitters := []Fitter{
		WeibullFitter{}, LogNormalFitter{}, GammaFitter{}, NormalFitter{}, BetaFitter{},
		ExponentialFitter{}, GumbelFitter{}, RayleighFitter{}, LaplaceFitter{},
	}
	var results []FitResult
	var failures []FitFailure
	for _, fitter := range fitters {
		o := fitOne(fitter, data)
		if o.failure != nil {
			failures = append(failures, *o.failure)
		} else {
			results = append(results, o.result)
		}
	}
	return results, failures, nil
}

// BenchmarkCrossover vergleicht sequenziell und parallel über Stichprobengrößen, um den Schwellenwert zu finden.
func BenchmarkCrossover(b *testing.B) {
	sizes := []int{10, 20, 30, 50, 100, 200, 500, 1_000, 5_000, 10_000, 50_000, 100_000, 500_000, 1_000_000}
	for _, n := range sizes {
		data := benchData(n)
		b.Run(fmt.Sprintf("seq/n=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, _, err := selectAllModelsSequential(data); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("par/n=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, _, err := SelectAllModels(data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
