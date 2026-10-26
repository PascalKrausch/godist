package distribution

import (
	"math"
	"testing"
)

func TestDistributionFunctions(t *testing.T) {
	tests := []struct {
		name string
		dist ContinuousDistribution
		x    float64
		pdf  float64
		cdf  float64
	}{
		{
			name: "Beta",
			dist: Beta{Alpha: 2, Beta: 2, Min: 0, Max: 1},
			x:    0.5,
			pdf:  1.5,
			cdf:  0.5,
		},
		{
			name: "Exponential",
			dist: Exponential{Lambda: 2},
			x:    1,
			pdf:  2 * math.Exp(-2),
			cdf:  1 - math.Exp(-2),
		},
		{
			name: "Laplace",
			dist: Laplace{Mu: 0, Scale: 2},
			x:    2,
			pdf:  math.Exp(-1) / 4,
			cdf:  1 - 0.5*math.Exp(-1),
		},
		{
			name: "Rayleigh",
			dist: Rayleigh{Scale: 1},
			x:    1,
			pdf:  math.Exp(-0.5),
			cdf:  1 - math.Exp(-0.5),
		},
		{
			name: "Normal",
			dist: Normal{Mu: 0, Sigma: 1},
			x:    0,
			pdf:  1 / math.Sqrt(2*math.Pi),
			cdf:  0.5,
		},
		{
			name: "Gumbel",
			dist: Gumbel{Mu: 0, Beta: 1},
			x:    0,
			pdf:  1 / math.E,
			cdf:  1 / math.E,
		},
		{
			name: "Gamma",
			dist: Gamma{Alpha: 2, Beta: 3},
			x:    3,
			pdf:  1 / (3 * math.E),
			cdf:  1 - 2/math.E,
		},
		{
			name: "LogNormal",
			dist: LogNormal{Mu: 0, Sigma: 1},
			x:    1,
			pdf:  1 / math.Sqrt(2*math.Pi),
			cdf:  0.5,
		},
		{
			name: "Weibull",
			dist: Weibull{Shape: 2, Scale: 3},
			x:    3,
			pdf:  2 / (3 * math.E),
			cdf:  1 - 1/math.E,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertNear(t, "PDF", tt.dist.PDF(tt.x), tt.pdf)
			assertNear(t, "CDF", tt.dist.CDF(tt.x), tt.cdf)
			assertNear(t, "LogPDF", tt.dist.LogPDF(tt.x), math.Log(tt.pdf))
			assertNear(t, "Quantile(CDF)", tt.dist.Quantile(tt.cdf), tt.x)
			if got := len(tt.dist.Params()); got == 0 {
				t.Fatal("Params() returned no parameters")
			}
		})
	}
}

func TestDistributionQuantileBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		dist  ContinuousDistribution
		lower float64
		upper float64
	}{
		{"Beta", Beta{Alpha: 2, Beta: 2, Min: 0, Max: 1}, 0, 1},
		{"Exponential", Exponential{Lambda: 2}, 0, math.Inf(1)},
		{"Laplace", Laplace{Mu: 0, Scale: 2}, math.Inf(-1), math.Inf(1)},
		{"Rayleigh", Rayleigh{Scale: 1}, 0, math.Inf(1)},
		{"Normal", Normal{Mu: 0, Sigma: 1}, math.Inf(-1), math.Inf(1)},
		{"Gumbel", Gumbel{Mu: 0, Beta: 1}, math.Inf(-1), math.Inf(1)},
		{"Gamma", Gamma{Alpha: 2, Beta: 3}, 0, math.Inf(1)},
		{"LogNormal", LogNormal{Mu: 0, Sigma: 1}, 0, math.Inf(1)},
		{"Weibull", Weibull{Shape: 2, Scale: 3}, 0, math.Inf(1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dist.Quantile(0); got != tt.lower {
				t.Errorf("Quantile(0) = %g, want %g", got, tt.lower)
			}
			if got := tt.dist.Quantile(1); got != tt.upper {
				t.Errorf("Quantile(1) = %g, want %g", got, tt.upper)
			}
			for _, p := range []float64{-0.1, 1.1, math.NaN()} {
				if got := tt.dist.Quantile(p); !math.IsNaN(got) {
					t.Errorf("Quantile(%g) = %g, want NaN outside [0, 1]", p, got)
				}
			}
		})
	}
}

