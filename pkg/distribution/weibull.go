package distribution

import (
	"math"

	"gonum.org/v1/gonum/stat"
)

// Weibull repräsentiert eine 2-Parameter-Weibull-Verteilung (Shape k, Scale λ).
// Der Träger ist [0, ∞), d.h. Location ist implizit 0.
type Weibull struct {
	Shape float64 // Formparameter (k > 0)
	Scale float64 // Skalenparameter (λ > 0)
}

func (w Weibull) Params() []float64 {
	return []float64{w.Shape, w.Scale}
}

func (w Weibull) ParamNames() []string {
	return []string{"shape", "scale"}
}

func (w Weibull) PDF(x float64) float64 {
	if x < 0 || w.Shape <= 0 || w.Scale <= 0 {
		return 0
	}
	return math.Exp(w.LogPDF(x))
}

func (w Weibull) LogPDF(x float64) float64 {
	if x < 0 || w.Shape <= 0 || w.Scale <= 0 {
		return math.Inf(-1)
	}
	if x == 0 {
		if w.Shape < 1 {
			return math.Inf(1)
		}
		return math.Inf(-1)
	}
	z := x / w.Scale
	return math.Log(w.Shape/w.Scale) + (w.Shape-1)*math.Log(z) - math.Pow(z, w.Shape)
}

func (w Weibull) CDF(x float64) float64 {
	if x < 0 || w.Shape <= 0 || w.Scale <= 0 {
		return 0
	}
	z := x / w.Scale
	return 1 - math.Exp(-math.Pow(z, w.Shape))
}

// ==========================================
// WEIBULL FITTER
// ==========================================

// WeibullFitter schätzt Shape und Scale per Maximum-Likelihood (MLE).
type WeibullFitter struct{}

func (f WeibullFitter) Name() string {
	return "Weibull"
}

func (f WeibullFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanData(data)
	n := float64(len(cleaned))
	if n == 0 {
		return nil, ErrEmptyData
	}

	// Transformation: Logarithmiere die Daten für MLE
	logs := make([]float64, len(cleaned))
	for i, x := range cleaned {
		logs[i] = math.Log(x)
	}

	// Startwert für Shape basierend auf der Variationskoeffizient-Methode
	mean := stat.Mean(cleaned, nil)
	if mean <= 0 || !IsFinite(mean) {
		return nil, ErrFitFailed
	}

	stdDev := stat.StdDev(cleaned, nil)
	cv := stdDev / mean
	if cv <= 0 || !IsFinite(cv) {
		return nil, ErrFitFailed
	}

	// Näherung für Shape aus der Variationskoeffizient-Methode
	shape := math.Pow(cv, -1.086)
	if !IsFinite(shape) || shape <= 0 {
		shape = 1.0
	}

	// Newton-Raphson Verfeinerung für Shape (MLE)
	shape = refineWeibullShape(logs, shape)
	if !IsFinite(shape) || shape <= 0 {
		return nil, ErrFitFailed
	}

	// Scale aus Shape und den logarithmierten Daten berechnen
	scale := weibullScaleFromLogs(logs, shape)
	if !IsFinite(scale) || scale <= 0 {
		return nil, ErrFitFailed
	}

	return Weibull{Shape: shape, Scale: scale}, nil
}

// refineWeibullShape verfeinert den Shape-Parameter per Newton-Raphson für MLE.
func refineWeibullShape(logs []float64, initial float64) float64 {
	k := initial
	if !IsFinite(k) || k <= 0 {
		k = 1.0
	}

	meanLog := stat.Mean(logs, nil)

	// Maximum Log für numerische Stabilität gegen Exp-Overflow
	maxLog := logs[0]
	for _, logY := range logs[1:] {
		if logY > maxLog {
			maxLog = logY
		}
	}

	const maxIterations = 20
	const tolerance = 1e-8

	for iter := 0; iter < maxIterations; iter++ {
		sumPow := 0.0
		sumPowLog := 0.0
		for _, logY := range logs {
			pow := math.Exp(k * (logY - maxLog))
			sumPow += pow
			sumPowLog += pow * logY
		}

		if sumPow <= 0 || !IsFinite(sumPow) {
			break
		}

		denom := sumPowLog/sumPow - meanLog
		if denom <= 0 || !IsFinite(denom) {
			break
		}

		next := 1.0 / denom
		if !IsFinite(next) || next <= 0 {
			break
		}

		if math.Abs(next-k) <= tolerance*math.Max(1, k) {
			k = next
			break
		}
		k = next
	}

	return math.Max(k, 1e-6)
}

// weibullScaleFromLogs berechnet den Scale-Parameter aus Shape und logarithmierten Daten.
func weibullScaleFromLogs(logs []float64, shape float64) float64 {
	maxLog := logs[0]
	for _, logY := range logs[1:] {
		if logY > maxLog {
			maxLog = logY
		}
	}

	sumPow := 0.0
	for _, logY := range logs {
		sumPow += math.Exp(shape * (logY - maxLog))
	}

	logScale := maxLog + math.Log(sumPow/float64(len(logs)))/shape
	return math.Exp(logScale)
}
