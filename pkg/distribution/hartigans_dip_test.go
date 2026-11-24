package distribution

import (
	"fmt"
	"math"
	"math/rand/v2"
	"runtime"
	"testing"
)

// sampleBimodalNormal mischt zwei Normalverteilungen mit demselben Anteil zu einer
// bimodalen Stichprobe.
func sampleBimodalNormal(rng *rand.Rand, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		if i%2 == 0 {
			out[i] = -2.0 + 0.5*rng.NormFloat64()
		} else {
			out[i] = 2.0 + 0.5*rng.NormFloat64()
		}
	}
	return out
}

func TestDipStatisticEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		data []float64
	}{
		{name: "leere Daten", data: []float64{}},
		{name: "ein Wert", data: []float64{1.5}},
		{name: "drei Werte", data: []float64{1, 2, 3}},
		{name: "identische Werte", data: []float64{2, 2, 2, 2, 2, 2}},
		{name: "identische Werte unsortiert", data: []float64{7, 7, 7, 7, 7, 7, 7}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DipStatistic(tt.data); got != 0.0 {
				t.Errorf("DipStatistic(%v) = %v, want 0", tt.data, got)
			}
		})
	}
}

func TestDipStatisticUnimodalVsBimodal(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 1337))
	n := 300

	unimodal := sampleNormal(rng, 0, 1, n)
	bimodal := sampleBimodalNormal(rng, n)

	dipUni := DipStatistic(unimodal)
	dipBi := DipStatistic(bimodal)

	if dipUni <= 0 {
		t.Errorf("DipStatistic für unimodale Daten = %v, want > 0", dipUni)
	}
	if !math.IsNaN(dipUni) && dipUni >= 0.25 {
		t.Errorf("DipStatistic für unimodale Daten = %v, unerwartet hoch", dipUni)
	}
	if dipBi <= dipUni {
		t.Errorf("DipStatistic bimodal (%v) sollte größer sein als unimodal (%v)", dipBi, dipUni)
	}
	if dipBi < 0.05 {
		t.Errorf("DipStatistic für bimodale Daten = %v, expected deutlich > 0", dipBi)
	}
}

func TestDipTest(t *testing.T) {
	t.Run("zu wenige Daten", func(t *testing.T) {
		res := DipTest([]float64{1, 2, 3}, 100, 1)
		if res.Dip != 0 || res.PValue != 1.0 {
			t.Errorf("DipTest mit n<4 = %+v, want Dip=0, PValue=1", res)
		}
	})

	t.Run("identische Werte", func(t *testing.T) {
		res := DipTest([]float64{4, 4, 4, 4, 4, 4, 4, 4}, 100, 1)
		if res.Dip != 0 || res.PValue != 1.0 {
			t.Errorf("DipTest mit identischen Werten = %+v, want Dip=0, PValue=1", res)
		}
	})

	t.Run("keine Simulationen", func(t *testing.T) {
		res := DipTest([]float64{1, 2, 3, 4}, 0, 1)
		if res.PValue != 1.0 {
			t.Errorf("DipTest mit simulations=0: PValue = %v, want 1", res.PValue)
		}
	})

	t.Run("NaN und Inf werden wie in DipStatistic verworfen", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(9, 4))
		clean := sampleBimodalNormal(rng, 120)
		dirty := make([]float64, 0, len(clean)+4)
		for i, v := range clean {
			if i%30 == 0 {
				dirty = append(dirty, math.NaN())
			} else if i%30 == 15 {
				dirty = append(dirty, math.Inf(1))
			}
			dirty = append(dirty, v)
		}
		dirty = append(dirty, math.Inf(-1))

		want := DipTest(clean, 300, 5)
		got := DipTest(dirty, 300, 5)
		if got != want {
			t.Errorf("DipTest mit NaN/Inf = %+v, want %+v", got, want)
		}
	})

	t.Run("bimodal vs unimodal p-Wert", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(7, 99))
		n := 150
		sims := 500

		resUni := DipTest(sampleNormal(rng, 0, 1, n), sims, 1)
		resBi := DipTest(sampleBimodalNormal(rng, n), sims, 1)

		if resBi.PValue >= resUni.PValue {
			t.Errorf("bimodaler p-Wert (%v) sollte kleiner sein als unimodaler (%v)", resBi.PValue, resUni.PValue)
		}
		if resBi.PValue > 0.2 {
			t.Errorf("bimodaler p-Wert = %v, expected klein", resBi.PValue)
		}
	})
}

