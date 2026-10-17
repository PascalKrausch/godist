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
	if len(cleaned) == 0 {
		return nil, ErrEmptyData
	}

	// Schätzung auf den positiven Werten; Nullen werden unten gegen den Formparameter geprüft.
	positive := make([]float64, 0, len(cleaned))
	for _, x := range cleaned {
		if x > 0 {
			positive = append(positive, x)
		}
	}
	hasZeros := len(positive) < len(cleaned)
	if len(positive) == 0 {
		return nil, ErrFitFailed
	}

	// Transformation: Logarithmiere die Daten für MLE
	logs := make([]float64, len(positive))
	for i, x := range positive {
		logs[i] = math.Log(x)
	}

	shape, ok := solveWeibullShape(logs)
	if !ok {
		return nil, ErrFitFailed
	}

	// Strenge Nullwert-Policy (Fast-Fail): Bei Formparameter < 1 ist die Dichte an 0 unendlich.
	if hasZeros && shape < 1 {
		return nil, ErrFitFailed
	}

	// Scale aus Shape und den logarithmierten Daten berechnen
	scale := weibullScaleFromLogs(logs, shape)
	if !IsFinite(scale) || scale <= 0 {
		return nil, ErrFitFailed
	}

	return Weibull{Shape: shape, Scale: scale}, nil
}

// solveWeibullShape bestimmt den Shape-MLE als Nullstelle der Score-Gleichung
//
//	g(k) = Σ xᵢᵏ ln xᵢ / Σ xᵢᵏ − mean(ln x) − 1/k
//
// g ist streng monoton steigend in k, daher ist die Bisektion auf einem Intervall
// mit Vorzeichenwechsel global konvergent (anders als eine Fixpunktiteration).
func solveWeibullShape(logs []float64) (float64, bool) {
	meanLog := stat.Mean(logs, nil)
	maxLog := logs[0]
	for _, l := range logs[1:] {
		if l > maxLog {
			maxLog = l
		}
	}

	g := func(k float64) float64 {
		sumPow := 0.0
		sumPowLog := 0.0
		for _, l := range logs {
			w := math.Exp(k * (l - maxLog))
			sumPow += w
			sumPowLog += w * l
		}
		return sumPowLog/sumPow - meanLog - 1.0/k
	}

	lo, hi := 1e-3, 1e3
	gLo, gHi := g(lo), g(hi)
	if !IsFinite(gLo) || !IsFinite(gHi) || gLo > 0 || gHi < 0 {
		return 0, false
	}

	for iter := 0; iter < 200 && hi-lo > 1e-12*hi; iter++ {
		mid := 0.5 * (lo + hi)
		gMid := g(mid)
		if gMid == 0 {
			return mid, true
		}
		if gMid < 0 {
			lo = mid
		} else {
			hi = mid
		}
	}

	k := 0.5 * (lo + hi)
	return k, IsFinite(k) && k > 0
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
