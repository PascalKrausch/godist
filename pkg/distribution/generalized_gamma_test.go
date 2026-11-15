package distribution

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
)

// sampleGeneralizedGamma per Inversionsmethode über die Quantilfunktion.
func sampleGeneralizedGamma(rng *rand.Rand, dist GeneralizedGamma, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = dist.Quantile(rng.Float64())
	}
	return out
}

// Referenzwerte stammen aus scipy.stats.gengamma(a=shape, c=power, scale=scale).
func TestGeneralizedGammaMatchesScipyReference(t *testing.T) {
	tests := []struct {
		name        string
		dist        GeneralizedGamma
		x, pdf      float64
		logPDF, cdf float64
	}{
		{"positive power", GeneralizedGamma{Shape: 2, Scale: 3, Power: 0.7}, 1.0, 0.094590786226929222, -2.3581952048460559, 0.079333553673155352},
		{"negative power", GeneralizedGamma{Shape: 1.5, Scale: 0.8, Power: -0.6}, 1.0, 0.23094810636496296, -1.4655622414681568, 0.62601201211537982},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dist.PDF(tt.x); relErr(got, tt.pdf) > 1e-12 {
				t.Errorf("PDF(%v) = %v, want %v", tt.x, got, tt.pdf)
			}
			if got := tt.dist.LogPDF(tt.x); relErr(got, tt.logPDF) > 1e-12 {
				t.Errorf("LogPDF(%v) = %v, want %v", tt.x, got, tt.logPDF)
			}
			if got := tt.dist.CDF(tt.x); relErr(got, tt.cdf) > 1e-12 {
				t.Errorf("CDF(%v) = %v, want %v", tt.x, got, tt.cdf)
			}
		})
	}
}

func TestGeneralizedGammaQuantileRoundTripInTails(t *testing.T) {
	for _, dist := range []GeneralizedGamma{
		{Shape: 2, Scale: 1e-150, Power: 0.4},
		{Shape: 1.5, Scale: 1e150, Power: -0.8},
	} {
		for _, p := range []float64{1e-10, 1e-4, 0.5, 1 - 1e-4, 1 - 1e-10} {
			x := dist.Quantile(p)
			if math.IsNaN(x) || x <= 0 {
				t.Fatalf("Quantile(%v) of %+v = %v, want positive finite value", p, dist, x)
			}
			if got := dist.CDF(x); relErr(got, p) > 1e-8 {
				t.Errorf("CDF(Quantile(%v)) of %+v = %v", p, dist, got)
			}
		}
	}
}

func TestGeneralizedGammaFitRecoversParameters(t *testing.T) {
	for _, truth := range []GeneralizedGamma{
		{Shape: 2, Scale: 1.5, Power: 0.8},
		{Shape: 1.5, Scale: 2, Power: -0.7},
	} {
		rng := rand.New(rand.NewPCG(42, 1337))
		data := sampleGeneralizedGamma(rng, truth, 5000)

		fitted, err := GeneralizedGammaFitter{}.Fit(data)
		if err != nil {
			t.Fatalf("Fit(%+v) error = %v", truth, err)
		}
		got := fitted.(GeneralizedGamma)
		const tol = 0.15
		if relErr(got.Shape, truth.Shape) > tol || relErr(got.Scale, truth.Scale) > tol || relErr(got.Power, truth.Power) > tol {
			t.Errorf("Fit() = %+v, want close to %+v", got, truth)
		}
	}
}

// Die GG enthält Gamma (power = 1) und Weibull (shape = 1) als Spezialfälle;
// der MLE muss daher mindestens deren Likelihood erreichen.
func TestGeneralizedGammaFitNestsGammaAndWeibull(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 1))

	gammaData := sampleGammaInt(rng, 2, 3, 2000)
	gammaFit, err := GammaFitter{}.Fit(gammaData)
	if err != nil {
		t.Fatal(err)
	}
	ggFit, err := GeneralizedGammaFitter{}.Fit(gammaData)
	if err != nil {
		t.Fatal(err)
	}
	gammaMetrics, _ := EvaluateFit(gammaFit, gammaData)
	ggMetrics, _ := EvaluateFit(ggFit, gammaData)
	if ggMetrics.LogLikelihood < gammaMetrics.LogLikelihood-1e-6 {
		t.Errorf("GG log-likelihood %v below Gamma %v on gamma data", ggMetrics.LogLikelihood, gammaMetrics.LogLikelihood)
	}

	weibullData := sampleWeibull(rng, 1.5, 3, 2000)
	weibullFit, err := WeibullFitter{}.Fit(weibullData)
	if err != nil {
		t.Fatal(err)
	}
	ggFit2, err := GeneralizedGammaFitter{}.Fit(weibullData)
	if err != nil {
		t.Fatal(err)
	}
	weibullMetrics, _ := EvaluateFit(weibullFit, weibullData)
	ggMetrics2, _ := EvaluateFit(ggFit2, weibullData)
	if ggMetrics2.LogLikelihood < weibullMetrics.LogLikelihood-1e-6 {
		t.Errorf("GG log-likelihood %v below Weibull %v on weibull data", ggMetrics2.LogLikelihood, weibullMetrics.LogLikelihood)
	}
}

