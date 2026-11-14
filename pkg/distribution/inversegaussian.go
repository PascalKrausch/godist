package distribution

import (
	"math"
)

// InverseGaussian repräsentiert eine inverse Gauß-Verteilung (Wald-Verteilung)
// mit Mittelwert μ > 0 und Formparameter λ > 0. Der Träger ist (0, ∞).
// Die Dichte lautet f(x) = sqrt(λ/(2πx³)) · exp(−λ(x−μ)²/(2μ²x)) für x > 0.
type InverseGaussian struct {
	Mu     float64 // Mittelwert μ > 0
	Lambda float64 // Formparameter λ > 0
}

func (g InverseGaussian) Valid() bool {
	return IsFinite(g.Mu) && IsFinite(g.Lambda) && g.Mu > 0 && g.Lambda > 0
}

func (g InverseGaussian) Params() []float64 {
	return []float64{g.Mu, g.Lambda}
}

func (g InverseGaussian) ParamNames() []string {
	return []string{"mu", "lambda"}
}

// logPhi liefert ln Φ(z) der Standardnormalverteilung. Für sehr negative z
// wird die Mills-Ratio-Asymptotik genutzt, weil erfc dort unterläuft.
func logPhi(z float64) float64 {
	if z < -15 {
		iz := 1 / (z * z)
		series := 1 - iz + 3*iz*iz - 15*iz*iz*iz + 105*iz*iz*iz*iz - 945*iz*iz*iz*iz*iz
		return -0.5*z*z - math.Log(-z) - 0.5*math.Log(2*math.Pi) + math.Log(series)
	}
	return math.Log(0.5 * math.Erfc(-z/math.Sqrt2))
}

func (g InverseGaussian) PDF(x float64) float64 {
	if !g.Valid() || x <= 0 || math.IsInf(x, 1) {
		return 0
	}
	return math.Exp(g.LogPDF(x))
}

func (g InverseGaussian) LogPDF(x float64) float64 {
	if !g.Valid() || x <= 0 || math.IsInf(x, 1) {
		return math.Inf(-1)
	}
	// −λ(x−μ)²/(2μ²x) = λ/μ − λx/(2μ²) − λ/(2x); die Form ohne quadrierte
	// Differenz vermeidet Überlauf für große x.
	exponent := g.Lambda/g.Mu - g.Lambda*x/(2*g.Mu*g.Mu) - g.Lambda/(2*x)
	return 0.5*math.Log(g.Lambda/(2*math.Pi)) - 1.5*math.Log(x) + exponent
}

func (g InverseGaussian) CDF(x float64) float64 {
	if !g.Valid() || x <= 0 {
		return 0
	}
	if math.IsInf(x, 1) {
		return 1
	}
	t := math.Sqrt(g.Lambda / x)
	a := t * (x/g.Mu - 1)
	b := -t * (x/g.Mu + 1)
	term1 := 0.5 * math.Erfc(-a/math.Sqrt2)
	// exp(2λ/μ)·Φ(b) wird über den Logarithmus berechnet, damit große λ/μ
	// nicht überlaufen und winzige Φ(b) nicht unterlaufen.
	logTerm2 := 2*g.Lambda/g.Mu + logPhi(b)
	var term2 float64
	if logTerm2 > math.Log(math.SmallestNonzeroFloat64) {
		term2 = math.Exp(logTerm2)
	}
	f := term1 + term2
	if f > 1 {
		return 1
	}
	if f < 0 {
		return 0
	}
	return f
}

// Quantile invertiert die CDF per Bisektion. Es gibt keine geschlossene Form;
// die obere Schranke wird durch Verdopplung gefunden.
func (g InverseGaussian) Quantile(p float64) float64 {
	if !g.Valid() || !validProbability(p) {
		return math.NaN()
	}
	if p == 0 {
		return 0
	}
	if p == 1 {
		return math.Inf(1)
	}
	hi := g.Mu
	for i := 0; i < 200; i++ {
		if g.CDF(hi) >= p {
			return bisectCDF(g.CDF, p, 0, hi)
		}
		hi *= 2
		if math.IsInf(hi, 0) {
			return math.NaN()
		}
	}
	return math.NaN()
}

// InverseGaussianFitter schätzt μ und λ per Maximum-Likelihood.
// Die Score-Gleichungen liefern die geschlossene Lösung μ̂ = mean(x) und
// λ̂ = n·μ̂ / Σ(μ̂ − xᵢ)/xᵢ = 1 / (mean(1/x) − 1/mean(x)).
type InverseGaussianFitter struct{}

func (f InverseGaussianFitter) Name() string {
	return "InverseGaussian"
}

func (f InverseGaussianFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	if err := checkNonNegative(cleaned); err != nil {
		return nil, err
	}
	if len(cleaned) == 0 {
		return nil, ErrEmptyData
	}

	// Strenge Nullwert-Policy (Fast-Fail): Nullen liegen außerhalb des Trägers (0, ∞).
	n := float64(len(cleaned))
	var sum, sumInv float64
	for _, x := range cleaned {
		if x == 0 {
			return nil, ErrFitFailed
		}
		sum += x
		sumInv += 1 / x
	}

	mu := sum / n
	// Nenner = Σ(μ − xᵢ)/xᵢ = μ·Σ(1/xᵢ) − n > 0 für nicht-konstante Daten (Jensen).
	denom := mu*sumInv - n
	if !(denom > 0) {
		return nil, ErrFitFailed
	}
	lambda := n * mu / denom

	dist := InverseGaussian{Mu: mu, Lambda: lambda}
	if !dist.Valid() {
		return nil, ErrFitFailed
	}
	return dist, nil
}
