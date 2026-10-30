package distribution

import (
	"errors"
	"math"

	"gonum.org/v1/gonum/mathext"
)

// ChiSquare repräsentiert eine Chi-Quadrat-Verteilung mit DF Freiheitsgraden (k > 0).
// Sie ist der Spezialfall Gamma(α = k/2, θ = 2) mit nur einem freien Parameter.
type ChiSquare struct {
	DF float64 // k > 0
}

func (c ChiSquare) Quantile(p float64) float64 {
	if !c.Valid() || !validProbability(p) {
		return math.NaN()
	}
	x := 2 * mathext.GammaIncRegInv(c.DF/2, p)
	// Zwei Newton-Schritte auf der CDF glätten den Restfehler der Inversion.
	for i := 0; i < 2; i++ {
		d := c.PDF(x)
		if d <= 0 || !IsFinite(d) {
			break
		}
		x -= (c.CDF(x) - p) / d
	}
	return x
}

func (c ChiSquare) Params() []float64 {
	return []float64{c.DF}
}

func (c ChiSquare) ParamNames() []string {
	return []string{"df"}
}

func (c ChiSquare) Valid() bool {
	return c.DF > 0 && IsFinite(c.DF)
}

func (c ChiSquare) PDF(x float64) float64 {
	if !c.Valid() || x < 0 {
		return 0
	}
	return math.Exp(c.LogPDF(x))
}

func (c ChiSquare) LogPDF(x float64) float64 {
	if !c.Valid() || x < 0 {
		return math.Inf(-1)
	}
	a := c.DF / 2
	if x == 0 {
		// Wie bei Gamma: Für k < 2 ist die Dichte an 0 unendlich, für k = 2 gilt f(0) = 1/2, sonst 0.
		switch {
		case a < 1:
			return math.Inf(1)
		case a == 1:
			return -math.Ln2
		default:
			return math.Inf(-1)
		}
	}
	lgamma, _ := math.Lgamma(a)
	return (a-1)*math.Log(x) - x/2 - a*math.Ln2 - lgamma
}

func (c ChiSquare) CDF(x float64) float64 {
	if !c.Valid() || x <= 0 {
		return 0
	}
	return mathextGammaIncReg(c.DF/2, x/2)
}

// ChiSquareFitter schätzt k per Maximum-Likelihood. Mit θ = 2 fest lautet die MLE-Gleichung
// ψ(k/2) = mean(ln x) − ln 2, die per Newton-Raphson auf ψ gelöst wird.
type ChiSquareFitter struct{}

func (f ChiSquareFitter) Name() string {
	return "ChiSquare"
}

func (f ChiSquareFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	if err := checkNonNegative(cleaned); err != nil {
		return nil, err
	}
	if len(cleaned) == 0 {
		return nil, ErrEmptyData
	}

	// Strenge Nullwert-Policy (Fast-Fail) wie bei Gamma: Nullen werden nicht entfernt.
	sumLog := 0.0
	for _, x := range cleaned {
		if x == 0 {
			return nil, ErrFitFailed
		}
		sumLog += math.Log(x)
	}
	target := sumLog/float64(len(cleaned)) - math.Ln2

	a, ok := solveChiSquareShape(target)
	if !ok {
		return nil, errors.New("chi-quadrat-parameter konnten nicht geschätzt werden")
	}
	return ChiSquare{DF: 2 * a}, nil
}

// solveChiSquareShape löst ψ(a) = target für a > 0. ψ ist streng monoton steigend, daher gibt es
// höchstens eine Lösung. Newton-Schritte werden auf a > 0 begrenzt.
func solveChiSquareShape(target float64) (float64, bool) {
	if !IsFinite(target) {
		return 0, false
	}
	a := 1.0
	for i := 0; i < 100; i++ {
		f := mathext.Digamma(a) - target
		step := f / trigamma(a)
		next := a - step
		for next <= 0 {
			step /= 2
			next = a - step
		}
		a = next
		if math.Abs(step) < 1e-12*(1+a) {
			return a, IsFinite(a) && a > 0
		}
	}
	return a, IsFinite(a) && a > 0
}
