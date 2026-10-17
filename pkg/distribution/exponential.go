package distribution

import (
	"errors"
	"math"

	"gonum.org/v1/gonum/stat"
)

// Exponential repräsentiert eine Exponentialverteilung.
type Exponential struct {
	Lambda float64 // Rate-Parameter (λ > 0), Kehrwert des Mittelwerts
}

func (e Exponential) Params() []float64 {
	return []float64{e.Lambda}
}

func (e Exponential) ParamNames() []string {
	return []string{"rate"}
}

func (e Exponential) PDF(x float64) float64 {
	if x < 0 || e.Lambda <= 0 {
		return 0
	}
	return e.Lambda * math.Exp(-e.Lambda*x)
}

func (e Exponential) LogPDF(x float64) float64 {
	if x < 0 || e.Lambda <= 0 {
		return math.Inf(-1)
	}
	return math.Log(e.Lambda) - e.Lambda*x
}

func (e Exponential) CDF(x float64) float64 {
	if x < 0 || e.Lambda <= 0 {
		return 0
	}
	return 1.0 - math.Exp(-e.Lambda*x)
}

// ExponentialFitter schätzt Lambda per Maximum-Likelihood (MLE).
type ExponentialFitter struct{}

func (f ExponentialFitter) Name() string {
	return "Exponential"
}

func (f ExponentialFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	if err := checkNonNegative(cleaned); err != nil {
		return nil, err
	}
	if len(cleaned) < 1 {
		return nil, errors.New("zu wenige gültige Datenpunkte für Exponentialverteilung")
	}

	mu := stat.Mean(cleaned, nil)
	if mu <= 0 || !IsFinite(mu) {
		return nil, errors.New("Mittelwert der Daten ist 0 oder ungültig (λ kann nicht bestimmt werden)")
	}

	// MLE für Lambda ist 1 / Arithmetisches Mittel
	lambda := 1.0 / mu

	return Exponential{Lambda: lambda}, nil
}
