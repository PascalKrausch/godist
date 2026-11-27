package distribution

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"

	"gonum.org/v1/gonum/mathext"
	"gonum.org/v1/gonum/optimize"
)

// GeneralizedGamma repräsentiert eine verallgemeinerte Gamma-Verteilung
// (Stacy, 1962) mit Formparameter Shape > 0, Skalenparameter Scale > 0 und
// Potenzparameter Power ≠ 0. Die Dichte ist
//
//	f(x) = |Power| / (Scale·Γ(Shape)) · (x/Scale)^(Power·Shape − 1) · exp(−(x/Scale)^Power),  x > 0.
//
// Power = 1 liefert die Gamma-Verteilung, Shape = 1 die Weibull-Verteilung.
// Für Power < 0 geht die Dichte bei 0 ebenfalls gegen 0.
type GeneralizedGamma struct {
	Shape float64 // Formparameter p > 0
	Scale float64 // Skalenparameter a > 0
	Power float64 // Potenzparameter d ≠ 0
}

func (g GeneralizedGamma) valid() bool {
	return IsFinite(g.Shape) && IsFinite(g.Scale) && IsFinite(g.Power) &&
		g.Shape > 0 && g.Scale > 0 && g.Power != 0
}

func (g GeneralizedGamma) Params() []float64 {
	return []float64{g.Shape, g.Scale, g.Power}
}

func (g GeneralizedGamma) ParamNames() []string {
	return []string{"shape", "scale", "power"}
}

func (g GeneralizedGamma) PDF(x float64) float64 {
	if !g.valid() || x < 0 {
		return 0
	}
	return math.Exp(g.LogPDF(x))
}

func (g GeneralizedGamma) LogPDF(x float64) float64 {
	if !g.valid() || x < 0 {
		return math.Inf(-1)
	}
	if x == 0 {
		if g.Power < 0 {
			return math.Inf(-1)
		}
		// Wie bei der Gamma-Verteilung entscheidet p·d über den Randwert:
		// < 1 Singularität, = 1 endlicher Wert |d|/(a·Γ(p)), > 1 Dichte 0.
		switch dp := g.Power * g.Shape; {
		case dp < 1:
			return math.Inf(1)
		case dp == 1:
			lgamma, _ := math.Lgamma(g.Shape)
			return math.Log(math.Abs(g.Power)) - math.Log(g.Scale) - lgamma
		default:
			return math.Inf(-1)
		}
	}
	if math.IsInf(x, 1) {
		return math.Inf(-1)
	}
	lgamma, _ := math.Lgamma(g.Shape)
	return math.Log(math.Abs(g.Power)) - math.Log(g.Scale) - lgamma +
		(g.Power*g.Shape-1)*math.Log(x/g.Scale) - math.Pow(x/g.Scale, g.Power)
}

// CDF: Mit U = (x/a)^d ~ Gamma(Shape, 1) gilt für d > 0
// F(x) = P(Shape, U) und für d < 0 F(x) = Q(Shape, U) (obere regulierte Gamma-Funktion).
func (g GeneralizedGamma) CDF(x float64) float64 {
	if !g.valid() || x <= 0 {
		return 0
	}
	if math.IsInf(x, 1) {
		return 1
	}
	u := math.Pow(x/g.Scale, g.Power)
	if g.Power > 0 {
		return mathext.GammaIncReg(g.Shape, u)
	}
	return mathext.GammaIncRegComp(g.Shape, u)
}

// Quantile invertiert die CDF über das Gamma-Quantil: x = a·Q^(1/d).
// Für d < 0 wird das komplementäre Gamma-Quantil verwendet, da F = Q gilt.
func (g GeneralizedGamma) Quantile(p float64) float64 {
	if !g.valid() || !validProbability(p) {
		return math.NaN()
	}
	if p == 0 {
		return 0
	}
	if p == 1 {
		return math.Inf(1)
	}
	if g.Power > 0 {
		return g.Scale * math.Pow(mathext.GammaIncRegInv(g.Shape, p), 1/g.Power)
	}
	return g.Scale * math.Pow(mathext.GammaIncRegCompInv(g.Shape, p), 1/g.Power)
}

// ==========================================
// GENERALIZED GAMMA FITTER
// ==========================================

