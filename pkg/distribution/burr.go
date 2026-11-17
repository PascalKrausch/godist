package distribution

import (
	"errors"
	"fmt"
	"math"

	"gonum.org/v1/gonum/optimize"
)

// BurrXII repräsentiert eine Burr-Typ-XII-Verteilung mit Skalierung λ (Scale > 0) und zwei Formparametern
// c (ShapeC > 0) und k (ShapeK > 0). Die Verteilungsfunktion lautet F(x) = 1 − (1 + (x/λ)^c)^(−k) für x > 0.
type BurrXII struct {
	Scale  float64 // Skalierungsparameter λ
	ShapeC float64 // Formparameter c
	ShapeK float64 // Formparameter k
}

func (b BurrXII) Valid() bool {
	return IsFinite(b.Scale) && IsFinite(b.ShapeC) && IsFinite(b.ShapeK) &&
		b.Scale > 0 && b.ShapeC > 0 && b.ShapeK > 0
}

func (b BurrXII) Params() []float64 {
	return []float64{b.Scale, b.ShapeC, b.ShapeK}
}

func (b BurrXII) ParamNames() []string {
	return []string{"scale", "shape_c", "shape_k"}
}

// softplus berechnet ln(1 + e^z) ohne Überlauf.
func softplus(z float64) float64 {
	s, _ := softplusSigmoid(z)
	return s
}

// softplusSigmoid liefert softplus(z) = ln(1 + e^z) und sigmoid(z) = 1 / (1 + e^(−z)) ohne Überlauf
// und mit nur einer Exponentialfunktion; die Likelihood-Schleife braucht beide Werte gemeinsam.
func softplusSigmoid(z float64) (float64, float64) {
	e := math.Exp(-math.Abs(z))
	if z >= 0 {
		return z + math.Log1p(e), 1 / (1 + e)
	}
	return math.Log1p(e), e / (1 + e)
}

// logOnePlusU liefert ln(1 + (x/λ)^c) für x > 0.
func (b BurrXII) logOnePlusU(x float64) float64 {
	return softplus(b.ShapeC * (math.Log(x) - math.Log(b.Scale)))
}

func (b BurrXII) PDF(x float64) float64 {
	if !b.Valid() || x <= 0 || math.IsInf(x, 1) {
		return 0
	}
	return math.Exp(b.LogPDF(x))
}

func (b BurrXII) LogPDF(x float64) float64 {
	if !b.Valid() || x <= 0 || math.IsInf(x, 1) {
		return math.Inf(-1)
	}
	lx := math.Log(x)
	lScale := math.Log(b.Scale)
	z := b.ShapeC * (lx - lScale)
	return math.Log(b.ShapeC) + math.Log(b.ShapeK) + (b.ShapeC-1)*lx - b.ShapeC*lScale -
		(b.ShapeK+1)*softplus(z)
}

func (b BurrXII) CDF(x float64) float64 {
	if !b.Valid() || x <= 0 {
		return 0
	}
	if math.IsInf(x, 1) {
		return 1
	}
	// −expm1 statt 1 − exp: bleibt für kleine CDF-Werte genau.
	return -math.Expm1(-b.ShapeK * b.logOnePlusU(x))
}

// Quantile invertiert die CDF: λ·((1 − p)^(−1/k) − 1)^(1/c). Die Klammer wird über expm1 und log1p
// berechnet, damit kleine p nicht durch Auslöschung verloren gehen.
func (b BurrXII) Quantile(p float64) float64 {
	if !b.Valid() || math.IsNaN(p) || !validProbability(p) {
		return math.NaN()
	}
	if p == 0 {
		return 0
	}
	if p == 1 {
		return math.Inf(1)
	}
	u := math.Expm1(-math.Log1p(-p) / b.ShapeK)
	return b.Scale * math.Pow(u, 1/b.ShapeC)
}

// BurrXIIFitter schätzt Scale, ShapeC und ShapeK per Maximum-Likelihood.
type BurrXIIFitter struct{}

func (f BurrXIIFitter) Name() string {
	return "BurrXII"
}

// burrProfile hält die logarithmierten, zentrierten Daten y_i = ln x_i − m und die Summe Σy_i.
// Die Zielfunktion wird über (γ, b) = (ln c, ln λ − m) profiliert; k ist dann geschlossen als n / Σs_i bestimmt.
type burrProfile struct {
	y    []float64
	sumY float64

	// Cache des letzten evaluate-Aufrufs: Func und Grad fragen am selben Punkt (gamma, b) ab,
	// sodass der O(n)-Pass pro Optimierungsschritt nur einmal läuft.
	cacheGamma, cacheB                   float64
	cacheLL, cacheDG, cacheDB, cacheSumS float64
	cacheOK, cacheValid                  bool
}

