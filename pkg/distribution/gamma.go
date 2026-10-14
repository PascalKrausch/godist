package distribution

import (
	"math"

	"gonum.org/v1/gonum/mathext"
	"gonum.org/v1/gonum/stat/distuv"
)

// Gamma repräsentiert eine Gamma-Verteilung mit Form- (Shape/α) und Skalenparameter (Scale/θ).
type Gamma struct {
	Alpha float64 // Shape parameter (α > 0)
	Beta  float64 // Scale parameter (θ > 0)
}

func (g Gamma) Params() []float64 {
	return []float64{g.Alpha, g.Beta}
}

func (g Gamma) PDF(x float64) float64 {
	if x <= 0 || g.Alpha <= 0 || g.Beta <= 0 {
		return 0
	}
	return math.Exp(g.LogPDF(x))
}

func (g Gamma) LogPDF(x float64) float64 {
	if x <= 0 || g.Alpha <= 0 || g.Beta <= 0 {
		return math.Inf(-1)
	}
	lgamma, _ := math.Lgamma(g.Alpha)
	return (g.Alpha-1)*math.Log(x) - (x / g.Beta) - (g.Alpha * math.Log(g.Beta)) - lgamma
}

func (g Gamma) CDF(x float64) float64 {
	if x <= 0 || g.Alpha <= 0 || g.Beta <= 0 {
		return 0
	}
	// Nutzen der regulierten unvollständigen Gamma-Funktion P(a, x) aus gonum
	return mathextGammaIncReg(g.Alpha, x/g.Beta)
}

// GammaFitter schätzt Alpha und Beta via Momentenmethode + Thom-Näherung für MLE.
type GammaFitter struct{}

func (f GammaFitter) Name() string {
	return "Gamma"
}

func (f GammaFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanData(data)
	n := float64(len(cleaned))
	if n == 0 {
		return nil, ErrEmptyData
	}

	sum := 0.0
	sumLog := 0.0
	for _, x := range cleaned {
		sum += x
		sumLog += math.Log(x)
	}

	mean := sum / n
	meanLog := sumLog / n
	s := math.Log(mean) - meanLog // Thom-Indikator

	if s <= 0 {
		return nil, ErrFitFailed
	}

	// Thom-Approximation für den MLE des Formparameters Alpha (α)
	alpha := (1.0 + math.Sqrt(1.0+4.0*s/3.0)) / (4.0 * s)

	// Newton-Raphson Verfeinerung für Alpha (optional, erhöht Genauigkeit)
	alpha = refineGammaAlpha(alpha, s)

	beta := mean / alpha // Scale Parameter θ = E[X] / α

	if alpha <= 0 || beta <= 0 || math.IsNaN(alpha) || math.IsNaN(beta) {
		return nil, ErrFitFailed
	}

	return Gamma{Alpha: alpha, Beta: beta}, nil
}

// Helper für die regulierte unvollständige Gammafunktion via Gonum
func mathextGammaIncReg(a, x float64) float64 {
	dist := distuv.Gamma{Alpha: a, Beta: 1.0}
	return dist.CDF(x)
}

// Newton-Raphson Schritt zur Exakten Bestimmung von Alpha aus MLE
func refineGammaAlpha(initialAlpha, s float64) float64 {
	a := initialAlpha
	for i := 0; i < 5; i++ {
		// Digamma (Ψ) und Trigamma (Ψ') Approximationen
		digamma := mathext.Digamma(a)
		// Trigamma über analytische Näherung berechnen
		tri := trigamma(a)
		f := math.Log(a) - digamma - s
		fPrime := (1.0 / a) - tri

		delta := f / fPrime
		a -= delta
		if math.Abs(delta) < 1e-8 {
			break
		}
	}
	return a
}

// trigamma berechnet die 1. Ableitung der Digamma-Funktion ψ'(x)
func trigamma(x float64) float64 {
	if x <= 0 {
		return math.NaN()
	}

	// Recurrence relation nutzen, um x auf >= 8 zu skalieren (erhöht Präzision)
	shift := 0.0
	for x < 8.0 {
		shift += 1.0 / (x * x)
		x++
	}

	// Asymptotische Reihe für x >= 8
	invX2 := 1.0 / (x * x)
	ans := (1.0 / x) + (0.5 * invX2) +
		invX2/x*(1.0/6.0-invX2*(1.0/30.0-invX2*(1.0/42.0)))

	return ans + shift
}
