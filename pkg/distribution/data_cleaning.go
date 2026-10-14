package distribution

import (
	"math"
	"sort"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
)

// CleanData entfernt ungültige Werte (NaN, Inf, negative) aus einem Datensatz.
// Dies ist eine generische Funktion für alle Verteilungen, die positive Daten benötigen.
func CleanData(data []float64) []float64 {
	out := make([]float64, 0, len(data))
	for _, v := range data {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			continue
		}
		out = append(out, v)
	}
	return out
}

// CleanDataWithZero entfernt nur NaN und Inf, behält aber Nullwerte.
func CleanDataWithZero(data []float64) []float64 {
	out := make([]float64, 0, len(data))
	for _, v := range data {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		out = append(out, v)
	}
	return out
}

// SampleSummary enthält statistische Kennzahlen eines Datensatzes.
type SampleSummary struct {
	Mean   float64
	StdDev float64
	Min    float64
	Max    float64
}

// Summarize berechnet grundlegende statistische Kennzahlen eines Datensatzes.
func Summarize(data []float64) SampleSummary {
	n := len(data)
	if n == 0 {
		return SampleSummary{}
	}

	summary := SampleSummary{
		Mean: stat.Mean(data, nil),
		Min:  floats.Min(data),
		Max:  floats.Max(data),
	}
	if n > 1 {
		variance := stat.Variance(data, nil)
		if variance > 0 {
			summary.StdDev = math.Sqrt(variance)
		}
	}
	return summary
}

// SortedCopy erstellt eine sortierte Kopie eines Datensatzes.
func SortedCopy(data []float64) []float64 {
	out := make([]float64, len(data))
	copy(out, data)
	sort.Float64s(out)
	return out
}

// IsFinite prüft, ob ein Wert eine gültige finite Zahl ist.
func IsFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