// evaluate liefert die Log-Likelihood (bis auf Konstanten) des profilierten Modells, dessen
// Gradienten bezüglich (γ, b) und Σs_i. ok ist false, wenn Σs_i nicht positiv und endlich ist.
func (p *burrProfile) evaluate(gamma, b float64) (ll, dGamma, dB, sumS float64, ok bool) {
	if p.cacheValid && p.cacheGamma == gamma && p.cacheB == b {
		return p.cacheLL, p.cacheDG, p.cacheDB, p.cacheSumS, p.cacheOK
	}
	c := math.Exp(gamma)
	n := float64(len(p.y))

	var sumSig, sumSigD float64
	sumS = 0
	for _, yi := range p.y {
		d := yi - b
		s, sig := softplusSigmoid(c * d)
		sumS += s
		sumSig += sig
		sumSigD += sig * d
	}
	if !(sumS > 0) || math.IsInf(sumS, 0) {
		p.cacheGamma, p.cacheB, p.cacheLL, p.cacheDG, p.cacheDB, p.cacheSumS, p.cacheOK, p.cacheValid =
			gamma, b, 0, 0, 0, 0, false, true
		return 0, 0, 0, 0, false
	}

	kHat := n / sumS
	ll = (c-1)*p.sumY + n*gamma - n*c*b - n*math.Log(sumS) - sumS
	dB = c * ((kHat+1)*sumSig - n)
	dGamma = c*p.sumY + n - n*c*b - c*(kHat+1)*sumSigD
	p.cacheGamma, p.cacheB, p.cacheLL, p.cacheDG, p.cacheDB, p.cacheSumS, p.cacheOK, p.cacheValid =
		gamma, b, ll, dGamma, dB, sumS, true, true
	return ll, dGamma, dB, sumS, true
}

// Fit schätzt die Parameter per L-BFGS auf der profilierten Log-Likelihood mit analytischem Gradienten.
// Startwerte stammen aus der Log-Logistic-MLE (k = 1 ist ein Spezialfall der Burr-XII-Verteilung).
// Nullwerte scheitern, da ln(0) nicht definiert ist; negative Werte liegen außerhalb des Trägers.
func (f BurrXIIFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	if err := checkNonNegative(cleaned); err != nil {
		return nil, err
	}
	if len(cleaned) == 0 {
		return nil, ErrEmptyData
	}
	if len(cleaned) < 3 {
		return nil, fmt.Errorf("BurrXII benötigt mindestens drei Beobachtungen: %w", ErrFitFailed)
	}

	logs := make([]float64, len(cleaned))
	var meanLog float64
	for i, x := range cleaned {
		if x == 0 {
			return nil, fmt.Errorf("BurrXII scheitert bei Nullwerten, da ln(0) nicht definiert ist: %w", ErrFitFailed)
		}
		logs[i] = math.Log(x)
		meanLog += logs[i]
	}
	meanLog /= float64(len(logs))

	profile := burrProfile{y: make([]float64, len(logs))}
	for i, l := range logs {
		profile.y[i] = l - meanLog
		profile.sumY += profile.y[i]
	}

	start, err := LogLogisticFitter{}.Fit(cleaned)
	if err != nil {
		return nil, err
	}
	ll := start.(LogLogistic)

	n := float64(len(cleaned))
	problem := optimize.Problem{
		Func: func(x []float64) float64 {
			if math.Abs(x[0]) > math.Log(burrMaxShape) {
				return math.Inf(1)
			}
			v, _, _, _, ok := profile.evaluate(x[0], x[1])
			if !ok {
				return math.Inf(1)
			}
			return -v / n
		},
		Grad: func(grad, x []float64) {
			_, dG, dB, _, ok := profile.evaluate(x[0], x[1])
			if !ok {
				grad[0] = math.Inf(1)
				grad[1] = math.Inf(1)
				return
			}
			grad[0] = -dG / n
			grad[1] = -dB / n
		},
	}

	settings := &optimize.Settings{
		GradientThreshold: 1e-10,
		MajorIterations:   1000,
		FuncEvaluations:   10000,
		GradEvaluations:   10000,
	}
	initial := []float64{math.Log(ll.Shape), math.Log(ll.Scale) - meanLog}

	result, err := optimize.Minimize(problem, initial, settings, &optimize.LBFGS{})
	if err != nil && !errors.Is(err, optimize.ErrNoProgress) && !errors.Is(err, optimize.ErrLinesearcherFailure) {
		return nil, fmt.Errorf("BurrXII-MLE-Optimierung fehlgeschlagen: %w: %v", ErrFitFailed, err)
	}

	gamma, b := result.Location.X[0], result.Location.X[1]
	if math.Abs(gamma) > math.Log(burrMaxShape) {
		return nil, fmt.Errorf("BurrXII-Formparameter an der Grenze: %w", ErrFitFailed)
	}
	_, dG, dB, sumS, ok := profile.evaluate(gamma, b)
	if !ok || math.Max(math.Abs(dG), math.Abs(dB))/n > 1e-6 {
		return nil, fmt.Errorf("BurrXII-MLE nicht konvergiert: %w", ErrFitFailed)
	}

	c := math.Exp(gamma)
	k := n / sumS
	if c > burrMaxShape || k > burrMaxShape {
		return nil, fmt.Errorf("BurrXII-Formparameter gegen Grenzwert %g gelaufen (Grenzfall, MLE nicht eindeutig): %w", burrMaxShape, ErrFitFailed)
	}
	dist := BurrXII{Scale: math.Exp(b + meanLog), ShapeC: c, ShapeK: k}
	if !dist.Valid() {
		return nil, fmt.Errorf("BurrXII-Schätzung ungültig: %w", ErrFitFailed)
	}
	return dist, nil
}

// burrMaxShape begrenzt c und k. Für größere Werte nähert sich die Verteilung einer Grenzform
// (Weibull- bzw. Gamma-artig), und die Likelihood hat kein eindeutiges Maximum im Inneren.
const burrMaxShape = 1e6