// GeneralizedGammaFitter schätzt Shape, Scale und Power per Maximum-Likelihood.
// Für festes (p, d) hat die Score-Gleichung in a die geschlossene Lösung
// a^d = Σxᵢ^d / (n·p); (p, d) werden daher per L-BFGS auf der in a profilierten
// Log-Likelihood optimiert. Die Optimierung läuft für d > 0 und d < 0 getrennt,
// da d = 0 (lognormaler Grenzfall) nicht im Definitionsbereich liegt.
type GeneralizedGammaFitter struct{}

func (f GeneralizedGammaFitter) Name() string {
	return "GeneralizedGamma"
}

func (f GeneralizedGammaFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	if err := checkNonNegative(cleaned); err != nil {
		return nil, err
	}
	if len(cleaned) < 3 {
		return nil, fmt.Errorf("GeneralizedGamma benötigt mindestens drei Beobachtungen: %w", ErrFitFailed)
	}

	// Strenge Nullwert-Policy (Fast-Fail): ln(0) ist für die Schätzung nicht definiert.
	logs := make([]float64, len(cleaned))
	var meanLog float64
	for i, x := range cleaned {
		if x == 0 {
			return nil, fmt.Errorf("GeneralizedGamma scheitert bei Nullwerten, da ln(0) nicht definiert ist: %w", ErrFitFailed)
		}
		logs[i] = math.Log(x)
		meanLog += logs[i]
	}
	meanLog /= float64(len(logs))

	// z wird zentriert um meanLog berechnet und in-place in logs gespeichert;
	// min/max werden für die O(1)-Maximumsbestimmung in eval mitgeführt.
	zMin, zMax := math.Inf(1), math.Inf(-1)
	varies := false
	for i, l := range logs {
		z := l - meanLog
		logs[i] = z
		if z < zMin {
			zMin = z
		}
		if z > zMax {
			zMax = z
		}
		if z != 0 {
			varies = true
		}
	}
	if !varies {
		return nil, fmt.Errorf("GeneralizedGamma benötigt variierende Beobachtungen: %w", ErrFitFailed)
	}

	profile := ggProfile{z: logs, n: float64(len(logs)), meanLog: meanLog, zMin: zMin, zMax: zMax}

	// Startwerte: Gamma (d = 1) und Weibull (p = 1) sind Spezialfälle der GG.
	// Für d < 0 gibt es keinen geschachtelten Kandidaten; der Gamma-Start wird
	// mit d = −1 verwendet. Die Startfits werden direkt aus den bereits
	// berechneten logs abgeleitet, statt die Fitter erneut über die Daten laufen
	// zu lassen: Gamma braucht nur Σx und meanLog (Thom-Approximation), und
	// solveWeibullShape ist gegen Verschiebungen der logs invariant, sodass die
	// zentrierten z genügen.
	var best GeneralizedGamma
	bestPhi := math.Inf(-1)
	fitted := false
	tryStart := func(sign, alpha0, gamma0 float64) {
		dist, phi, ok := profile.optimize(sign, alpha0, gamma0)
		if !ok || phi <= bestPhi {
			return
		}
		// Sicherheitscheck: die Likelihood muss an allen Datenpunkten endlich sein,
		// sonst schlägt EvaluateFit später fehl.
		for _, x := range cleaned {
			if !IsFinite(dist.LogPDF(x)) {
				return
			}
		}
		best, bestPhi, fitted = dist, phi, true
	}

	// Gamma-Start: Thom-Indikator s = ln(mean) − meanLog mit Newton-Verfeinerung,
	// äquivalent zu GammaFitter.Fit, aber ohne erneute Daten- und Log-Pässe.
	var sum float64
	for _, x := range cleaned {
		sum += x
	}
	var alpha0Gamma float64
	haveAlpha0 := false
	if s := math.Log(sum/profile.n) - meanLog; s > 0 && IsFinite(s) {
		alpha := (1.0 + math.Sqrt(1.0+4.0*s/3.0)) / (4.0 * s)
		alpha = refineGammaAlpha(alpha, s)
		beta := sum / profile.n / alpha
		if alpha > 0 && IsFinite(alpha) && beta > 0 && IsFinite(beta) {
			alpha0Gamma = math.Log(alpha)
			haveAlpha0 = true
			tryStart(1, alpha0Gamma, 0)
		}
	}
	if shape, ok := solveWeibullShape(profile.z); ok && shape > 0 && IsFinite(shape) {
		tryStart(1, 0, math.Log(shape))
	}
	// Für d < 0 ist die profillierte Likelihood multimodal: ein einzelner
	// Start (z. B. der Gamma-Start mit großem α) kann in ein lokales
	// Optimum laufen und das globale Maximum mit stark negativem d
	// verpassen. Daher wird ein kleines deterministisches Startgitter
	// über (α, γ) probiert; tryStart behält das beste φ.
	alphaStarts := []float64{0}
	if haveAlpha0 {
		alphaStarts = append([]float64{alpha0Gamma}, alphaStarts...)
	}
	for _, a0 := range alphaStarts {
		for _, g0 := range []float64{0, math.Log(5), math.Log(30)} {
			tryStart(-1, a0, g0)
		}
	}
	if !fitted {
		tryStart(1, 0, 0)
		tryStart(-1, 0, 0)
	}
	if !fitted {
		return nil, fmt.Errorf("GeneralizedGamma-MLE-Optimierung fehlgeschlagen: %w", ErrFitFailed)
	}
	return best, nil
}

