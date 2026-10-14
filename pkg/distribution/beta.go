package distribution

import (
	"errors"
	"math"

	"gonum.org/v1/gonum/mathext"
	"gonum.org/v1/gonum/stat"
)

// Beta repräsentiert eine Beta-Verteilung im Intervall [Min, Max].
// Für die Standard-Beta-Verteilung gilt Min = 0 und Max = 1.
type Beta struct {
	Alpha float64 // Formparameter (α > 0)
	Beta  float64 // Formparameter (β > 0)
	Min   float64 // Untere Intervallgrenze
	Max   float64 // Obere Intervallgrenze
}

func (b Beta) Params() []float64 {
	return []float64{b.Alpha, b.Beta, b.Min, b.Max}
}

func (b Beta) Valid() bool {
	return b.Alpha > 0 && b.Beta > 0 && b.Max > b.Min && IsFinite(b.Alpha) && IsFinite(b.Beta)
}

// Transformiert x aus [Min, Max] nach [0, 1]
func (b Beta) normalize(x float64) float64 {
	return (x - b.Min) / (b.Max - b.Min)
}

func (b Beta) PDF(x float64) float64 {
	if !b.Valid() || x < b.Min || x > b.Max {
		return 0
	}
	return math.Exp(b.LogPDF(x))
}

func (b Beta) LogPDF(x float64) float64 {
	if !b.Valid() || x < b.Min || x > b.Max {
		return math.Inf(-1)
	}

	y := b.normalize(x)
	if y == 0 || y == 1 {
		return math.Inf(-1)
	}

	lgammaA, _ := math.Lgamma(b.Alpha)
	lgammaB, _ := math.Lgamma(b.Beta)
	lgammaAB, _ := math.Lgamma(b.Alpha + b.Beta)

	// LogPDF mit Skalierungsfaktor 1 / (Max - Min)
	logBetaFunc := lgammaA + lgammaB - lgammaAB
	return (b.Alpha-1)*math.Log(y) + (b.Beta-1)*math.Log(1-y) - logBetaFunc - math.Log(b.Max-b.Min)
}

func (b Beta) CDF(x float64) float64 {
	if !b.Valid() || x <= b.Min {
		return 0
	}
	if x >= b.Max {
		return 1
	}
	y := b.normalize(x)
	// Regulierte unvollständige Beta-Funktion I_y(α, β) aus Gonum
	return mathext.RegIncBeta(b.Alpha, b.Beta, y)
}

// BetaFitter schätzt Alpha und Beta via Momentenmethode.
type BetaFitter struct {
	// Wenn Min == Max == 0, werden Min/Max automatisch aus den Daten ermittelt.
	Min float64
	Max float64
}

func (f BetaFitter) Name() string {
	return "Beta"
}

func (f BetaFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanData(data)
	if len(cleaned) < 3 {
		return nil, errors.New("zu wenige Datenpunkte für Beta-Fit")
	}

	minVal := f.Min
	maxVal := f.Max

	// Grenzen automatisch bestimmen, falls nicht vorgegeben
	if minVal == 0 && maxVal == 0 {
		summary := Summarize(cleaned)
		margin := 0.01 * (summary.Max - summary.Min)
		if margin == 0 {
			margin = 0.001
		}
		minVal = summary.Min - margin
		maxVal = summary.Max + margin
	}

	// Daten auf [0, 1] skalieren
	rangeVal := maxVal - minVal
	scaled := make([]float64, len(cleaned))
	for i, x := range cleaned {
		if x <= minVal || x >= maxVal {
			return nil, errors.New("datenpunkte liegen außerhalb der angegebenen Min/Max-Grenzen")
		}
		scaled[i] = (x - minVal) / rangeVal
	}

	// Momentenmethode im skalierten Raum [0, 1]
	mean := stat.Mean(scaled, nil)
	variance := stat.Variance(scaled, nil)

	if variance >= mean*(1.0-mean) || variance <= 0 {
		return nil, errors.New("datenvarianz ist zu hoch/ungültig für Beta-Verteilung")
	}

	// Schätzung für α und β
	commonFactor := (mean * (1.0 - mean) / variance) - 1.0
	alpha := mean * commonFactor
	betaParam := (1.0 - mean) * commonFactor

	if alpha <= 0 || betaParam <= 0 || !IsFinite(alpha) || !IsFinite(betaParam) {
		return nil, errors.New("beta-parameter konnte nicht geschätzt werden")
	}

	return Beta{
		Alpha: alpha,
		Beta:  betaParam,
		Min:   minVal,
		Max:   maxVal,
	}, nil
}
