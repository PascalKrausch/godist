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

func (w Weibull) Quantile(p float64) float64 {
	if w.Shape <= 0 || w.Scale <= 0 || !validProbability(p) {
		return math.NaN()
	}
	return w.Scale * math.Pow(-math.Log1p(-p), 1/w.Shape)
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
	cleaned := CleanDataWithZero(data)
	if err := checkNonNegative(cleaned); err != nil {
		return nil, err
	}
	if len(cleaned) == 0 {
		return nil, ErrEmptyData
	}

	// Strenge Nullwert-Policy (Fast-Fail): Nullen werden nicht entfernt. Für Shape ≠ 1 ist die Dichte an 0
	// 0 oder ∞; Shape = 1 entspricht der Exponentialverteilung.
	for _, x := range cleaned {
		if x == 0 {
			return nil, ErrFitFailed
		}
	}

	// Transformation: Logarithmiere die Daten für MLE
	logs := make([]float64, len(cleaned))
	for i, x := range cleaned {
		logs[i] = math.Log(x)
	}

	shape, ok := solveWeibullShape(logs)
	if !ok {
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
// g ist streng monoton steigend in k (g'(k) = Var_w(ln x) + 1/k² > 0), die Nullstelle ist also eindeutig.
// Newton-Raphson konvergiert quadratisch, bleibt aber im Klammerintervall mit Vorzeichenwechsel:
// verlässt ein Schritt das Intervall, wird bisektiert. So bleibt die globale Konvergenz der Bisektion erhalten.
func solveWeibullShape(logs []float64) (float64, bool) {
	meanLog := stat.Mean(logs, nil)
	maxLog := logs[0]
	for _, l := range logs[1:] {
		if l > maxLog {
			maxLog = l
		}
	}

	// g(k) und g'(k) in einem Durchlauf; Skalierung mit maxLog kürzt sich in den Quotienten heraus.
	eval := func(k float64) (float64, float64) {
		var s0, s1, s2 float64
		for _, l := range logs {
			w := math.Exp(k * (l - maxLog))
			s0 += w
			s1 += w * l
			s2 += w * l * l
		}
		m1 := s1 / s0
		return m1 - meanLog - 1.0/k, s2/s0 - m1*m1 + 1.0/(k*k)
	}

	lo, hi := 1e-3, 1e3
	gLo, _ := eval(lo)
	gHi, _ := eval(hi)
	if !IsFinite(gLo) || !IsFinite(gHi) || gLo > 0 || gHi < 0 {
		return 0, false
	}

	// Startwert: Momentenschätzung über die Standardabweichung der logarithmierten Daten (Gumbel-Näherung).
	variance := 0.0
	for _, l := range logs {
		variance += (l - meanLog) * (l - meanLog)
	}
	std := math.Sqrt(variance / float64(len(logs)))
	k := lo
	if std > 0 {
		k = 1.2825 / std
	}
	if !(k > lo && k < hi) {
		k = 0.5 * (lo + hi)
	}

	for iter := 0; iter < 200; iter++ {
		gk, dgk := eval(k)
		if gk == 0 {
			return k, true
		}
		if gk < 0 {
			lo = k
		} else {
			hi = k
		}

		next := k - gk/dgk
		if !(next > lo && next < hi) {
			next = 0.5 * (lo + hi)
		}
		if math.Abs(next-k) <= 1e-12*math.Abs(next) {
			return next, IsFinite(next) && next > 0
		}
		k = next
	}

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