func assertDipInvariant(t *testing.T, base []float64, transformed []float64, label string) {
	t.Helper()
	want := DipStatistic(base)
	got := DipStatistic(transformed)
	if math.IsNaN(got) {
		t.Fatalf("DipStatistic(%s) = NaN, want ~%v", label, want)
	}
	// float64-Rundung bei großen Translationen/Skalierungen
	tol := 1e-9 * math.Max(1, math.Abs(want))
	if math.Abs(got-want) > tol {
		t.Errorf("DipStatistic(%s) = %v, want ~%v (Invarianzverletzung)", label, got, want)
	}
}

func TestDipStatisticInvariance(t *testing.T) {
	rng := rand.New(rand.NewPCG(2024, 7))
	base := sampleBimodalNormal(rng, 200)

	t.Run("Translation", func(t *testing.T) {
		shifted := make([]float64, len(base))
		for i, v := range base {
			shifted[i] = v + 1234.5
		}
		assertDipInvariant(t, base, shifted, "translation")
	})

	t.Run("Skalierung", func(t *testing.T) {
		scaled := make([]float64, len(base))
		for i, v := range base {
			scaled[i] = v * 37.5
		}
		assertDipInvariant(t, base, scaled, "scaling")
	})

	t.Run("Translation und Skalierung", func(t *testing.T) {
		affine := make([]float64, len(base))
		for i, v := range base {
			affine[i] = v*0.001 - 9876.0
		}
		assertDipInvariant(t, base, affine, "affine")
	})

	t.Run("Spiegelung", func(t *testing.T) {
		reflected := make([]float64, len(base))
		for i, v := range base {
			reflected[i] = -v
		}
		assertDipInvariant(t, base, reflected, "reflection")
	})
}

func TestDipTestInvariance(t *testing.T) {
	rng := rand.New(rand.NewPCG(31337, 1))
	base := sampleBimodalNormal(rng, 120)

	scaled := make([]float64, len(base))
	reflected := make([]float64, len(base))
	for i, v := range base {
		scaled[i] = v*13.0 + 500.0
		reflected[i] = -v
	}

	resBase := DipTest(base, 200, 42)
	resScaled := DipTest(scaled, 200, 42)
	resReflected := DipTest(reflected, 200, 42)

	tol := 1e-9 * math.Max(1, math.Abs(resBase.Dip))
	if math.Abs(resScaled.Dip-resBase.Dip) > tol {
		t.Errorf("DipTest.Dip skaliert = %v, want ~%v", resScaled.Dip, resBase.Dip)
	}
	if math.Abs(resReflected.Dip-resBase.Dip) > tol {
		t.Errorf("DipTest.Dip gespiegelt = %v, want ~%v", resReflected.Dip, resBase.Dip)
	}
	// Der p-Wert ist eine deterministische Funktion der Dip-Statistik
	// (gleicher Seed, gleiche Simulationsanzahl) und muss daher übereinstimmen.
	if resScaled.PValue != resBase.PValue {
		t.Errorf("DipTest.PValue skaliert = %v, want %v", resScaled.PValue, resBase.PValue)
	}
	if resReflected.PValue != resBase.PValue {
		t.Errorf("DipTest.PValue gespiegelt = %v, want %v", resReflected.PValue, resBase.PValue)
	}
}

func TestDipTestReproducibility(t *testing.T) {
	rng := rand.New(rand.NewPCG(555, 42))
	data := sampleBimodalNormal(rng, 120)

	res1 := DipTest(data, 500, 7)
	res2 := DipTest(data, 500, 7)
	if res1 != res2 {
		t.Errorf("DipTest mit gleichem Seed nicht reproduzierbar: %+v vs %+v", res1, res2)
	}
}

