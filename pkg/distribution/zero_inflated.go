package distribution

import (
	"errors"
	"math"
)

// ZeroInflated mischt eine Punktmasse bei 0 mit einer Basisverteilung auf (0, ∞):
// P(X = 0) = PZero, für x > 0 gilt die Dichte (1 - PZero) · Base.PDF(x).
//
// PDF und LogPDF verwenden das gemischte Maß (Zähldichte bei 0, Dichte für x > 0),
// damit EvaluateFit die Likelihood ohne Sonderfall berechnen kann.
type ZeroInflated struct {
	Base  ContinuousDistribution
	PZero float64
}

func (z ZeroInflated) valid() bool {
	return z.Base != nil && z.PZero >= 0 && z.PZero < 1
}

// Quantile liefert 0 für p <= PZero, sonst das Quantil der Basisverteilung zum bedingten Anteil.
func (z ZeroInflated) Quantile(p float64) float64 {
	if !z.valid() || !validProbability(p) {
		return math.NaN()
	}
	if p <= z.PZero {
		return 0
	}
	return z.Base.Quantile((p - z.PZero) / (1 - z.PZero))
}

func (z ZeroInflated) Params() []float64 {
	return append(append([]float64{}, z.Base.Params()...), z.PZero)
}

func (z ZeroInflated) ParamNames() []string {
	return append(append([]string{}, z.Base.ParamNames()...), "p_zero")
}

func (z ZeroInflated) PDF(x float64) float64 {
	return math.Exp(z.LogPDF(x))
}

func (z ZeroInflated) LogPDF(x float64) float64 {
	if !z.valid() || x < 0 {
		return math.Inf(-1)
	}
	if x == 0 {
		return math.Log(z.PZero)
	}
	return math.Log(1-z.PZero) + z.Base.LogPDF(x)
}

func (z ZeroInflated) CDF(x float64) float64 {
	if !z.valid() || x < 0 {
		return 0
	}
	return z.PZero + (1-z.PZero)*z.Base.CDF(x)
}

// freeParamCount zählt nur die frei geschätzten Parameter der Basis plus PZero.
func (z ZeroInflated) freeParamCount() int {
	return freeParamCount(z.Base) + 1
}

// ZeroInflatedFitter schätzt PZero als Anteil der Nullen (MLE) und die Basisverteilung
// per Base auf den positiven Werten.
type ZeroInflatedFitter struct {
	Base Fitter
}

func (f ZeroInflatedFitter) Name() string {
	return "ZI-" + f.Base.Name()
}

func (f ZeroInflatedFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	if len(cleaned) == 0 {
		return nil, ErrEmptyData
	}
	if err := checkNonNegative(cleaned); err != nil {
		return nil, err
	}

	positives := make([]float64, 0, len(cleaned))
	zeros := 0
	for _, v := range cleaned {
		if v == 0 {
			zeros++
		} else {
			positives = append(positives, v)
		}
	}
	if zeros == 0 {
		return nil, errors.New("keine Nullwerte: Zero-Inflation ist nicht sinnvoll")
	}
	if len(positives) == 0 {
		return nil, errors.New("keine positiven Werte für die Basisverteilung")
	}

	base, err := f.Base.Fit(positives)
	if err != nil {
		return nil, err
	}

	return ZeroInflated{Base: base, PZero: float64(zeros) / float64(len(cleaned))}, nil
}
