package distribution

import (
	"math"
	"testing"
)

func TestFittersFitValidData(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	withInvalidValues := []float64{1, 2, math.NaN(), 3, math.Inf(1), 4, -1, 5, 6, 7, 8, 9, 10}
	withNonFiniteOnly := []float64{1, 2, math.NaN(), 3, math.Inf(1), 4, 5, 6, 7, 8, 9, 10}
	tests := []struct {
		name          string
		fitter        Fitter
		paramCount    int
		negativeValid bool
	}{
		{"Beta", BetaFitter{}, 4, false},
		{"Exponential", ExponentialFitter{}, 1, false},
		{"Normal", NormalFitter{}, 2, true},
		{"Gumbel", GumbelFitter{}, 2, true},
		{"Gamma", GammaFitter{}, 2, false},
		{"LogNormal", LogNormalFitter{}, 2, false},
		{"Weibull", WeibullFitter{}, 2, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist, err := tt.fitter.Fit(data)
			if err != nil {
				t.Fatalf("%s.Fit() error = %v", tt.name, err)
			}
			if len(dist.Params()) != tt.paramCount {
				t.Fatalf("%s.Fit() returned %d parameters, want %d", tt.name, len(dist.Params()), tt.paramCount)
			}
			for i, param := range dist.Params() {
				if math.IsNaN(param) || math.IsInf(param, 0) {
					t.Errorf("parameter %d = %g, want a finite value", i, param)
				}
			}

			metrics, err := EvaluateFit(dist, data)
			if err != nil {
				t.Fatalf("EvaluateFit() error = %v", err)
			}
			if metrics.SampleSize != len(data) || math.IsNaN(metrics.LogLikelihood) || math.IsInf(metrics.LogLikelihood, 0) {
				t.Errorf("EvaluateFit() returned invalid metrics: %+v", metrics)
			}

			cleanedInput := withInvalidValues
			if tt.negativeValid {
				cleanedInput = withNonFiniteOnly
			}
			filteredDist, err := tt.fitter.Fit(cleanedInput)
			if err != nil {
				t.Fatalf("%s.Fit() with values removed by CleanData returned error: %v", tt.name, err)
			}
			for i, want := range dist.Params() {
				got := filteredDist.Params()[i]
				if math.IsNaN(got) || math.Abs(got-want) > 1e-12*(1+math.Abs(want)) {
					t.Errorf("parameter after cleaning[%d] = %.16g, want %.16g", i, got, want)
				}
			}
		})
	}
}

func TestFittersRejectInsufficientOrInvalidData(t *testing.T) {
	tests := []struct {
		name   string
		fitter Fitter
		data   []float64
	}{
		{"Beta needs at least three observations", BetaFitter{}, []float64{1, 2}},
		{"Exponential needs a positive mean", ExponentialFitter{}, []float64{0}},
		{"Normal needs variation", NormalFitter{}, []float64{1}},
		{"Gumbel needs at least two observations", GumbelFitter{}, []float64{1}},
		{"Gamma needs varying observations", GammaFitter{}, []float64{1}},
		{"LogNormal needs varying observations", LogNormalFitter{}, []float64{1}},
		{"Weibull needs at least two observations", WeibullFitter{}, []float64{1}},
		{"Beta rejects values on specified boundaries", BetaFitter{Min: 0, Max: 1}, []float64{0, 0.3, 0.8}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist, err := tt.fitter.Fit(tt.data)
			if err == nil {
				t.Fatalf("Fit() = %v, want an error", dist)
			}
			if dist != nil {
				t.Errorf("Fit() returned a distribution with an error: %v", dist)
			}
		})
	}
}

func TestExponentialFitterRate(t *testing.T) {
	dist, err := (ExponentialFitter{}).Fit([]float64{1, 2, 3, 4})
	if err != nil {
		t.Fatalf("Fit() error = %v", err)
	}
	got, ok := dist.(Exponential)
	if !ok {
		t.Fatalf("Fit() returned %T, want Exponential", dist)
	}
	want := 0.4
	if math.IsNaN(got.Lambda) || math.Abs(got.Lambda-want) > 1e-12*(1+math.Abs(want)) {
		t.Errorf("rate = %.16g, want %.16g", got.Lambda, want)
	}
}
