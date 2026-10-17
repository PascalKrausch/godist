package distribution

import (
	"math"

	"gonum.org/v1/gonum/stat"
)

// LogNormal repräsentiert eine Log-Normal-Verteilung.
// X ~ LogNormal(μ, σ) bedeutet, dass ln(X) ~ Normal(μ, σ) ist.
type LogNormal struct {
	Mu    float64 // Mittelwert der logarithmierten Daten E[ln(X)]
	Sigma float64 // Standardabweichung der logarithmierten Daten SD[ln(X)]
}

func (l LogNormal) Params() []float64 {
	return []float64{l.Mu, l.Sigma}
}

func (l LogNormal) ParamNames() []string {
	return []string{"mu", "sigma"}
}

func (l LogNormal) PDF(x float64) float64 {
	if x <= 0 || l.Sigma <= 0 {
		return 0
	}
	return math.Exp(l.LogPDF(x))
}

func (l LogNormal) LogPDF(x float64) float64 {
	if x <= 0 || l.Sigma <= 0 {
		return math.Inf(-1)
	}
	logX := math.Log(x)
	diff := logX - l.Mu
	return -logX - math.Log(l.Sigma) - 0.5*math.Log(2*math.Pi) - (diff*diff)/(2*l.Sigma*l.Sigma)
}

func (l LogNormal) CDF(x float64) float64 {
	if x <= 0 || l.Sigma <= 0 {
		return 0
	}
	// CDF nutzt die Fehlerfunktion (Erf) der Standardnormalverteilung
	z := (math.Log(x) - l.Mu) / (l.Sigma * math.Sqrt2)
	return 0.5 * (1 + math.Erf(z))
}

// LogNormalFitter schätzt Mu und Sigma per Analytischem Maximum-Likelihood (MLE).
type LogNormalFitter struct{}

func (f LogNormalFitter) Name() string {
	return "LogNormal"
}

func (f LogNormalFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	if err := checkNonNegative(cleaned); err != nil {
		return nil, err
	}
	n := float64(len(cleaned))
	if n == 0 {
		return nil, ErrEmptyData
	}

	// Strenge Nullwert-Policy (Fast-Fail): log(0) ist -Inf, daher kein Fit.
	for _, x := range cleaned {
		if x == 0 {
			return nil, ErrFitFailed
		}
	}

	// Transformation Y = ln(X)
	logs := make([]float64, len(cleaned))
	for i, x := range cleaned {
		logs[i] = math.Log(x)
	}

	// MLE für Normalverteilung auf logarithmierten Daten (Varianz mit Divisor n)
	mu := stat.Mean(logs, nil)
	variance := stat.Variance(logs, nil) * (n - 1) / n
	sigma := math.Sqrt(variance)

	if sigma <= 0 || math.IsNaN(sigma) {
		return nil, ErrFitFailed
	}

	return LogNormal{Mu: mu, Sigma: sigma}, nil
}
