// Package stats berechnet deskriptive Kennzahlen einer Stichprobe.
// Es enthält keine Darstellung; CLI und Dashboard entscheiden selbst, wie die Werte ausgegeben werden.
package stats

import (
	"encoding/json"
	"math"
	"slices"

	"gonum.org/v1/gonum/stat"
)

// Summary enthält die deskriptiven Kennzahlen einer Stichprobe.
// Varianz und Standardabweichung sind Stichprobenwerte (n-1). Schiefe und Kurtosis sind die
// Stichprobenkorrigierten Werte G1 bzw. G2 (Excess-Kurtosis, Normalverteilung ergibt 0).
// Nicht berechenbare Kennzahlen sind NaN, z. B. Varianz bei n < 2 oder Schiefe bei n < 3.
type Summary struct {
	N        int
	Mean     float64
	Median   float64
	Variance float64
	StdDev   float64
	Min      float64
	Q1       float64
	Q3       float64
	Max      float64
	Skewness float64
	Kurtosis float64
}

// MarshalJSON schreibt nicht berechenbare Kennzahlen als null, damit das JSON gültig bleibt.
func (s Summary) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		N        int      `json:"n"`
		Mean     *float64 `json:"mean"`
		Median   *float64 `json:"median"`
		Variance *float64 `json:"variance"`
		StdDev   *float64 `json:"std_dev"`
		Min      *float64 `json:"min"`
		Q1       *float64 `json:"q1"`
		Q3       *float64 `json:"q3"`
		Max      *float64 `json:"max"`
		Skewness *float64 `json:"skewness"`
		Kurtosis *float64 `json:"kurtosis"`
	}{
		N:        s.N,
		Mean:     finiteOrNil(s.Mean),
		Median:   finiteOrNil(s.Median),
		Variance: finiteOrNil(s.Variance),
		StdDev:   finiteOrNil(s.StdDev),
		Min:      finiteOrNil(s.Min),
		Q1:       finiteOrNil(s.Q1),
		Q3:       finiteOrNil(s.Q3),
		Max:      finiteOrNil(s.Max),
		Skewness: finiteOrNil(s.Skewness),
		Kurtosis: finiteOrNil(s.Kurtosis),
	})
}

func finiteOrNil(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}

// Describe berechnet die Kennzahlen aus values. Die Eingabe wird nicht verändert.
// Nicht endliche Werte muss der Aufrufer vorher entfernen.
func Describe(values []float64) Summary {
	nan := math.NaN()
	s := Summary{
		N:        len(values),
		Mean:     nan,
		Median:   nan,
		Variance: nan,
		StdDev:   nan,
		Min:      nan,
		Q1:       nan,
		Q3:       nan,
		Max:      nan,
		Skewness: nan,
		Kurtosis: nan,
	}
	if len(values) == 0 {
		return s
	}

	sorted := slices.Clone(values)
	slices.Sort(sorted)

	s.Mean = stat.Mean(values, nil)
	s.Min = sorted[0]
	s.Max = sorted[len(sorted)-1]
	s.Median = quantileSorted(sorted, 0.5)
	s.Q1 = quantileSorted(sorted, 0.25)
	s.Q3 = quantileSorted(sorted, 0.75)

	if len(values) >= 2 {
		s.Variance = stat.Variance(values, nil)
		s.StdDev = math.Sqrt(s.Variance)
	}
	if s.StdDev > 0 && len(values) >= 3 {
		s.Skewness = stat.Skew(values, nil)
	}
	if s.StdDev > 0 && len(values) >= 4 {
		s.Kurtosis = stat.ExKurtosis(values, nil)
	}
	return s
}

// quantileSorted berechnet das p-Quantil aus sortierten Daten durch lineare Interpolation (Typ 7, wie NumPy und R).
// Dieselbe Definition nutzt internal/plot für die Q-Q-Daten.
func quantileSorted(sorted []float64, p float64) float64 {
	pos := p * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	frac := pos - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}