// ggMaxParam begrenzt p = e^α und |d| = e^γ auf [1e-6, 1e6]. Für |d| → 0 nähert sich
// die GG der Lognormalverteilung an; der Grenzfall wird wie bei BurrXII nicht als
// eigenes Maximum akzeptiert, aber Parametersätze an der Grenze bleiben gültig.
const ggMaxParam = 1e6

// ggProfile hält die logarithmierten, zentrierten Daten z_i = ln x_i − mean(ln x).
// Die profillierte Log-Likelihood wird über (α, γ) = (ln p, ln|d|) optimiert;
// die Skala a ist für festes (p, d) geschlossen bestimmt.
type ggProfile struct {
	z       []float64
	n       float64
	meanLog float64
	zMin    float64
	zMax    float64

	// Cache des letzten eval-Aufrufs: der Optimierer fragt Func und Grad
	// üblicherweise am selben Punkt (α, γ) an, die O(n)-Summen müssen dann
	// nicht doppelt berechnet werden.
	cSign, cAlpha, cGamma      float64
	cPhi, cDAlpha, cDGamma, cL float64
	cOK, cHit                  bool
}

// eval liefert die profillierte Log-Likelihood φ = ℓ/n, deren Gradienten
// bezüglich (α, γ) und L = ln(S₀/(n·p)) für die Skalenrückrechnung.
// ok ist false, wenn die Summen nicht endlich sind.
func (p *ggProfile) eval(sign, alpha, gamma float64) (phi, dAlpha, dGamma, L float64, ok bool) {
	if p.cHit && p.cSign == sign && p.cAlpha == alpha && p.cGamma == gamma {
		return p.cPhi, p.cDAlpha, p.cDGamma, p.cL, p.cOK
	}
	defer func() {
		p.cSign, p.cAlpha, p.cGamma = sign, alpha, gamma
		p.cPhi, p.cDAlpha, p.cDGamma, p.cL = phi, dAlpha, dGamma, L
		p.cOK, p.cHit = ok, true
	}()

	d := sign * math.Exp(gamma)
	shape := math.Exp(alpha)

	// S₀ = Σ e^(d·z_i) und S₁/S₀ = Σ z_i e^(d·z_i) / Σ e^(d·z_i), verschoben
	// berechnet, damit große |d·z_i| nicht überlaufen. Das Maximum von d·z_i
	// liegt in zMin bzw. zMax, weil d·z_i in z_i monoton ist.
	maxDz := d * p.zMax
	if d < 0 {
		maxDz = d * p.zMin
	}
	sw, sz := p.sums(d, maxDz)
	if math.IsNaN(sw) || math.IsNaN(sz) || sw <= 0 || math.IsInf(sw, 0) || math.IsInf(sz, 0) {
		return 0, 0, 0, 0, false
	}

	lnS0 := maxDz + math.Log(sw)
	L = lnS0 - math.Log(p.n) - alpha // ln(S₀/(n·p))
	lgamma, _ := math.Lgamma(shape)
	digamma := mathext.Digamma(shape)

	// φ = ln|d| − meanLog − lgamma(p) − p − p·L  (pro Beobachtung)
	phi = gamma - p.meanLog - lgamma - shape - shape*L
	dAlpha = -shape * (digamma + L)
	dGamma = 1 - d*shape*(sz/sw)
	return phi, dAlpha, dGamma, L, true
}

