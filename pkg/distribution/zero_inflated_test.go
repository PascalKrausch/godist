package distribution

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestZeroInflatedFitSeparatesZerosFromBase(t *testing.T) {
	data := []float64{0, 0, 0, 0, 1, 2, 3, 4, 5, 6}
	dist, err := ZeroInflatedFitter{Base: ExponentialFitter{}}.Fit(data)
	if err != nil {
		t.Fatalf("Fit() error = %v", err)
	}
	zi, ok := dist.(ZeroInflated)
	if !ok {
		t.Fatalf("Fit() returned %T, want ZeroInflated", dist)
	}
	if math.Abs(zi.PZero-0.4) > 1e-12 {
		t.Errorf("PZero = %v, want 0.4", zi.PZero)
	}
	baseWant, err := ExponentialFitter{}.Fit([]float64{1, 2, 3, 4, 5, 6})
	if err != nil {
		t.Fatalf("base Fit() error = %v", err)
	}
	if math.Abs(zi.Base.Params()[0]-baseWant.Params()[0]) > 1e-12 {
		t.Errorf("base rate = %v, want %v (fit on positive values only)", zi.Base.Params()[0], baseWant.Params()[0])
	}
}

func TestZeroInflatedMassAndCDFConsistency(t *testing.T) {
	z := ZeroInflated{Base: Gamma{Alpha: 2, Beta: 1.5}, PZero: 0.3}

	if got := math.Exp(z.LogPDF(0)); math.Abs(got-0.3) > 1e-12 {
		t.Errorf("point mass at 0 = %v, want 0.3", got)
	}
	if got := z.CDF(0); math.Abs(got-0.3) > 1e-12 {
		t.Errorf("CDF(0) = %v, want 0.3", got)
	}
	x := 2.0
	wantLogPDF := math.Log(0.7) + z.Base.LogPDF(x)
	if got := z.LogPDF(x); math.Abs(got-wantLogPDF) > 1e-12 {
		t.Errorf("LogPDF(%v) = %v, want %v", x, got, wantLogPDF)
	}
	if got := z.CDF(1e6); math.Abs(got-1) > 1e-9 {
		t.Errorf("CDF(large) = %v, want 1", got)
	}
	if !math.IsInf(z.LogPDF(-1), -1) || z.CDF(-1) != 0 {
		t.Errorf("negative values must lie outside the support")
	}
}

func TestZeroInflatedQuantileInvertsCDF(t *testing.T) {
	z := ZeroInflated{Base: Exponential{Lambda: 0.5}, PZero: 0.4}

	if got := z.Quantile(0.3); got != 0 {
		t.Errorf("Quantile(0.3) = %v, want 0 (inside point mass)", got)
	}
	for _, x := range []float64{0.5, 1, 4} {
		p := z.CDF(x)
		if got := z.Quantile(p); math.Abs(got-x) > 1e-9 {
			t.Errorf("Quantile(CDF(%v)) = %v, want %v", x, got, x)
		}
	}
}

func TestZeroInflatedFitErrors(t *testing.T) {
	tests := []struct {
		name string
		data []float64
		want error
	}{
		{"empty", []float64{}, ErrEmptyData},
		{"no zeros", []float64{1, 2, 3}, nil},
		{"only zeros", []float64{0, 0, 0}, nil},
		{"negative", []float64{0, -1, 2}, ErrNegativeValues},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ZeroInflatedFitter{Base: GammaFitter{}}.Fit(tt.data)
			if err == nil {
				t.Fatalf("Fit() error = nil, want error")
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("Fit() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestZeroInflatedEvaluateFitCountsPZeroParameter(t *testing.T) {
	data := []float64{0, 0, 1, 2, 3, 4, 5, 6, 7, 8}
	dist, err := ZeroInflatedFitter{Base: GammaFitter{}}.Fit(data)
	if err != nil {
		t.Fatalf("Fit() error = %v", err)
	}
	metrics, err := EvaluateFit(dist, data)
	if err != nil {
		t.Fatalf("EvaluateFit() error = %v", err)
	}
	wantAIC := 2*3 - 2*metrics.LogLikelihood
	if math.Abs(metrics.AIC-wantAIC) > 1e-9 {
		t.Errorf("AIC = %v, want %v (k = 3: alpha, scale, p_zero)", metrics.AIC, wantAIC)
	}
	if math.IsInf(metrics.LogLikelihood, 0) || math.IsNaN(metrics.LogLikelihood) {
		t.Errorf("LogLikelihood = %v, want finite", metrics.LogLikelihood)
	}
}

func TestSelectAllModelsZeroInflatedPolicyOnlyRanksZIModels(t *testing.T) {
	data := []float64{0, 0, 0, 1, 2, 3, 4, 5, 6, 7}

	results, _, err := SelectAllModelsWithPolicy(data, PolicyZeroInflated)
	if err != nil {
		t.Fatalf("SelectAllModelsWithPolicy() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no ZI candidate fitted")
	}
	for _, r := range results {
		if !strings.HasPrefix(r.Name, "ZI-") {
			t.Errorf("candidate %q is not a ZI model", r.Name)
		}
	}

	fastFail, _, err := SelectAllModelsWithPolicy(data, PolicyFastFail)
	if err != nil {
		t.Fatalf("SelectAllModelsWithPolicy(FastFail) error = %v", err)
	}
	for _, r := range fastFail {
		if strings.HasPrefix(r.Name, "ZI-") {
			t.Errorf("fast-fail policy produced ZI candidate %q", r.Name)
		}
	}
}
