package distribution

import (
	"math"
)

// LogLogistic repräsentiert eine Log-logistische Verteilung mit Skalierung (α > 0) und Form (β > 0).
// X ~ LogLogistic(α, β) bedeutet, dass ln(X) ~ Logistic(μ = ln α, s = 1/β) ist.
type LogLogistic struct {
	Scale float64 // Skalierungsparameter α (Median)
	Shape float64 // Formparameter β (s > 0)
}

// logistic liefert die Logistic-Verteilung von ln(X), auf der alle Methoden aufbauen.
func (l LogLogistic) logistic() Logistic {
	return Logistic{Mu: math.Log(l.Scale), Scale: 1 / l.Shape}
}

func (l LogLogistic) Quantile(p float64) float64 {
	if !l.Valid() || !validProbability(p) {
		return math.NaN()
	}
	return math.Exp(l.logistic().Quantile(p))
}

func (l LogLogistic) Params() []float64 {
	return []float64{l.Scale, l.Shape}
}

func (l LogLogistic) ParamNames() []string {
	return []string{"scale", "shape"}
}

func (l LogLogistic) Valid() bool {
	return l.logistic().Valid()
}

func (l LogLogistic) PDF(x float64) float64 {
	if !l.Valid() {
		return 0
	}
	return math.Exp(l.LogPDF(x))
}

func (l LogLogistic) LogPDF(x float64) float64 {
	if x <= 0 || !l.Valid() {
		return math.Inf(-1)
	}
	// Jacobi-Faktor 1/x für die Transformation von ln(X) zurück auf X
	return l.logistic().LogPDF(math.Log(x)) - math.Log(x)
}

func (l LogLogistic) CDF(x float64) float64 {
	if x <= 0 || !l.Valid() {
		return 0
	}
	return l.logistic().CDF(math.Log(x))
}

// LogLogisticFitter schätzt Scale und Shape per Maximum-Likelihood.
type LogLogisticFitter struct{}

func (f LogLogisticFitter) Name() string {
	return "LogLogistic"
}

// Fit nutzt, dass ln(X) logistisch verteilt ist: Die MLE der Logistic-Verteilung auf den logarithmierten
// Daten ergibt direkt die MLE von LogLogistic (μ → α = e^μ, s → β = 1/s).
func (f LogLogisticFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	if err := checkNonNegative(cleaned); err != nil {
		return nil, err
	}
	if len(cleaned) == 0 {
		return nil, ErrEmptyData
	}

	// Strenge Nullwert-Policy (Fast-Fail): log(0) ist -Inf, daher kein Fit.
	logs := make([]float64, len(cleaned))
	for i, x := range cleaned {
		if x == 0 {
			return nil, ErrFitFailed
		}
		logs[i] = math.Log(x)
	}

	fitted, err := LogisticFitter{}.Fit(logs)
	if err != nil {
		return nil, err
	}
	logistic := fitted.(Logistic)
	return LogLogistic{Scale: math.Exp(logistic.Mu), Shape: 1 / logistic.Scale}, nil
}
