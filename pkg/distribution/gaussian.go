package distribution

import (
	"errors"
	"math"

	"gonum.org/v1/gonum/stat"
)

// Normal repräsentiert eine Gauß'sche Normalverteilung.
type Normal struct {
	Mu    float64 // Mittelwert (μ)
	Sigma float64 // Standardabweichung (σ > 0)
}

func (n Normal) Params() []float64 {
	return []float64{n.Mu, n.Sigma}
}

func (n Normal) PDF(x float64) float64 {
	if n.Sigma <= 0 {
		return 0
	}
	return math.Exp(n.LogPDF(x))
}

func (n Normal) LogPDF(x float64) float64 {
	if n.Sigma <= 0 {
		return math.Inf(-1)
	}
	diff := x - n.Mu
	return -0.5*math.Log(2*math.Pi) - math.Log(n.Sigma) - (diff*diff)/(2*n.Sigma*n.Sigma)
}

func (n Normal) CDF(x float64) float64 {
	if n.Sigma <= 0 {
		return 0
	}
	return 0.5 * (1 + math.Erf((x-n.Mu)/(n.Sigma*math.Sqrt2)))
}

// NormalFitter schätzt Mu und Sigma per Maximum-Likelihood (MLE).
type NormalFitter struct{}

func (f NormalFitter) Name() string {
	return "Normal"
}

func (f NormalFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanData(data)
	if len(cleaned) < 2 {
		return nil, errors.New("zu wenige gültige Datenpunkte für Normalverteilung")
	}

	mu := stat.Mean(cleaned, nil)
	variance := stat.Variance(cleaned, nil)
	sigma := math.Sqrt(variance)

	if sigma <= 0 || !IsFinite(sigma) {
		return nil, errors.New("varianz der Daten ist 0 oder ungültig")
	}

	return Normal{Mu: mu, Sigma: sigma}, nil
}
