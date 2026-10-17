package distribution

import (
	"errors"
	"math"
)

// Rayleigh repräsentiert eine Rayleigh-Verteilung mit dem Skalierungsparameter Scale (σ > 0).
type Rayleigh struct {
	Scale float64 // σ > 0
}

func (r Rayleigh) Params() []float64 {
	return []float64{r.Scale}
}

func (r Rayleigh) ParamNames() []string {
	return []string{"scale"}
}

func (r Rayleigh) PDF(x float64) float64 {
	if x < 0 || r.Scale <= 0 {
		return 0
	}
	return math.Exp(r.LogPDF(x))
}

func (r Rayleigh) LogPDF(x float64) float64 {
	if x < 0 || r.Scale <= 0 {
		return math.Inf(-1)
	}
	if x == 0 {
		return math.Inf(-1)
	}
	sigmaSq := r.Scale * r.Scale
	return math.Log(x) - 2.0*math.Log(r.Scale) - (x*x)/(2.0*sigmaSq)
}

func (r Rayleigh) CDF(x float64) float64 {
	if x < 0 || r.Scale <= 0 {
		return 0
	}
	// F(x; σ) = 1 - exp(-x^2 / (2σ^2))
	sigmaSq := r.Scale * r.Scale
	return 1.0 - math.Exp(-(x*x)/(2.0*sigmaSq))
}

// RayleighFitter schätzt den Skalierungsparameter über die geschlossene MLE-Formel.
type RayleighFitter struct{}

func (f RayleighFitter) Name() string {
	return "Rayleigh"
}

func (f RayleighFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	if err := checkNonNegative(cleaned); err != nil {
		return nil, err
	}
	if len(cleaned) == 0 {
		return nil, errors.New("zu wenige gültige Datenpunkte für Rayleigh-Fit")
	}

	// Schätzung auf den positiven Werten; Nullen werden wie bei Weibull/Gamma behandelt.
	positive := make([]float64, 0, len(cleaned))
	for _, x := range cleaned {
		if x > 0 {
			positive = append(positive, x)
		}
	}
	if len(positive) == 0 {
		return nil, ErrFitFailed
	}

	sumSquares := 0.0
	for _, x := range positive {
		sumSquares += x * x
	}

	// Exakte analytische MLE-Formel für Rayleigh:
	// σ = sqrt( (1 / 2N) * sum(x_i^2) )
	n := float64(len(positive))
	scale := math.Sqrt(sumSquares / (2.0 * n))

	if scale <= 0 || !IsFinite(scale) {
		return nil, errors.New("rayleigh-parameter konnte nicht geschätzt werden")
	}

	return Rayleigh{Scale: scale}, nil
}