func TestDipTestGOMAXPROCSInvariance(t *testing.T) {
	rng := rand.New(rand.NewPCG(777, 1))
	data := sampleBimodalNormal(rng, 150)

	old := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(old)
	resSingle := DipTest(data, 500, 7)

	runtime.GOMAXPROCS(old)
	resMulti := DipTest(data, 500, 7)

	if resSingle != resMulti {
		t.Errorf("DipTest hängt von GOMAXPROCS ab: single=%+v multi=%+v", resSingle, resMulti)
	}
}

func TestDipStatisticReferenceAndContracts(t *testing.T) {
	t.Run("Eingabe-Slice wird nicht mutiert", func(t *testing.T) {
		orig := []float64{4.2, -1.5, 0.3, 8.9, 2.1, -0.4}
		snapshot := append([]float64(nil), orig...)

		_ = DipStatistic(orig)

		for i := range orig {
			if orig[i] != snapshot[i] {
				t.Fatalf("DipStatistic hat Eingabe-Slice an Index %d mutiert: got %v, want %v", i, orig[i], snapshot[i])
			}
		}
	})

	t.Run("Maximaler Dip bei perfekter Zweipunkt-Masse", func(t *testing.T) {
		// Extremfall: Zwei getrennte Punktmassen -> D_n = 0.25 (asymptotisches Maximum)
		data := []float64{0, 0, 0, 0, 0, 10, 10, 10, 10, 10}
		got := DipStatistic(data)
		if math.Abs(got-0.25) > 1e-12 {
			t.Errorf("DipStatistic(Zweipunkt-Masse) = %v, want 0.25", got)
		}
	})

	t.Run("Ties und diskrete Werte", func(t *testing.T) {
		// Reale Rundungsartefakte wie bei winequality-red (citric acid)
		data := []float64{0.0, 0.0, 0.0, 0.01, 0.02, 0.25, 0.49, 0.49, 0.49, 0.50}
		got := DipStatistic(data)
		if math.IsNaN(got) || got <= 0 || got > 0.25 {
			t.Errorf("DipStatistic mit Ties = %v, want in (0, 0.25]", got)
		}
	})

	t.Run("NaN und Inf werden ignoriert", func(t *testing.T) {
		clean := []float64{-2.0, -1.9, -2.1, 2.0, 1.9, 2.1}
		dirty := []float64{math.NaN(), -2.0, math.Inf(1), -1.9, -2.1, 2.0, math.Inf(-1), 1.9, 2.1}

		want := DipStatistic(clean)
		got := DipStatistic(dirty)
		if math.Abs(got-want) > 1e-12 {
			t.Errorf("DipStatistic mit NaN/Inf = %v, want %v", got, want)
		}
	})
}

var (
	benchDipSink    float64
	benchDipResSink DipTestResult
)

func BenchmarkDipStatistic(b *testing.B) {
	sizes := []int{100, 1000, 10000, 100000, 1000000}

	for _, n := range sizes {
		rng := rand.New(rand.NewPCG(42, uint64(n)))
		data := sampleBimodalNormal(rng, n)

		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			var acc float64
			for i := 0; i < b.N; i++ {
				acc += DipStatistic(data)
			}
			benchDipSink = acc
		})
	}
}

func BenchmarkDipTest(b *testing.B) {
	cases := []struct {
		n    int
		sims int
	}{
		{n: 100, sims: 500},
		{n: 100, sims: 2000},
		{n: 100, sims: 10000},
		{n: 1000, sims: 500},
		{n: 1000, sims: 2000},
		{n: 1000, sims: 10000},
		{n: 10000, sims: 500},
		{n: 10000, sims: 2000},
		{n: 10000, sims: 10000},
		{n: 100000, sims: 500},
	}

	for _, tc := range cases {
		rng := rand.New(rand.NewPCG(1337, uint64(tc.n+tc.sims)))
		data := sampleBimodalNormal(rng, tc.n)

		b.Run(fmt.Sprintf("n=%d/sims=%d", tc.n, tc.sims), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			var last DipTestResult
			for i := 0; i < b.N; i++ {
				last = DipTest(data, tc.sims, 42)
			}
			benchDipResSink = last
		})
	}
}
