package distribution

import "math"

// validProbability meldet, ob p im Intervall [0, 1] liegt; NaN ergibt false.
func validProbability(p float64) bool {
	return p >= 0 && p <= 1
}

// stdNormalQuantile ist das Quantil der Standardnormalverteilung.
// Für beide Enden wird die Erfc-Variante genutzt, damit die Ränder nicht an Genauigkeit verlieren.
func stdNormalQuantile(p float64) float64 {
	if p < 0.5 {
		return -math.Sqrt2 * math.Erfcinv(2*p)
	}
	return math.Sqrt2 * math.Erfinv(2*p-1)
}

// bisectCDF invertiert eine monoton steigende CDF auf dem endlichen Intervall [lo, hi].
func bisectCDF(cdf func(float64) float64, p, lo, hi float64) float64 {
	for i := 0; i < 2000; i++ {
		mid := lo + (hi-lo)/2
		if mid <= lo || mid >= hi {
			break
		}
		if cdf(mid) < p {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo + (hi-lo)/2
}