// ggParallelMin ist die Mindestgröße von z, ab der sums die Exponentialsumme
// auf mehrere Goroutinen verteilt. Darunter überwiegt der Synchronisations-
// overhead den Gewinn.
const ggParallelMin = 1 << 13

// ggMaxWorkers begrenzt die Parallelität innerhalb eines eval-Aufrufs.
const ggMaxWorkers = 8

// sums berechnet S₀ = Σ e^(d·z_i − maxDz) und S₁ = Σ z_i·e^(d·z_i − maxDz).
// Die Aufteilung in zusammenhängende Blöcke ist deterministisch; die
// Summationsreihenfolge weicht daher leicht von der sequenziellen Variante ab.
func (p *ggProfile) sums(d, maxDz float64) (sw, sz float64) {
	z := p.z
	n := len(z)
	if n < ggParallelMin {
		for _, zi := range z {
			e := math.Exp(d*zi - maxDz)
			sw += e
			sz += zi * e
		}
		return sw, sz
	}

	workers := runtime.GOMAXPROCS(0)
	if workers > ggMaxWorkers {
		workers = ggMaxWorkers
	}
	chunk := (n + workers - 1) / workers
	nw := (n + chunk - 1) / chunk

	var parts [ggMaxWorkers]struct{ sw, sz float64 }
	var wg sync.WaitGroup
	for w := 0; w < nw; w++ {
		lo, hi := w*chunk, min((w+1)*chunk, n)
		wg.Add(1)
		go func(w, lo, hi int) {
			defer wg.Done()
			var sww, szz float64
			for _, zi := range z[lo:hi] {
				e := math.Exp(d*zi - maxDz)
				sww += e
				szz += zi * e
			}
			parts[w].sw, parts[w].sz = sww, szz
		}(w, lo, hi)
	}
	wg.Wait()
	for i := 0; i < nw; i++ {
		sw += parts[i].sw
		sz += parts[i].sz
	}
	return sw, sz
}

// optimize führt einen L-BFGS-Lauf für das vorgegebene Vorzeichen von d aus
// und rekonstruiert bei Erfolg das GeneralizedGamma-Modell.
func (p *ggProfile) optimize(sign, alpha0, gamma0 float64) (GeneralizedGamma, float64, bool) {
	problem := optimize.Problem{
		Func: func(x []float64) float64 {
			if math.Abs(x[0]) > math.Log(ggMaxParam) || math.Abs(x[1]) > math.Log(ggMaxParam) {
				return math.Inf(1)
			}
			phi, _, _, _, ok := p.eval(sign, x[0], x[1])
			if !ok || math.IsNaN(phi) {
				return math.Inf(1)
			}
			return -phi
		},
		Grad: func(grad, x []float64) {
			_, gA, gG, _, ok := p.eval(sign, x[0], x[1])
			if !ok {
				grad[0] = math.Inf(1)
				grad[1] = math.Inf(1)
				return
			}
			grad[0] = -gA
			grad[1] = -gG
		},
	}

	settings := &optimize.Settings{
		GradientThreshold: 1e-10,
		MajorIterations:   1000,
		FuncEvaluations:   10000,
		GradEvaluations:   10000,
	}
	result, err := optimize.Minimize(problem, []float64{alpha0, gamma0}, settings, &optimize.LBFGS{})
	if err != nil && !errors.Is(err, optimize.ErrNoProgress) && !errors.Is(err, optimize.ErrLinesearcherFailure) {
		return GeneralizedGamma{}, 0, false
	}

	alpha, gamma := result.Location.X[0], result.Location.X[1]
	if math.Abs(alpha) > math.Log(ggMaxParam) || math.Abs(gamma) > math.Log(ggMaxParam) {
		return GeneralizedGamma{}, 0, false
	}
	phi, _, _, L, ok := p.eval(sign, alpha, gamma)
	if !ok {
		return GeneralizedGamma{}, 0, false
	}

	d := sign * math.Exp(gamma)
	dist := GeneralizedGamma{
		Shape: math.Exp(alpha),
		Scale: math.Exp(p.meanLog + L/d),
		Power: d,
	}
	if !dist.valid() {
		return GeneralizedGamma{}, 0, false
	}
	return dist, phi, true
}
