package distribution

import (
	"errors"
	"math"

	"gonum.org/v1/gonum/stat"
)

// Weibull repräsentiert eine 3-Parameter-Weibull-Verteilung (Shape k, Scale λ, Location μ).
type Weibull struct {
	Shape    float64 // Formparameter (k > 0)
	Scale    float64 // Skalenparameter (λ > 0)
	Location float64 // Lageparameter (μ)
}

func (w Weibull) Params() []float64 {
	return []float64{w.Shape, w.Scale, w.Location}
}

func (w Weibull) Valid() bool {
	return IsFinite(w.Shape) && IsFinite(w.Scale) && IsFinite(w.Location) && w.Shape > 0 && w.Scale > 0
}

func (w Weibull) PDF(x float64) float64 {
	if !w.Valid() {
		return 0
	}
	z := (x - w.Location) / w.Scale
	if z < 0 {
		return 0
	}
	if z == 0 {
		switch {
		case w.Shape < 1:
			return math.Inf(1)
		case w.Shape == 1:
			return w.Shape / w.Scale
		default:
			return 0
		}
	}

	return (w.Shape / w.Scale) *
		math.Pow(z, w.Shape-1) *
		math.Exp(-math.Pow(z, w.Shape))
}

func (w Weibull) CDF(x float64) float64 {
	if !w.Valid() {
		return 0
	}
	z := (x - w.Location) / w.Scale
	if z < 0 {
		return 0
	}
	return 1 - math.Exp(-math.Pow(z, w.Shape))
}

func (w Weibull) LogPDF(x float64) float64 {
	if !w.Valid() {
		return math.Inf(-1)
	}
	z := (x - w.Location) / w.Scale
	if z < 0 {
		return math.Inf(-1)
	}
	if z == 0 {
		switch {
		case w.Shape < 1:
			return math.Inf(1)
		case w.Shape == 1:
			return math.Log(w.Shape / w.Scale)
		default:
			return math.Inf(-1)
		}
	}

	return math.Log(w.Shape/w.Scale) +
		(w.Shape-1)*math.Log(z) -
		math.Pow(z, w.Shape)
}

// ==========================================
// WEIBULL FITTER
// ==========================================

// WeibullFitter schätzt Shape, Scale und Location per Maximum-Likelihood-Approximation.
type WeibullFitter struct{}

func (f WeibullFitter) Name() string {
	return "Weibull"
}

func (f WeibullFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanData(data)
	if len(cleaned) < 3 {
		return nil, errors.New("zu wenige gültige Datenpunkte für Weibull-Fit")
	}

	summary := Summarize(cleaned)
	if summary.Min == summary.Max {
		loc := summary.Min - 1e-6
		return Weibull{
			Shape:    1,
			Scale:    math.Max(summary.Min-loc, 1e-6),
			Location: loc,
		}, nil
	}

	searchWidth := math.Max(3*summary.StdDev, math.Max(0.1*(summary.Max-summary.Min), 1.0))
	searchLower := math.Min(0, summary.Min-searchWidth)
	searchUpper := summary.Min - 1e-9
	if searchLower >= searchUpper {
		searchLower = searchUpper - math.Max(searchWidth, 1.0)
	}

	bestLL := math.Inf(-1)
	var best Weibull
	found := false

	// Vorallokation der Slices außerhalb der Hot-Loop zur Schonung des GC
	shifted := make([]float64, len(cleaned))
	logs := make([]float64, len(cleaned))

	const candidateCount = 32
	for i := 0; i < candidateCount; i++ {
		loc := searchLower + float64(i)*(searchUpper-searchLower)/float64(candidateCount-1)
		w, ll, ok := fitWeibullAtLocation(cleaned, loc, shifted, logs)
		if ok && ll > bestLL {
			bestLL = ll
			best = w
			found = true
		}
	}

	if !found {
		return nil, errors.New("weibull fit fehlgeschlagen")
	}

	return best, nil
}

// Helper für die Schätzung bei fixiertem Lageparameter (Location)
func fitWeibullAtLocation(data []float64, location float64, shifted, logs []float64) (Weibull, float64, bool) {
	for i, x := range data {
		y := x - location
		if y <= 0 || !IsFinite(y) {
			return Weibull{}, math.Inf(-1), false
		}
		shifted[i] = y
		logs[i] = math.Log(y)
	}

	summary := Summarize(shifted)
	if summary.Mean <= 0 || !IsFinite(summary.Mean) {
		return Weibull{}, math.Inf(-1), false
	}

	shape := math.Pow(summary.StdDev/summary.Mean, -1.086)
	if !IsFinite(shape) || shape <= 0 {
		shape = 1.5
	}
	shape = refineWeibullShape(logs, shape)
	if !IsFinite(shape) || shape <= 0 {
		return Weibull{}, math.Inf(-1), false
	}

	scale := weibullScaleFromLogs(logs, shape)
	if !IsFinite(scale) || scale <= 0 {
		return Weibull{}, math.Inf(-1), false
	}

	w := Weibull{Shape: shape, Scale: scale, Location: location}

	// Berechne Log-Likelihood direkt über das Objekt
	ll := 0.0
	for _, x := range data {
		v := w.LogPDF(x)
		if math.IsInf(v, -1) {
			return Weibull{}, math.Inf(-1), false
		}
		ll += v
	}
	if !IsFinite(ll) {
		return Weibull{}, math.Inf(-1), false
	}

	return w, ll, true
}

func refineWeibullShape(logs []float64, initial float64) float64 {
	k := initial
	if !IsFinite(k) || k <= 0 {
		k = 1.5
	}

	// Maximum Log für numerische Stabilität gegen Exp-Overflow
	maxLog := logs[0]
	for _, logY := range logs[1:] {
		if logY > maxLog {
			maxLog = logY
		}
	}

	meanLog := stat.Mean(logs, nil)
	for iter := 0; iter < 64; iter++ {
		sumPow := 0.0
		sumPowLog := 0.0
		for _, logY := range logs {
			// Skalierung mit maxLog verhindert math.Inf bei großen Werten
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

		if math.Abs(next-k) <= 1e-7*math.Max(1, k) {
			k = next
			break
		}
		k = next
	}

	return math.Max(k, 1e-6)
}

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

	// Exponentieller Ausgleich für die Max-Skalierung
	logScale := maxLog + math.Log(sumPow/float64(len(logs)))/shape
	return math.Exp(logScale)
}