func TestGeneralizedGammaFitIsScaleEquivariant(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 1))
	data := sampleGeneralizedGamma(rng, GeneralizedGamma{Shape: 1.8, Scale: 1, Power: 0.9}, 1000)

	base, err := GeneralizedGammaFitter{}.Fit(data)
	if err != nil {
		t.Fatal(err)
	}
	const factor = 1e-150
	scaled := make([]float64, len(data))
	for i, x := range data {
		scaled[i] = x * factor
	}
	shifted, err := GeneralizedGammaFitter{}.Fit(scaled)
	if err != nil {
		t.Fatal(err)
	}

	b, s := base.(GeneralizedGamma), shifted.(GeneralizedGamma)
	if relErr(s.Scale, b.Scale*factor) > 1e-6 || relErr(s.Shape, b.Shape) > 1e-6 || relErr(s.Power, b.Power) > 1e-6 {
		t.Errorf("scaled fit %+v not equivariant to %+v", s, b)
	}
}

func TestGeneralizedGammaProfileGradientMatchesFiniteDifference(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 1))
	data := sampleGeneralizedGamma(rng, GeneralizedGamma{Shape: 2, Scale: 3, Power: 0.7}, 300)
	logs := make([]float64, len(data))
	var mean float64
	for i, x := range data {
		logs[i] = math.Log(x)
		mean += logs[i]
	}
	mean /= float64(len(logs))
	profile := ggProfile{z: make([]float64, len(logs)), n: float64(len(logs)), meanLog: mean}
	for i, l := range logs {
		profile.z[i] = l - mean
	}

	const alpha, gamma, h = 0.5, -0.2, 1e-6
	_, dA, dG, _, ok := profile.eval(1, alpha, gamma)
	if !ok {
		t.Fatal("eval() not ok at test point")
	}
	phiPlusA, _, _, _, _ := profile.eval(1, alpha+h, gamma)
	phiMinusA, _, _, _, _ := profile.eval(1, alpha-h, gamma)
	phiPlusG, _, _, _, _ := profile.eval(1, alpha, gamma+h)
	phiMinusG, _, _, _, _ := profile.eval(1, alpha, gamma-h)
	if fdA := (phiPlusA - phiMinusA) / (2 * h); relErr(dA, fdA) > 1e-6 {
		t.Errorf("d/dalpha = %v, finite difference %v", dA, fdA)
	}
	if fdG := (phiPlusG - phiMinusG) / (2 * h); relErr(dG, fdG) > 1e-6 {
		t.Errorf("d/dgamma = %v, finite difference %v", dG, fdG)
	}
}

func TestGeneralizedGammaFitRejectsZerosAndNegatives(t *testing.T) {
	if _, err := (GeneralizedGammaFitter{}).Fit([]float64{0, 1, 2, 3}); !errors.Is(err, ErrFitFailed) {
		t.Errorf("zero values: err = %v, want ErrFitFailed", err)
	}
	if _, err := (GeneralizedGammaFitter{}).Fit([]float64{-1, 1, 2, 3}); !errors.Is(err, ErrNegativeValues) {
		t.Errorf("negative values: err = %v, want ErrNegativeValues", err)
	}
	if _, err := (GeneralizedGammaFitter{}).Fit([]float64{2, 2, 2}); !errors.Is(err, ErrFitFailed) {
		t.Errorf("constant data: err = %v, want ErrFitFailed", err)
	}
}

func BenchmarkGeneralizedGammaFit(b *testing.B) {
	for _, n := range []int{100, 10_000, 1_000_000} {
		data := sampleGeneralizedGamma(rand.New(rand.NewPCG(42, 1337)), GeneralizedGamma{Shape: 2, Scale: 1.5, Power: 0.8}, n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := (GeneralizedGammaFitter{}).Fit(data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
