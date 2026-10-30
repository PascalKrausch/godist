package distribution

import (
	"math"
	"sort"
)

// Cauchy repräsentiert eine Cauchy-Verteilung mit Lageparameter Mu (Median) und Skalierungsparameter
// Scale (s > 0). Mittelwert und Varianz existieren nicht.
type Cauchy struct {
	Mu    float64 // Lageparameter (Median)
	Scale float64 // Skalierungsparameter (s > 0)
}

func (c Cauchy) Quantile(p float64) float64 {
	if !c.Valid() || !validProbability(p) {
		return math.NaN()
	}
	switch p {
	case 0:
		return math.Inf(-1)
	case 1:
		return math.Inf(1)
	}
	return c.Mu + c.Scale*math.Tan(math.Pi*(p-0.5))
}

func (c Cauchy) Params() []float64 {
	return []float64{c.Mu, c.Scale}
}

func (c Cauchy) ParamNames() []string {
	return []string{"mu", "scale"}
}

func (c Cauchy) Valid() bool {
	return c.Scale > 0 && IsFinite(c.Mu) && IsFinite(c.Scale)
}

func (c Cauchy) PDF(x float64) float64 {
	if !c.Valid() {
		return 0
	}
	z := (x - c.Mu) / c.Scale
	return 1 / (math.Pi * c.Scale * (1 + z*z))
}

func (c Cauchy) LogPDF(x float64) float64 {
	if !c.Valid() {
		return math.Inf(-1)
	}
	z := (x - c.Mu) / c.Scale
	return -math.Log(math.Pi*c.Scale) - math.Log1p(z*z)
}

func (c Cauchy) CDF(x float64) float64 {
	if !c.Valid() {
		return 0
	}
	return 0.5 + math.Atan((x-c.Mu)/c.Scale)/math.Pi
}

// CauchyFitter schätzt Mu und Scale per Maximum-Likelihood. Die Likelihood ist nicht geschlossen lösbar,
// daher wird die Fixpunktiteration der Score-Gleichungen verwendet: mit Gewichten
// w_i = 1/(s² + (x_i − μ)²) gilt μ = Σ w_i x_i / Σ w_i und s² = n / (2 Σ w_i).
type CauchyFitter struct{}

func (f CauchyFitter) Name() string {
	return "Cauchy"
}

func (f CauchyFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	if len(cleaned) == 0 {
		return nil, ErrEmptyData
	}
	if len(cleaned) < 2 {
		return nil, ErrFitFailed
	}

	sorted := append([]float64(nil), cleaned...)
	sort.Float64s(sorted)
	n := float64(len(sorted))
	q1 := sorted[int(0.25*(n-1))]
	q3 := sorted[int(0.75*(n-1))]
	mu := sorted[int(0.5*(n-1))]
	s := (q3 - q1) / 2
	if s <= 0 || !IsFinite(s) {
		return nil, ErrFitFailed
	}

	for iter := 0; iter < 10000; iter++ {
		sumW, sumWX := 0.0, 0.0
		for _, x := range cleaned {
			d := x - mu
			w := 1 / (s*s + d*d)
			sumW += w
			sumWX += w * x
		}
		muNew := sumWX / sumW

		sumW = 0
		for _, x := range cleaned {
			d := x - muNew
			sumW += 1 / (s*s + d*d)
		}
		sNew := math.Sqrt(n / (2 * sumW))

		converged := math.Abs(muNew-mu) <= 1e-12*(1+math.Abs(mu)) && math.Abs(sNew-s) <= 1e-12*s
		mu, s = muNew, sNew
		if converged {
			break
		}
	}

	if !IsFinite(mu) || !IsFinite(s) || s <= 0 {
		return nil, ErrFitFailed
	}
	return Cauchy{Mu: mu, Scale: s}, nil
}
