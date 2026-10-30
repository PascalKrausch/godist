package distribution

import (
	"math"
	"testing"
)

// Numerische Stabilitätstests für Extremwerte und Grenzfälle.

func TestNumericalStabilityVerySmallValues(t *testing.T) {
	tests := []struct {
		name   string
		fitter Fitter
		data   []float64
	}{
		{
			name:   "Exponential with very small values",
			fitter: ExponentialFitter{},
			data:   []float64{1e-10, 2e-10, 3e-10, 4e-10, 5e-10, 1e-9, 2e-9, 5e-9},
		},
		{
			name:   "Gamma with very small values",
			fitter: GammaFitter{},
			data:   []float64{1e-10, 2e-10, 3e-10, 4e-10, 5e-10, 1e-9, 2e-9, 5e-9},
		},
		{
			name:   "LogNormal with very small values",
			fitter: LogNormalFitter{},
			data:   []float64{1e-8, 2e-8, 3e-8, 4e-8, 5e-8, 1e-7, 2e-7, 5e-7},
		},
		{
			name:   "Weibull with very small values",
			fitter: WeibullFitter{},
			data:   []float64{1e-10, 2e-10, 3e-10, 4e-10, 5e-10, 1e-9, 2e-9, 5e-9},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist, err := tt.fitter.Fit(tt.data)
			if err != nil {
				t.Logf("Fit() with very small values failed (may be expected): %v", err)
				return
			}

			// Überprüfe, dass Parameter endlich sind
			params := dist.Params()
			for i, p := range params {
				if math.IsNaN(p) || math.IsInf(p, 0) {
					t.Errorf("Params()[%d] = %g, want finite value", i, p)
				}
				// Mu bei LogNormal kann negativ sein, andere Parameter sollten positiv sein
				if tt.name == "LogNormal with very small values" && i == 0 {
					// Mu darf negativ sein
					continue
				}
				if p < 0 {
					t.Errorf("Params()[%d] = %g, want non-negative", i, p)
				}
			}

			// Überprüfe, dass PDF/CDF/LogPDF finite Werte liefern
			for _, x := range tt.data {
				pdf := dist.PDF(x)
				cdf := dist.CDF(x)
				logPdf := dist.LogPDF(x)

				if math.IsNaN(pdf) || math.IsInf(pdf, 0) {
					t.Errorf("PDF(%g) = %g, want finite value", x, pdf)
				}
				if math.IsNaN(cdf) || math.IsInf(cdf, 0) {
					t.Errorf("CDF(%g) = %g, want finite value", x, cdf)
				}
				// LogPDF darf -Inf sein, aber nicht NaN oder +Inf
				if math.IsNaN(logPdf) || math.IsInf(logPdf, 1) {
					t.Errorf("LogPDF(%g) = %g, want finite or -Inf", x, logPdf)
				}
			}
		})
	}
}

func TestNumericalStabilityVeryLargeValues(t *testing.T) {
	tests := []struct {
		name   string
		fitter Fitter
		data   []float64
	}{
		{
			name:   "Exponential with very large values",
			fitter: ExponentialFitter{},
			data:   []float64{1e8, 2e8, 3e8, 4e8, 5e8, 1e9, 2e9, 5e9},
		},
		{
			name:   "Gamma with very large values",
			fitter: GammaFitter{},
			data:   []float64{1e8, 2e8, 3e8, 4e8, 5e8, 1e9, 2e9, 5e9},
		},
		{
			name:   "LogNormal with very large values",
			fitter: LogNormalFitter{},
			data:   []float64{1e8, 2e8, 3e8, 4e8, 5e8, 1e9, 2e9, 5e9},
		},
		{
			name:   "Weibull with very large values",
			fitter: WeibullFitter{},
			data:   []float64{1e8, 2e8, 3e8, 4e8, 5e8, 1e9, 2e9, 5e9},
		},
		{
			name:   "Normal with very large values",
			fitter: NormalFitter{},
			data:   []float64{1e8, 2e8, 3e8, 4e8, 5e8, 1e9, 2e9, 5e9},
		},
		{
			name:   "Gumbel with very large values",
			fitter: GumbelFitter{},
			data:   []float64{1e8, 2e8, 3e8, 4e8, 5e8, 1e9, 2e9, 5e9},
		},
		{
			name:   "Logistic with very large values",
			fitter: LogisticFitter{},
			data:   []float64{1e8, 2e8, 3e8, 4e8, 5e8, 1e9, 2e9, 5e9},
		},
		{
			name:   "Pareto with very large values",
			fitter: ParetoFitter{},
			data:   []float64{1e8, 2e8, 3e8, 4e8, 5e8, 1e9, 2e9, 5e9},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist, err := tt.fitter.Fit(tt.data)
			if err != nil {
				t.Logf("Fit() with very large values failed (may be expected): %v", err)
				return
			}

			// Überprüfe, dass Parameter endlich sind
			params := dist.Params()
			for i, p := range params {
				if math.IsNaN(p) || math.IsInf(p, 0) {
					t.Errorf("Params()[%d] = %g, want finite value", i, p)
				}
			}

			// Überprüfe, dass PDF/CDF/LogPDF finite Werte liefern
			for _, x := range tt.data {
				pdf := dist.PDF(x)
				cdf := dist.CDF(x)
				logPdf := dist.LogPDF(x)

				if math.IsNaN(pdf) || math.IsInf(pdf, 0) {
					t.Errorf("PDF(%g) = %g, want finite value", x, pdf)
				}
				if math.IsNaN(cdf) || math.IsInf(cdf, 0) {
					t.Errorf("CDF(%g) = %g, want finite value", x, cdf)
				}
				if math.IsNaN(logPdf) || math.IsInf(logPdf, 1) {
					t.Errorf("LogPDF(%g) = %g, want finite or -Inf", x, logPdf)
				}
			}
		})
	}
}

