package distribution

import (
	"errors"
	"math"

	"gonum.org/v1/gonum/stat"
)

// Gumbel repräsentiert eine Gumbel-Verteilung (Extremwertverteilung Typ I).
type Gumbel struct {
	Mu   float64 // Lageparameter (Location μ)
	Beta float64 // Skalenparameter (Scale β > 0)
}

func (g Gumbel) Quantile(p float64) float64 {
	if g.Beta <= 0 || !validProbability(p) {
		return math.NaN()
	}
	return g.Mu - g.Beta*math.Log(-math.Log(p))
}

func (g Gumbel) Params() []float64 {
	return []float64{g.Mu, g.Beta}
}

func (g Gumbel) ParamNames() []string {
	return []string{"mu", "beta"}
}

func (g Gumbel) PDF(x float64) float64 {
	if g.Beta <= 0 {
		return 0
	}
	return math.Exp(g.LogPDF(x))
}

func (g Gumbel) LogPDF(x float64) float64 {
	if g.Beta <= 0 {
		return math.Inf(-1)
	}
	z := (x - g.Mu) / g.Beta
	// Log(1/beta * exp(-z - exp(-z))) -> -Log(beta) - z - exp(-z)
	return -math.Log(g.Beta) - z - math.Exp(-z)
}

func (g Gumbel) CDF(x float64) float64 {
	if g.Beta <= 0 {
		return 0
	}
	z := (x - g.Mu) / g.Beta
	return math.Exp(-math.Exp(-z))
}

// GumbelFitter schätzt Mu und Beta präzise per Maximum-Likelihood (MLE).
type GumbelFitter struct{}

func (f GumbelFitter) Name() string {
	return "Gumbel"
}

func (f GumbelFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	n := len(cleaned)
	if n < 2 {
		return nil, errors.New("zu wenige gültige Datenpunkte für Gumbel-Verteilung")
	}

	// 1. Startwerte via Momentenmethode generieren (für schnelle/sichere Konvergenz)
	mean := stat.Mean(cleaned, nil)
	variance := stat.Variance(cleaned, nil)
	stdDev := math.Sqrt(variance)

	// Momentenschätzer als Initial Guess
	initBeta := (stdDev * math.Sqrt(6)) / math.Pi

	// 2. Newton-Raphson zur numerischen Bestimmung von Beta (MLE)
	beta := initBeta
	const maxIterations = 20
	const tolerance = 1e-8

	converged := false
	for i := 0; i < maxIterations; i++ {
		var sumExp, sumXExp, sumXXExp float64
		invBeta := 1.0 / beta

		for _, x := range cleaned {
			e := math.Exp(-x * invBeta)
			sumExp += e
			sumXExp += x * e
			sumXXExp += x * x * e
		}

		if sumExp == 0 {
			break
		}

		// Erster Ableitungsterm der Log-Likelihood nach Beta gleich Null gesetzt:
		// f(beta) = beta - mean + (sum(x * exp(-x/beta)) / sum(exp(-x/beta)))
		xBarGumbel := sumXExp / sumExp
		f := beta - mean + xBarGumbel

		// Zweite Ableitung für die Newton-Schrittweite (f'(beta)):
		// Ableitung des Quotienten sumXExp / sumExp nach beta
		fPrime := 1.0 + invBeta*invBeta*((sumXXExp/sumExp)-(xBarGumbel*xBarGumbel))

		delta := f / fPrime
		beta -= delta

		if math.Abs(delta) < tolerance {
			converged = true
			break
		}
	}

	// Ohne Konvergenz ist kein MLE vorhanden: Fehler statt Momentenschätzer als Ergebnis ausgeben.
	if !converged || beta <= 0 || !IsFinite(beta) {
		return nil, ErrFitFailed
	}

	// 3. Nachdem Beta bestimmt ist, lässt sich Mu direkt analytisch berechnen:
	// mu = -beta * ln( (1/n) * sum(exp(-x/beta)) )
	var sumExpFinal float64
	for _, x := range cleaned {
		sumExpFinal += math.Exp(-x / beta)
	}

	mu := -beta * math.Log(sumExpFinal/float64(n))

	if !IsFinite(mu) {
		return nil, errors.New("MLE-Optimierung für Gumbel-Verteilung fehlgeschlagen")
	}

	return Gumbel{Mu: mu, Beta: beta}, nil
}
