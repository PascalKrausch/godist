package distribution

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
)

// sampleBurrXII per Inversionsmethode: λ·((1 − U)^(−1/k) − 1)^(1/c).
func sampleBurrXII(rng *rand.Rand, dist BurrXII, n int) []float64 {

	out := make([]float64, n)
	for i := range out {
		u := rng.Float64()
		out[i] = dist.Quantile(u)
	}
	return out
}

func relErr(got, want float64) float64 {
	return math.Abs(got-want) / math.Abs(want)
}

// Referenzwerte stammen aus scipy.stats.burr12(c=3, d=0.5, scale=2).
func TestBurrXIIMatchesScipyReference(t *testing.T) {
	dist := BurrXII{Scale: 2, ShapeC: 3, ShapeK: 0.5}
	tests := []struct {
		x, pdf, logPDF, cdf float64
	}{
		{0.5, 0.04579744046370775, -3.08352707449551, 0.00772212328633235},
		{1.5, 0.24882365355157626, -1.3910108520911098, 0.16137213062246536},
		{4.0, 0.1111111111111111, -2.1972245773362196, 2.0 / 3.0},
	}
	for _, tt := range tests {
		if got := dist.PDF(tt.x); relErr(got, tt.pdf) > 1e-12 {
			t.Errorf("PDF(%v) = %v, want %v", tt.x, got, tt.pdf)
		}
		if got := dist.LogPDF(tt.x); relErr(got, tt.logPDF) > 1e-12 {
			t.Errorf("LogPDF(%v) = %v, want %v", tt.x, got, tt.logPDF)
		}
		if got := dist.CDF(tt.x); relErr(got, tt.cdf) > 1e-12 {
			t.Errorf("CDF(%v) = %v, want %v", tt.x, got, tt.cdf)
		}
	}

	quantiles := []struct{ p, want float64 }{
		{1e-10, 0.0011696070953436273},
		{0.3, 2.0268488413573458},
		{0.9999, 928.3177636282315},
	}
	for _, tt := range quantiles {
		if got := dist.Quantile(tt.p); relErr(got, tt.want) > 1e-9 {
			t.Errorf("Quantile(%v) = %v, want %v", tt.p, got, tt.want)
		}
	}
}

func TestBurrXIIQuantileRoundTripInTails(t *testing.T) {
	dist := BurrXII{Scale: 1e-150, ShapeC: 0.4, ShapeK: 7}
	for _, p := range []float64{1e-30, 1e-12, 0.5, 1 - 1e-12} {
		x := dist.Quantile(p)
		if math.IsNaN(x) || x <= 0 {
			t.Fatalf("Quantile(%v) = %v, want positive finite value", p, x)
		}
		if got := dist.CDF(x); relErr(got, p) > 1e-8 {
			t.Errorf("CDF(Quantile(%v)) = %v", p, got)
		}
	}
}

func TestBurrXIIFitRecoversParameters(t *testing.T) {
	truth := BurrXII{Scale: 2, ShapeC: 3, ShapeK: 0.7}
	rng := rand.New(rand.NewPCG(42, 1337))
	data := sampleBurrXII(rng, truth, 5000)

	fitted, err := BurrXIIFitter{}.Fit(data)
	if err != nil {
		t.Fatal(err)
	}
	got := fitted.(BurrXII)
	const tol = 0.1
	if relErr(got.Scale, truth.Scale) > tol || relErr(got.ShapeC, truth.ShapeC) > tol || relErr(got.ShapeK, truth.ShapeK) > tol {
		t.Errorf("Fit() = %+v, want close to %+v", got, truth)
	}
}

// Für k = 1 ist Burr XII identisch mit Log-Logistic, die MLE muss also dieselbe Log-Likelihood erreichen.
func TestBurrXIIReducesToLogLogistic(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 1))
	data := sampleLogLogistic(rng, 2, 3, 2000)

	llFit, err := LogLogisticFitter{}.Fit(data)
	if err != nil {
		t.Fatal(err)
	}
	burrFit, err := BurrXIIFitter{}.Fit(data)
	if err != nil {
		t.Fatal(err)
	}

	llMetrics, _ := EvaluateFit(llFit, data)
	burrMetrics, _ := EvaluateFit(burrFit, data)
	if burrMetrics.LogLikelihood < llMetrics.LogLikelihood-1e-6 {
		t.Errorf("BurrXII log-likelihood %v below LogLogistic %v", burrMetrics.LogLikelihood, llMetrics.LogLikelihood)
	}
}