func TestNumericalStabilityWideRangeValues(t *testing.T) {
	// Datensatz mit sehr großer Spanne (6 Größenordnungen)
	data := []float64{1e-6, 1e-4, 1e-2, 1, 1e2, 1e4, 1e6}

	tests := []struct {
		name   string
		fitter Fitter
	}{
		{"LogNormal", LogNormalFitter{}},
		{"Gamma", GammaFitter{}},
		{"Weibull", WeibullFitter{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist, err := tt.fitter.Fit(data)
			if err != nil {
				t.Logf("Fit() with wide range values failed (may be expected): %v", err)
				return
			}

			params := dist.Params()
			for i, p := range params {
				if math.IsNaN(p) || math.IsInf(p, 0) {
					t.Errorf("Params()[%d] = %g, want finite value", i, p)
				}
			}

			// Teste PDF/CDF über den gesamten Bereich
			testPoints := []float64{1e-7, 1e-5, 1e-3, 0.1, 10, 1e3, 1e5, 1e7}
			for _, x := range testPoints {
				pdf := dist.PDF(x)
				cdf := dist.CDF(x)
				logPdf := dist.LogPDF(x)

				if math.IsNaN(pdf) || math.IsInf(pdf, 0) {
					t.Errorf("PDF(%g) = %g, want finite value", x, pdf)
				}
				if math.IsNaN(cdf) || math.IsInf(cdf, 0) {
					t.Errorf("CDF(%g) = %g, want finite value", x, cdf)
				}
				if math.IsNaN(logPdf) || math.IsInf(logPdf, 1) {
					t.Errorf("LogPDF(%g) = %g, want finite or -Inf", x, logPdf)
				}
			}
		})
	}
}

// Teste die numerische Stabilität bei Parametern nahe den Grenzen des Definitionsbereichs.
func TestNumericalStabilityParametersNearBoundaries(t *testing.T) {
	tests := []struct {
		name string
		dist ContinuousDistribution
	}{
		{
			name: "Exponential with very small lambda",
			dist: Exponential{Lambda: 1e-10},
		},
		{
			name: "Exponential with very large lambda",
			dist: Exponential{Lambda: 1e10},
		},
		{
			name: "Normal with very small sigma",
			dist: Normal{Mu: 0, Sigma: 1e-10},
		},
		{
			name: "Normal with very large sigma",
			dist: Normal{Mu: 0, Sigma: 1e10},
		},
		{
			name: "Gamma with very small alpha",
			dist: Gamma{Alpha: 1e-6, Beta: 1},
		},
		{
			name: "Gamma with very large alpha",
			dist: Gamma{Alpha: 1e6, Beta: 1},
		},
		{
			name: "Weibull with very small shape",
			dist: Weibull{Shape: 1e-6, Scale: 1},
		},
		{
			name: "Weibull with very large shape",
			dist: Weibull{Shape: 100, Scale: 1},
		},
		{
			name: "LogNormal with very small sigma",
			dist: LogNormal{Mu: 0, Sigma: 1e-10},
		},
		{
			name: "LogNormal with very large sigma",
			dist: LogNormal{Mu: 0, Sigma: 1e10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Teste an repräsentativen Punkten
			testPoints := []float64{0.1, 1, 10, 100}

			for _, x := range testPoints {
				pdf := tt.dist.PDF(x)
				cdf := tt.dist.CDF(x)
				logPdf := tt.dist.LogPDF(x)

				// PDF darf 0 sein, sollte aber nicht NaN/Inf sein
				if math.IsNaN(pdf) || math.IsInf(pdf, 0) {
					t.Errorf("PDF(%g) = %g, want finite or 0", x, pdf)
				}
				// CDF sollte im Bereich [0, 1] liegen
				if math.IsNaN(cdf) || math.IsInf(cdf, 0) || cdf < 0 || cdf > 1 {
					t.Errorf("CDF(%g) = %g, want value in [0, 1]", x, cdf)
				}
				// LogPDF darf -Inf sein, sollte aber nicht NaN/+Inf sein
				if math.IsNaN(logPdf) || math.IsInf(logPdf, 1) {
					t.Errorf("LogPDF(%g) = %g, want finite or -Inf", x, logPdf)
				}
			}
		})
	}
}