func TestDistributionQuantileRoundTrip(t *testing.T) {
	dists := []struct {
		name string
		dist ContinuousDistribution
	}{
		{"Beta", Beta{Alpha: 2, Beta: 3, Min: -1, Max: 4}},
		{"Exponential", Exponential{Lambda: 1.5}},
		{"Laplace", Laplace{Mu: 1, Scale: 2}},
		{"Rayleigh", Rayleigh{Scale: 1.5}},
		{"Normal", Normal{Mu: 1, Sigma: 2}},
		{"Gumbel", Gumbel{Mu: 1, Beta: 2}},
		{"Gamma", Gamma{Alpha: 2.5, Beta: 3}},
		{"LogNormal", LogNormal{Mu: 0, Sigma: 0.8}},
		{"Weibull", Weibull{Shape: 1.7, Scale: 3}},
	}
	probs := []float64{0.001, 0.01, 0.1, 0.25, 0.5, 0.75, 0.9, 0.99, 0.999}

	for _, tt := range dists {
		t.Run(tt.name, func(t *testing.T) {
			for _, p := range probs {
				x := tt.dist.Quantile(p)
				if math.IsNaN(x) || math.IsInf(x, 0) {
					t.Fatalf("Quantile(%g) = %g, want a finite value", p, x)
				}
				if got := tt.dist.CDF(x); math.Abs(got-p) > 1e-9 {
					t.Errorf("CDF(Quantile(%g)) = %.12g, want %g", p, got, p)
				}
			}
		})
	}
}

func TestDistributionQuantileMonotone(t *testing.T) {
	dist := Gamma{Alpha: 0.7, Beta: 2}
	previous := math.Inf(-1)
	for _, p := range []float64{0.001, 0.1, 0.2, 0.5, 0.8, 0.99} {
		q := dist.Quantile(p)
		if q <= previous {
			t.Errorf("Quantile(%g) = %g is not larger than previous quantile %g", p, q, previous)
		}
		previous = q
	}
}

func TestDistributionSupportBoundaries(t *testing.T) {
	tests := []struct {
		name string
		dist ContinuousDistribution
		x    float64
		cdf  float64
	}{
		{"Beta below minimum", Beta{Alpha: 2, Beta: 2, Min: 0, Max: 1}, -0.1, 0},
		{"Beta at minimum", Beta{Alpha: 2, Beta: 2, Min: 0, Max: 1}, 0, 0},
		{"Beta above maximum", Beta{Alpha: 2, Beta: 2, Min: 0, Max: 1}, 1.1, 1},
		{"Exponential below zero", Exponential{Lambda: 2}, -0.1, 0},
		{"Rayleigh below zero", Rayleigh{Scale: 1}, -0.1, 0},
		{"Rayleigh at zero", Rayleigh{Scale: 1}, 0, 0},
		{"Gamma at zero", Gamma{Alpha: 2, Beta: 3}, 0, 0},
		{"LogNormal at zero", LogNormal{Mu: 0, Sigma: 1}, 0, 0},
		{"Weibull at zero", Weibull{Shape: 2, Scale: 3}, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dist.PDF(tt.x); got != 0 {
				t.Errorf("PDF(%g) = %g, want 0 outside or at zero-density support boundary", tt.x, got)
			}
			if got := tt.dist.CDF(tt.x); got != tt.cdf {
				t.Errorf("CDF(%g) = %g, want %g", tt.x, got, tt.cdf)
			}
			if got := tt.dist.LogPDF(tt.x); !math.IsInf(got, -1) {
				t.Errorf("LogPDF(%g) = %g, want -Inf", tt.x, got)
			}
		})
	}
}

