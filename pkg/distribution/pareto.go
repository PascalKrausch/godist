package distribution

import (
	"errors"
	"math"
)

// Pareto repräsentiert eine Pareto-Verteilung (Typ I) mit Formparameter Shape (α > 0) und
// Skalierungsparameter Scale (xm > 0). Der Träger ist [xm, ∞).
type Pareto struct {
	Shape float64 // α > 0
	Scale float64 // xm > 0, untere Schranke des Trägers
}

func (p Pareto) Quantile(q float64) float64 {
	if !p.Valid() || !validProbability(q) {
		return math.NaN()
	}
	// x = xm · (1 − q)^(−1/α), als exp(−ln(1 − q)/α) für Stabilität bei kleinen q
	return p.Scale * math.Exp(-math.Log1p(-q)/p.Shape)
}

func (p Pareto) Params() []float64 {
	return []float64{p.Shape, p.Scale}
}

func (p Pareto) ParamNames() []string {
	return []string{"shape", "scale"}
}

func (p Pareto) Valid() bool {
	return p.Shape > 0 && p.Scale > 0 && IsFinite(p.Shape) && IsFinite(p.Scale)
}

func (p Pareto) PDF(x float64) float64 {
	if !p.Valid() || x < p.Scale {
		return 0
	}
	return math.Exp(p.LogPDF(x))
}

func (p Pareto) LogPDF(x float64) float64 {
	if !p.Valid() || x < p.Scale {
		return math.Inf(-1)
	}
	// ln f(x) = ln α + α · ln xm − (α + 1) · ln x
	return math.Log(p.Shape) + p.Shape*math.Log(p.Scale) - (p.Shape+1)*math.Log(x)
}

func (p Pareto) CDF(x float64) float64 {
	if !p.Valid() || x < p.Scale {
		return 0
	}
	// F(x) = 1 − (xm/x)^α = −expm1(α · ln(xm/x))
	return -math.Expm1(p.Shape * math.Log(p.Scale/x))
}

// ParetoFitter schätzt Shape und Scale per Maximum-Likelihood (MLE).
type ParetoFitter struct{}

func (f ParetoFitter) Name() string {
	return "Pareto"
}

// Fit: Die Likelihood steigt mit xm bis zum Minimum der Daten, daher ist xm = min(x).
// Mit L_i = ln(x_i / xm) folgt α = n / Σ L_i.
func (f ParetoFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	if err := checkNonNegative(cleaned); err != nil {
		return nil, err
	}
	if len(cleaned) == 0 {
		return nil, ErrEmptyData
	}

	// Strenge Nullwert-Policy (Fast-Fail): Nullwerte liegen nie im Träger [xm, ∞) mit xm > 0.
	xm := math.Inf(1)
	for _, x := range cleaned {
		if x == 0 {
			return nil, ErrFitFailed
		}
		xm = math.Min(xm, x)
	}

	sumLog := 0.0
	for _, x := range cleaned {
		sumLog += math.Log(x / xm)
	}
	if sumLog <= 0 || !IsFinite(sumLog) {
		return nil, errors.New("pareto-parameter konnten nicht geschätzt werden")
	}

	return Pareto{Shape: float64(len(cleaned)) / sumLog, Scale: xm}, nil
}