// Teste die numerische Stabilität bei Überlauf/Unterlauf
func TestNumericalStabilityOverflowUnderflow(t *testing.T) {
	tests := []struct {
		name string
		dist ContinuousDistribution
		x    float64
	}{
		{
			name: "Normal PDF far in the tail",
			dist: Normal{Mu: 0, Sigma: 1},
			x:    100,
		},
		{
			name: "Exponential PDF with large x",
			dist: Exponential{Lambda: 10},
			x:    100,
		},
		{
			name: "Gamma PDF with large x",
			dist: Gamma{Alpha: 2, Beta: 1},
			x:    1000,
		},
		{
			name: "Weibull PDF with large x",
			dist: Weibull{Shape: 2, Scale: 1},
			x:    1000,
		},
		{
			name: "LogNormal PDF with large x",
			dist: LogNormal{Mu: 0, Sigma: 1},
			x:    1e10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf := tt.dist.PDF(tt.x)
			cdf := tt.dist.CDF(tt.x)
			logPdf := tt.dist.LogPDF(tt.x)

			// PDF sollte 0 oder sehr klein sein, aber nicht NaN/Inf
			if math.IsNaN(pdf) || math.IsInf(pdf, 0) {
				t.Errorf("PDF(%g) = %g, want finite or 0", tt.x, pdf)
			}
			// CDF sollte im Bereich [0, 1] liegen
			if math.IsNaN(cdf) || math.IsInf(cdf, 0) || cdf < 0 || cdf > 1 {
				t.Errorf("CDF(%g) = %g, want value in [0, 1]", tt.x, cdf)
			}
			// LogPDF sollte -Inf oder sehr negativ sein, aber nicht NaN/+Inf
			if math.IsNaN(logPdf) || math.IsInf(logPdf, 1) {
				t.Errorf("LogPDF(%g) = %g, want finite or -Inf", tt.x, logPdf)
			}
		})
	}
}

// Teste die numerische Stabilität bei 0-Werten
func TestNumericalStabilityZeroValues(t *testing.T) {
	// Teste, dass 0-Werte korrekt behandelt werden
	data := []float64{0, 0.1, 0.5, 1, 2, 5, 10}

	tests := []struct {
		name   string
		fitter Fitter
	}{
		{"Exponential", ExponentialFitter{}},
		{"Gamma", GammaFitter{}},
		{"LogNormal", LogNormalFitter{}},
		{"Weibull", WeibullFitter{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist, err := tt.fitter.Fit(data)
			if err != nil {
				t.Logf("Fit() with zero values failed (may be expected): %v", err)
				return
			}

			// Teste PDF/CDF/LogPDF bei x=0
			pdf := dist.PDF(0)
			cdf := dist.CDF(0)
			logPdf := dist.LogPDF(0)

			// Weibull mit Shape < 1 hat eine Singularität bei 0 (PDF = +Inf, LogPDF = +Inf)
			// Das ist mathematisch korrekt und wird akzeptiert
			if tt.name != "Weibull" {
				if math.IsNaN(pdf) || math.IsInf(pdf, 0) {
					t.Errorf("PDF(0) = %g, want finite or 0", pdf)
				}
				if math.IsNaN(logPdf) || math.IsInf(logPdf, 1) {
					t.Errorf("LogPDF(0) = %g, want finite or -Inf", logPdf)
				}
			}
			if math.IsNaN(cdf) || math.IsInf(cdf, 0) || cdf < 0 || cdf > 1 {
				t.Errorf("CDF(0) = %g, want value in [0, 1]", cdf)
			}
		})
	}
}

func TestNumericalStabilityNearZeroParameters(t *testing.T) {
	// Teste Parameterschätzung mit Daten nahe 0
	data := []float64{1e-10, 2e-10, 3e-10, 4e-10, 5e-10}

	tests := []struct {
		name   string
		fitter Fitter
	}{
		{"Exponential", ExponentialFitter{}},
		{"Gamma", GammaFitter{}},
		{"LogNormal", LogNormalFitter{}},
		{"Weibull", WeibullFitter{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist, err := tt.fitter.Fit(data)
			if err != nil {
				t.Logf("Fit() with near-zero data failed (may be expected): %v", err)
				return
			}

			params := dist.Params()
			for i, p := range params {
				if math.IsNaN(p) || math.IsInf(p, 0) {
					t.Errorf("Params()[%d] = %g, want finite value", i, p)
				}
			}
		})
	}
}
