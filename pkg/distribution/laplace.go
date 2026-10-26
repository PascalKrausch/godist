package distribution

import (
	"errors"
	"math"
	"sort"
)

// Laplace repräsentiert eine Laplace-Verteilung mit Location (μ) und Scale (b > 0).
type Laplace struct {
	Mu    float64 // Lageparameter (Median)
	Scale float64 // Skalierungsparameter (b > 0)
}

func (l Laplace) Quantile(p float64) float64 {
	if !l.Valid() || !validProbability(p) {
		return math.NaN()
	}
	if p < 0.5 {
		return l.Mu + l.Scale*math.Log(2*p)
	}
	return l.Mu - l.Scale*math.Log(2*(1-p))
}

func (l Laplace) Params() []float64 {
	return []float64{l.Mu, l.Scale}
}

func (l Laplace) ParamNames() []string {
	return []string{"mu", "scale"}
}

func (l Laplace) Valid() bool {
	return l.Scale > 0 && IsFinite(l.Mu) && IsFinite(l.Scale)
}

func (l Laplace) PDF(x float64) float64 {
	if !l.Valid() {
		return 0
	}
	return math.Exp(l.LogPDF(x))
}

func (l Laplace) LogPDF(x float64) float64 {
	if !l.Valid() {
		return math.Inf(-1)
	}
	// f(x) = 1 / (2b) * exp(-|x - μ| / b)
	// ln f(x) = -ln(2) - ln(b) - |x - μ| / b
	diff := math.Abs(x - l.Mu)
	return -math.Log(2.0) - math.Log(l.Scale) - (diff / l.Scale)
}

func (l Laplace) CDF(x float64) float64 {
	if !l.Valid() {
		return 0
	}
	diff := x - l.Mu
	if diff < 0 {
		return 0.5 * math.Exp(diff/l.Scale)
	}
	return 1.0 - 0.5*math.Exp(-diff/l.Scale)
}

// LaplaceFitter schätzt Mu (Median) und Scale (Mean Absolute Deviation) analytisch.
type LaplaceFitter struct{}

func (f LaplaceFitter) Name() string {
	return "Laplace"
}

func (f LaplaceFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	if len(cleaned) < 2 {
		return nil, errors.New("zu wenige Datenpunkte für Laplace-Fit")
	}

	// Daten kopieren und sortieren, um den Median zu bestimmen
	sorted := make([]float64, len(cleaned))
	copy(sorted, cleaned)
	sort.Float64s(sorted)

	n := len(sorted)
	var median float64
	if n%2 == 0 {
		median = (sorted[n/2-1] + sorted[n/2]) / 2.0
	} else {
		median = sorted[n/2]
	}

	// Skalierungsparameter b = (1 / N) * sum(|x_i - median|)
	sumAbsDev := 0.0
	for _, x := range cleaned {
		sumAbsDev += math.Abs(x - median)
	}
	scale := sumAbsDev / float64(n)

	if scale <= 0 || !IsFinite(scale) || !IsFinite(median) {
		return nil, errors.New("laplace-parameter konnten nicht geschätzt werden")
	}

	return Laplace{Mu: median, Scale: scale}, nil
}