func TestBurrXIIFitIsScaleEquivariant(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 1))
	data := sampleBurrXII(rng, BurrXII{Scale: 1, ShapeC: 2, ShapeK: 1.5}, 1000)

	base, err := BurrXIIFitter{}.Fit(data)
	if err != nil {
		t.Fatal(err)
	}
	const factor = 1e-150
	scaled := make([]float64, len(data))
	for i, x := range data {
		scaled[i] = x * factor
	}
	shifted, err := BurrXIIFitter{}.Fit(scaled)
	if err != nil {
		t.Fatal(err)
	}

	b, s := base.(BurrXII), shifted.(BurrXII)
	if relErr(s.Scale, b.Scale*factor) > 1e-6 || relErr(s.ShapeC, b.ShapeC) > 1e-6 || relErr(s.ShapeK, b.ShapeK) > 1e-6 {
		t.Errorf("scaled fit %+v not equivariant to %+v", s, b)
	}
}

func TestBurrXIIProfileGradientMatchesFiniteDifference(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 1))
	data := sampleBurrXII(rng, BurrXII{Scale: 3, ShapeC: 1.5, ShapeK: 2}, 300)
	logs := make([]float64, len(data))
	var mean float64
	for i, x := range data {
		logs[i] = math.Log(x)
		mean += logs[i]
	}
	mean /= float64(len(logs))
	profile := burrProfile{y: make([]float64, len(logs))}
	for i, l := range logs {
		profile.y[i] = l - mean
		profile.sumY += profile.y[i]
	}

	const gamma, b, h = 0.2, -0.3, 1e-6
	_, dG, dB, ok := profile.evaluate(gamma, b)
	if !ok {
		t.Fatal("evaluate() not ok at test point")
	}
	llPlusG, _, _, _ := profile.evaluate(gamma+h, b)
	llMinusG, _, _, _ := profile.evaluate(gamma-h, b)
	llPlusB, _, _, _ := profile.evaluate(gamma, b+h)
	llMinusB, _, _, _ := profile.evaluate(gamma, b-h)
	if fdG := (llPlusG - llMinusG) / (2 * h); relErr(dG, fdG) > 1e-6 {
		t.Errorf("d/dgamma = %v, finite difference %v", dG, fdG)
	}
	if fdB := (llPlusB - llMinusB) / (2 * h); relErr(dB, fdB) > 1e-6 {
		t.Errorf("d/db = %v, finite difference %v", dB, fdB)
	}
}

func TestBurrXIIFitRejectsBoundaryMLE(t *testing.T) {
	// Die Likelihood wächst hier mit k → ∞; ein Wert über dem Grenzwert wird nicht als MLE ausgegeben.
	if _, err := (BurrXIIFitter{}).Fit([]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}); !errors.Is(err, ErrFitFailed) {
		t.Errorf("err = %v, want ErrFitFailed", err)
	}
}

func TestBurrXIIFitRejectsZerosAndNegatives(t *testing.T) {
	if _, err := (BurrXIIFitter{}).Fit([]float64{0, 1, 2, 3}); !errors.Is(err, ErrFitFailed) {
		t.Errorf("zero values: err = %v, want ErrFitFailed", err)
	}
	if _, err := (BurrXIIFitter{}).Fit([]float64{-1, 1, 2, 3}); !errors.Is(err, ErrNegativeValues) {
		t.Errorf("negative values: err = %v, want ErrNegativeValues", err)
	}
}

func BenchmarkBurrXIIFit(b *testing.B) {
	for _, n := range []int{100, 10_000, 1_000_000} {
		data := sampleBurrXII(rand.New(rand.NewPCG(42, 1337)), BurrXII{Scale: 2, ShapeC: 3, ShapeK: 0.7}, n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := (BurrXIIFitter{}).Fit(data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