func TestGammaLogPDFAtZero(t *testing.T) {
	if got, want := (Gamma{Alpha: 1, Beta: 4}).LogPDF(0), -math.Log(4); math.Abs(got-want) > 1e-12 {
		t.Errorf("Gamma(α=1) LogPDF(0) = %v, want %v", got, want)
	}
	if got := (Gamma{Alpha: 2, Beta: 4}).LogPDF(0); !math.IsInf(got, -1) {
		t.Errorf("Gamma(α=2) LogPDF(0) = %v, want -Inf", got)
	}
	if got := (Gamma{Alpha: 0.5, Beta: 4}).LogPDF(0); !math.IsInf(got, 1) {
		t.Errorf("Gamma(α=0.5) LogPDF(0) = %v, want +Inf", got)
	}
}

func TestDistributionCDFProperties(t *testing.T) {
	tests := []struct {
		name string
		dist ContinuousDistribution
		xs   []float64
	}{
		{"Beta", Beta{Alpha: 2, Beta: 3, Min: 0, Max: 1}, []float64{-1, 0, 0.25, 0.5, 0.75, 1, 2}},
		{"Exponential", Exponential{Lambda: 1.5}, []float64{-1, 0, 0.25, 1, 3}},
		{"Laplace", Laplace{Mu: 1, Scale: 2}, []float64{-5, -1, 0, 1, 3, 7}},
		{"Rayleigh", Rayleigh{Scale: 1.5}, []float64{-1, 0, 0.25, 1, 3, 10}},
		{"Normal", Normal{Mu: 1, Sigma: 2}, []float64{-5, -1, 1, 3, 7}},
		{"Gumbel", Gumbel{Mu: 1, Beta: 2}, []float64{-5, -1, 1, 3, 7}},
		{"Gamma", Gamma{Alpha: 2, Beta: 3}, []float64{-1, 0, 0.25, 1, 3, 10}},
		{"LogNormal", LogNormal{Mu: 0, Sigma: 1}, []float64{-1, 0, 0.25, 1, 3, 10}},
		{"Weibull", Weibull{Shape: 2, Scale: 3}, []float64{-1, 0, 0.25, 1, 3, 10}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := 0.0
			for _, x := range tt.xs {
				cdf := tt.dist.CDF(x)
				if math.IsNaN(cdf) || cdf < 0 || cdf > 1 {
					t.Errorf("CDF(%g) = %g, want a value in [0, 1]", x, cdf)
				}
				if cdf < previous {
					t.Errorf("CDF is not monotone: CDF(%g) = %g after %g", x, cdf, previous)
				}
				pdf := tt.dist.PDF(x)
				if pdf < 0 || math.IsNaN(pdf) {
					t.Errorf("PDF(%g) = %g, want a non-negative, non-NaN value", x, pdf)
				}
				previous = cdf
			}
		})
	}
}

func TestDistributionRejectsInvalidParameters(t *testing.T) {
	tests := []struct {
		name string
		dist ContinuousDistribution
	}{
		{"Beta", Beta{Alpha: 2, Beta: 2, Min: 1, Max: 1}},
		{"Exponential", Exponential{Lambda: 0}},
		{"Laplace", Laplace{Mu: 0, Scale: 0}},
		{"Rayleigh", Rayleigh{Scale: 0}},
		{"Normal", Normal{Mu: 0, Sigma: 0}},
		{"Gumbel", Gumbel{Mu: 0, Beta: 0}},
		{"Gamma", Gamma{Alpha: 2, Beta: 0}},
		{"LogNormal", LogNormal{Mu: 0, Sigma: 0}},
		{"Weibull", Weibull{Shape: 2, Scale: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dist.PDF(1); got != 0 {
				t.Errorf("PDF(1) = %g, want 0 for invalid parameters", got)
			}
			if got := tt.dist.CDF(1); got != 0 {
				t.Errorf("CDF(1) = %g, want 0 for invalid parameters", got)
			}
			if got := tt.dist.LogPDF(1); !math.IsInf(got, -1) {
				t.Errorf("LogPDF(1) = %g, want -Inf for invalid parameters", got)
			}
			if got := tt.dist.Quantile(0.5); !math.IsNaN(got) {
				t.Errorf("Quantile(0.5) = %g, want NaN for invalid parameters", got)
			}
		})
	}
}

func assertNear(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.IsNaN(got) || math.Abs(got-want) > 1e-12*(1+math.Abs(want)) {
		t.Errorf("%s = %.16g, want %.16g", label, got, want)
	}
}
