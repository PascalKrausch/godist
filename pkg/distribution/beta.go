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
	// boundsEstimated markiert, dass Min/Max per ML aus den Daten geschätzt
	// wurden (BetaFitter ohne vorgegebene Grenzen). Dann zählen sie als
	// freie Parameter für AIC/BIC.
	boundsEstimated bool
}

func (b Beta) Quantile(p float64) float64 {
	if !b.Valid() || !validProbability(p) {
		return math.NaN()
	}
	if p == 0 {
		return b.Min
	}
	if p == 1 {
		return b.Max
	}
	y := bisectCDF(func(y float64) float64 {
		return mathext.RegIncBeta(b.Alpha, b.Beta, y)
	}, p, 0, 1)
	return b.Min + y*(b.Max-b.Min)
}

func (b Beta) Params() []float64 {
	return []float64{b.Alpha, b.Beta, b.Min, b.Max}
}

func (b Beta) ParamNames() []string {
	return []string{"alpha", "beta", "min", "max"}
}

// freeParamCount: Min und Max zählen nur, wenn sie per ML aus den Daten
// geschätzt wurden (boundsEstimated); vorgegebene Grenzen sind fix.
func (b Beta) freeParamCount() int {
	if b.boundsEstimated {
		return 4
	}
	return 2
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
	cleaned := CleanDataWithZero(data)
	if len(cleaned) < 3 {
		return nil, errors.New("zu wenige Datenpunkte für Beta-Fit")
	}

	minVal := f.Min
	maxVal := f.Max

	// Grenzen automatisch bestimmen, falls nicht vorgegeben: Min/Max werden
	// dann als freie ML-Parameter mit α, β ≥ 1 geschätzt (siehe
	// fitBetaAutoBounds). Eine feste Boundary-Dichte verhindert die
	// unbegrenzte Likelihood klassischer Stützpunktschätzung.
	if minVal == 0 && maxVal == 0 {
		return fitBetaAutoBounds(cleaned)
	}

	// Daten auf [0, 1] skalieren und Log-Summen für MLE berechnen
	rangeVal := maxVal - minVal
	var sumLogY, sumLog1mY float64
	for _, x := range cleaned {
		if x <= minVal || x >= maxVal {
			return nil, errors.New("datenpunkte liegen außerhalb der angegebenen Min/Max-Grenzen")
		}
		y := (x - minVal) / rangeVal
		sumLogY += math.Log(y)
		sumLog1mY += math.Log1p(-y)
	}

	// Momentenmethode im skalierten Raum [0, 1]
	mean := stat.Mean(cleaned, nil)
	variance := stat.Variance(cleaned, nil)

	// Transformiere mean und variance von [minVal, maxVal] nach [0, 1]
	scaledMean := (mean - minVal) / rangeVal
	scaledVariance := variance / (rangeVal * rangeVal)

	if scaledVariance >= scaledMean*(1.0-scaledMean) || scaledVariance <= 0 {
		return nil, errors.New("datenvarianz ist zu hoch/ungültig für Beta-Verteilung")
	}

	// Schätzung für α und β
	commonFactor := (scaledMean * (1.0 - scaledMean) / scaledVariance) - 1.0
	alpha := scaledMean * commonFactor
	betaParam := (1.0 - scaledMean) * commonFactor

	if alpha <= 0 || betaParam <= 0 || !IsFinite(alpha) || !IsFinite(betaParam) {
		return nil, errors.New("beta-parameter konnte nicht geschätzt werden")
	}

	alphaMLE, betaMLE, err := fitBetaMLE(
		sumLogY,
		sumLog1mY,
		alpha,
		betaParam,
		len(cleaned),
	)
	if err != nil {
		return nil, err
	}

	return Beta{
		Alpha: alphaMLE,
		Beta:  betaMLE,
		Min:   minVal,
		Max:   maxVal,
	}, nil
}

// betaNLL ist die negative Log-Likelihood der Beta-Verteilung auf [0, 1]
// mit den Suffizienzstatistiken s1 = Σln(y) und s2 = Σln(1−y).
func betaNLL(alpha, beta, n, s1, s2 float64) float64 {
	return n*mathext.Lbeta(alpha, beta) - (alpha-1)*s1 - (beta-1)*s2
}

// betaNLLGrad liefert den Gradienten von betaNLL nach (alpha, beta).
func betaNLLGrad(alpha, beta, n, s1, s2 float64) (float64, float64) {
	psiSum := mathext.Digamma(alpha + beta)
	return n*(mathext.Digamma(alpha)-psiSum) - s1,
		n*(mathext.Digamma(beta)-psiSum) - s2
}

// betaNLLHess liefert die Hessematrix von betaNLL nach (alpha, beta).
// Die Zielfunktion ist konvex, die Matrix daher positiv semidefinit.
func betaNLLHess(alpha, beta, n float64) (haa, hab, hbb float64) {
	triSum := trigamma(alpha + beta)
	return n * (trigamma(alpha) - triSum), -n * triSum, n * (trigamma(beta) - triSum)
}

// newtonBetaMLE minimiert betaNLL mit gedämpftem Newton-Verfahren und
// Armijo-Backtracking, das α, β > 0 garantiert. Bei nicht positiv
// definiter Hessematrix wird auf den negativen Gradienten zurückgefallen.
func newtonBetaMLE(s1, s2, n, alpha, beta float64) (float64, float64, error) {
	const (
		maxIter     = 200
		armijoC     = 1e-4
		gradTol     = 1e-10
		stallGradOK = 1e-6
		maxHalvings = 60
	)

	f := betaNLL(alpha, beta, n, s1, s2)
	for iter := 0; iter < maxIter; iter++ {
		ga, gb := betaNLLGrad(alpha, beta, n, s1, s2)
		if !IsFinite(ga) || !IsFinite(gb) || !IsFinite(f) {
			return 0, 0, errors.New("Beta-MLE-Optimierung fehlgeschlagen: nicht-finite Werte")
		}
		if math.Max(math.Abs(ga), math.Abs(gb))/n <= gradTol {
			return alpha, beta, nil
		}

		haa, hab, hbb := betaNLLHess(alpha, beta, n)
		det := haa*hbb - hab*hab
		da, db := -ga, -gb
		if haa > 0 && det > 0 && IsFinite(det) {
			da = -(hbb*ga - hab*gb) / det
			db = -(-hab*ga + haa*gb) / det
		}
		slope := ga*da + gb*db

		accepted := false
		step := 1.0
		for range maxHalvings {
			na, nb := alpha+step*da, beta+step*db
			if na > 0 && nb > 0 {
				nf := betaNLL(na, nb, n, s1, s2)
				if IsFinite(nf) && nf <= f+armijoC*step*slope {
					alpha, beta, f = na, nb, nf
					accepted = true
					break
				}
			}
			step *= 0.5
		}

		if !accepted {
			// Nahe am Optimum reicht die Rundungsgenauigkeit der
			// Zielfunktion nicht mehr für eine Verbesserung aus.
			ga, gb = betaNLLGrad(alpha, beta, n, s1, s2)
			if math.Max(math.Abs(ga), math.Abs(gb))/n > stallGradOK {
				return 0, 0, errors.New("Beta-MLE-Optimierung fehlgeschlagen: keine Verbesserung")
			}
			return alpha, beta, nil
		}
	}

	return 0, 0, errors.New("Beta-MLE nicht konvergiert: maximale Iterationen erreicht")
}

func fitBetaMLE(
	sumLogY, sumLog1mY float64,
	alpha0, beta0 float64,
	dataLen int,
) (float64, float64, error) {
	if dataLen < 2 {
		return 0, 0, errors.New(
			"zu wenige Datenpunkte für Beta-MLE",
		)
	}

	if alpha0 <= 0 || beta0 <= 0 ||
		math.IsNaN(alpha0) || math.IsNaN(beta0) ||
		math.IsInf(alpha0, 0) || math.IsInf(beta0, 0) {
		return 0, 0, errors.New(
			"ungültige Startwerte für Beta-MLE",
		)
	}

	alpha, betaParam, err := newtonBetaMLE(sumLogY, sumLog1mY, float64(dataLen), alpha0, beta0)
	if err != nil {
		return 0, 0, err
	}

	if !IsFinite(alpha) || !IsFinite(betaParam) ||
		alpha <= 0 || betaParam <= 0 {
		return 0, 0, errors.New(
			"Beta-MLE lieferte ungültige Parameter",
		)
	}

	return alpha, betaParam, nil
}

// betaAutoDeriv enthält Zielfunktion, Gradient und Hessematrix der Beta
// mit geschätzten Grenzen nach x = (α, β, min, max).
type betaAutoDeriv struct {
	f float64
	g [4]float64
	h [4][4]float64
}

// betaAutoFeasible prüft α, β > 1 sowie min < dataMin und max > dataMax.
func betaAutoFeasible(x [4]float64, dataMin, dataMax float64) bool {
	alpha, beta, lo, hi := x[0], x[1], x[2], x[3]
	return alpha > 1 && beta > 1 && lo < dataMin && hi > dataMax &&
		IsFinite(alpha) && IsFinite(beta) && IsFinite(lo) && IsFinite(hi)
}

// betaAutoLogSums liefert ΣlnA_i und ΣlnB_i mit A_i = x_i − lo, B_i = hi − x_i.
// math.Log dominiert die Laufzeit; daher wird das Produkt von je 8 auf (0, 1]
// normierten Faktoren nur einmal logarithmiert (Fehler ≈ 8 ulp je Block, kleiner
// als die Summationsfehler). Drohender Unterlauf fällt auf Einzel-Logarithmen zurück.
func betaAutoLogSums(data []float64, lo, hi float64) (sumLnA, sumLnB float64) {
	const block = 8
	const minProd = 1e-250
	L := hi - lo
	invL := 1 / L

	i := 0
	for ; i+block <= len(data); i += block {
		pa, pb := 1.0, 1.0
		for _, v := range data[i : i+block] {
			pa *= (v - lo) * invL
			pb *= (hi - v) * invL
		}
		if pa > minProd && pb > minProd {
			sumLnA += math.Log(pa)
			sumLnB += math.Log(pb)
			continue
		}
		for _, v := range data[i : i+block] {
			sumLnA += math.Log((v - lo) * invL)
			sumLnB += math.Log((hi - v) * invL)
		}
	}
	for _, v := range data[i:] {
		sumLnA += math.Log((v - lo) * invL)
		sumLnB += math.Log((hi - v) * invL)
	}

	lnL := float64(len(data)) * math.Log(L)
	return sumLnA + lnL, sumLnB + lnL
}

// betaAutoValue wertet nur die Negative-Log-Likelihood aus (ohne Ableitungen).
// Die Arithmetik entspricht betaAutoEval, damit Armijo-Vergleiche konsistent sind.
func betaAutoValue(x [4]float64, data []float64, dataMin, dataMax float64) (float64, bool) {
	if !betaAutoFeasible(x, dataMin, dataMax) {
		return 0, false
	}
	alpha, beta, lo, hi := x[0], x[1], x[2], x[3]
	n := float64(len(data))
	sumLnA, sumLnB := betaAutoLogSums(data, lo, hi)
	if !IsFinite(sumLnA + sumLnB) {
		return 0, false
	}
	c := alpha + beta - 1
	f := n*mathext.Lbeta(alpha, beta) - (alpha-1)*sumLnA - (beta-1)*sumLnB + c*n*math.Log(hi-lo)
	return f, IsFinite(f)
}

// betaAutoEval wertet die Negative-Log-Likelihood samt Ableitungen aus.
// Zulässig sind α, β > 1 sowie min < dataMin und max > dataMax; sonst ist ok false.
//
// Mit A_i = x_i − min, B_i = max − x_i, L = max − min und c = α+β−1 gilt
//
//	NLL = n·Lbeta(α,β) − (α−1)·ΣlnA_i − (β−1)·ΣlnB_i + c·n·ln L
func betaAutoEval(x [4]float64, data []float64, dataMin, dataMax float64) (betaAutoDeriv, bool) {
	var d betaAutoDeriv
	if !betaAutoFeasible(x, dataMin, dataMax) {
		return d, false
	}
	alpha, beta, lo, hi := x[0], x[1], x[2], x[3]

	n := float64(len(data))
	L := hi - lo
	sumLnA, sumLnB := betaAutoLogSums(data, lo, hi)
	var sumInvA, sumInvB, sumInvA2, sumInvB2 float64
	for _, v := range data {
		ia := 1 / (v - lo)
		ib := 1 / (hi - v)
		sumInvA += ia
		sumInvB += ib
		sumInvA2 += ia * ia
		sumInvB2 += ib * ib
	}
	if !IsFinite(sumLnA + sumLnB + sumInvA + sumInvB + sumInvA2 + sumInvB2) {
		return d, false
	}

	s := alpha + beta
	c := s - 1
	lnL := math.Log(L)
	d.f = n*mathext.Lbeta(alpha, beta) - (alpha-1)*sumLnA - (beta-1)*sumLnB + c*n*lnL
	if !IsFinite(d.f) {
		return d, false
	}

	psiS := mathext.Digamma(s)
	triS := trigamma(s)
	nL := n / L
	nL2 := n / (L * L)
	d.g = [4]float64{
		n*(mathext.Digamma(alpha)-psiS) - sumLnA + n*lnL,
		n*(mathext.Digamma(beta)-psiS) - sumLnB + n*lnL,
		(alpha-1)*sumInvA - c*nL,
		-(beta-1)*sumInvB + c*nL,
	}
	d.h = [4][4]float64{
		{n * (trigamma(alpha) - triS), -n * triS, sumInvA - nL, nL},
		{-n * triS, n * (trigamma(beta) - triS), -nL, -sumInvB + nL},
		{sumInvA - nL, -nL, (alpha-1)*sumInvA2 - c*nL2, c * nL2},
		{nL, -sumInvB + nL, c * nL2, (beta-1)*sumInvB2 - c*nL2},
	}
	return d, true
}

// cholSolve4 löst a·x = b für symmetrisch positiv definites a per Cholesky-
// Zerlegung a = L·Lᵀ, vollständig auf dem Stack. ok ist false, wenn a nicht
// positiv definit ist.
func cholSolve4(a [4][4]float64, b [4]float64) ([4]float64, bool) {
	var l [4][4]float64
	var minD, maxD float64
	for j := range 4 {
		s := a[j][j]
		for k := range j {
			s -= l[j][k] * l[j][k]
		}
		if !(s > 0) || !IsFinite(s) {
			return [4]float64{}, false
		}
		if j == 0 || s < minD {
			minD = s
		}
		maxD = max(maxD, s)
		l[j][j] = math.Sqrt(s)
		for i := j + 1; i < 4; i++ {
			t := a[i][j]
			for k := range j {
				t -= l[i][k] * l[j][k]
			}
			l[i][j] = t / l[j][j]
		}
	}
	// Wie gonum (Toleranz 1e16 auf die Konditionszahl): schlecht konditionierte
	// Matrizen werden abgelehnt, damit λ erhöht wird.
	if minD < 1e-16*maxD {
		return [4]float64{}, false
	}

	var y, x [4]float64
	for i := range 4 {
		s := b[i]
		for k := range i {
			s -= l[i][k] * y[k]
		}
		y[i] = s / l[i][i]
	}
	for i := 3; i >= 0; i-- {
		s := y[i]
		for k := i + 1; k < 4; k++ {
			s -= l[k][i] * x[k]
		}
		x[i] = s / l[i][i]
	}
	return x, true
}

// betaAutoNewtonDir löst (H_ff + λ·D)·d_f = −g_f auf den freien Variablen f
// mit dem kleinsten λ aus {0, 1e-8, 1e-7, …}, für das die Matrix positiv
// definit ist. D = diag(|H_ii|) skaliert λ relativ zur Krümmung jeder Variablen.
// Das System wird intern auf Einheitsdiagonale äquilibriert: Die Krümmung von
// min/max (∝ 1/A²) liegt am Datenrand um Größenordnungen über der von α, β,
// sodass sonst die Konditionsprüfung greift und λ die flachen Richtungen
// unnötig stark dämpft. Aktive Variablen erhalten d = 0.
func betaAutoNewtonDir(g [4]float64, h [4][4]float64, active [4]bool) ([4]float64, bool) {
	var sc [4]float64
	for i := range 4 {
		sc[i] = 1
		if a := math.Abs(h[i][i]); !active[i] && a > 0 && IsFinite(a) {
			sc[i] = 1 / math.Sqrt(a)
		}
	}

	var rhs [4]float64
	for i := range 4 {
		if !active[i] {
			rhs[i] = -g[i] * sc[i]
		}
	}

	lambda := 0.0
	for range 60 {
		var m [4][4]float64
		for i := range 4 {
			for j := i; j < 4; j++ {
				v := h[i][j] * sc[i] * sc[j]
				switch {
				case active[i] && i == j:
					v = 1
				case active[i] || active[j]:
					v = 0
				case i == j:
					v += lambda
				}
				m[i][j] = v
				m[j][i] = v
			}
		}
		if y, ok := cholSolve4(m, rhs); ok {
			var d [4]float64
			for i := range 4 {
				d[i] = y[i] * sc[i]
			}
			return d, true
		}

		if lambda == 0 {
			lambda = 1e-8
		} else {
			lambda *= 10
		}
	}
	return [4]float64{}, false
}

// betaAutoActive markiert Variablen, die an ihrer Randbedingung liegen und
// deren Gradient in Richtung des Randes zeigt. Diese werden im Newton-Schritt
// fixiert, damit die Schritte nicht gegen den Rand laufen.
func betaAutoActive(g [4]float64, x [4]float64, dataMin, dataMax float64) [4]bool {
	const tol = 1e-9
	span := dataMax - dataMin
	return [4]bool{
		x[0]-1 <= tol && g[0] > 0,
		x[1]-1 <= tol && g[1] > 0,
		dataMin-x[2] <= tol*span && g[2] < 0,
		x[3]-dataMax <= tol*span && g[3] > 0,
	}
}

// betaAutoPushedOut markiert randnahe, noch freie Variablen als aktiv, deren
// Richtungskomponente aus dem zulässigen Bereich herausführt. Liefert true,
// wenn sich die aktive Menge geändert hat.
func betaAutoPushedOut(x, d [4]float64, active *[4]bool, dataMin, dataMax float64) bool {
	const tol = 1e-9
	span := dataMax - dataMin
	out := [4]bool{
		x[0]-1 <= tol && d[0] < 0,
		x[1]-1 <= tol && d[1] < 0,
		dataMin-x[2] <= tol*span && d[2] > 0,
		x[3]-dataMax <= tol*span && d[3] < 0,
	}
	changed := false
	for i := range 4 {
		if out[i] && !active[i] {
			active[i] = true
			changed = true
		}
	}
	return changed
}

// betaAutoFreeGradNorm ist der größte Gradientenbetrag pro Beobachtung über
// die freien Variablen.
func betaAutoFreeGradNorm(g [4]float64, active [4]bool, n float64) float64 {
	m := 0.0
	for i := range 4 {
		if !active[i] {
			m = max(m, math.Abs(g[i]))
		}
	}
	return m / n
}

var errBetaDegenerate = errors.New(
	"beta-grenzen laufen in einen entarteten grenzfall (gamma-ähnlich)",
)

// betaAutoDegenerate erkennt den Grenzfall α, β → ∞ bzw. unendlicher Grenzen.
func betaAutoDegenerate(x [4]float64, dataRange float64) bool {
	return x[0] > 1e4 || x[1] > 1e4 || x[3]-x[2] > 1e3*dataRange
}

// betaAutoExpand verlängert einen akzeptierten vollen Newton-Schritt (x = x₀ + d)
// entlang d, solange der Gradient noch deutlich bergab zeigt (Wolfe-Krümmungs-
// bedingung) und f weiter sinkt. In gekrümmten, flachen Tälern (z. B. Daten nahe
// der Gamma-Grenze) wächst der Schritt so geometrisch statt linear.
func betaAutoExpand(x [4]float64, cur betaAutoDeriv, d [4]float64, slope float64, data []float64, dataMin, dataMax float64) ([4]float64, betaAutoDeriv) {
	var dg float64
	for i := range 4 {
		dg += cur.g[i] * d[i]
	}
	if !(dg < 0.5*slope) {
		return x, cur
	}

	var x0 [4]float64
	for i := range 4 {
		x0[i] = x[i] - d[i]
	}
	bestT, bestF := 1.0, cur.f
	for t := 2.0; t <= 1<<20; t *= 2 {
		var nx [4]float64
		for i := range 4 {
			nx[i] = x0[i] + t*d[i]
		}
		fv, ok := betaAutoValue(nx, data, dataMin, dataMax)
		if !ok || !(fv < bestF) {
			break
		}
		bestT, bestF = t, fv
	}
	if bestT == 1 {
		return x, cur
	}

	var nx [4]float64
	for i := range 4 {
		nx[i] = x0[i] + bestT*d[i]
	}
	if next, ok := betaAutoEval(nx, data, dataMin, dataMax); ok {
		return nx, next
	}
	return x, cur
}

// newtonBetaAuto minimiert die Negative-Log-Likelihood mit gedämpftem
// Newton-Verfahren in (α, β, min, max). Schritte, die die Zulässigkeit
// verletzen, werden per Armijo-Backtracking verworfen. Ist die Hessematrix
// nicht positiv definit, wird sie regularisiert oder auf den negativen
// Gradienten zurückgefallen.
func newtonBetaAuto(data []float64, dataMin, dataMax float64, x [4]float64) ([4]float64, error) {
	const (
		maxIter     = 500
		armijoC     = 1e-4
		gradTol     = 1e-10
		stallGradOK = 1e-6
		// stallGradLoose begrenzt das Kriterium über die vorhergesagte Verbesserung.
		stallGradLoose = 1e-4
		maxHalvings    = 60
	)

	n := float64(len(data))
	cur, ok := betaAutoEval(x, data, dataMin, dataMax)
	if !ok {
		return x, errors.New("Beta-MLE-Optimierung fehlgeschlagen: ungültige Startwerte")
	}

	for range maxIter {
		active := betaAutoActive(cur.g, x, dataMin, dataMax)
		gnorm := betaAutoFreeGradNorm(cur.g, active, n)
		if !IsFinite(gnorm) {
			return x, errors.New("Beta-MLE-Optimierung fehlgeschlagen: nicht-finite Werte")
		}
		if gnorm <= gradTol {
			return x, nil
		}

		d, ok := betaAutoNewtonDir(cur.g, cur.h, active)
		// Projiziertes Newton-Verfahren: Randnahe Variablen, die die Richtung
		// (durch Kopplung) nach außen schiebt, werden fixiert und d neu berechnet.
		for range 4 {
			if !ok || !betaAutoPushedOut(x, d, &active, dataMin, dataMax) {
				break
			}
			d, ok = betaAutoNewtonDir(cur.g, cur.h, active)
		}
		slope := 0.0
		for i := range 4 {
			slope += cur.g[i] * d[i]
		}
		if !ok || slope >= 0 {
			slope = 0
			for i := range 4 {
				d[i] = 0
				if !active[i] {
					d[i] = -cur.g[i]
					slope -= cur.g[i] * cur.g[i]
				}
			}
		}

		// Die vorhergesagte Verbesserung liegt unter der Rundungsgenauigkeit von f:
		// Eine weitere Iteration kann den Wert nicht mehr darstellbar senken.
		if -slope <= 1e-15*math.Max(1, math.Abs(cur.f)) && gnorm <= stallGradLoose {
			return x, nil
		}

		accepted := false
		fOld := cur.f
		step := 1.0
		for range maxHalvings {
			var nx [4]float64
			for i := range 4 {
				nx[i] = x[i] + step*d[i]
			}
			// Der volle Schritt wird meist akzeptiert; dann spart die Auswertung
			// mit Ableitungen einen zweiten Durchlauf über die Daten.
			if step == 1 {
				if next, ok := betaAutoEval(nx, data, dataMin, dataMax); ok &&
					next.f <= cur.f+armijoC*step*slope {
					x, cur = nx, next
					x, cur = betaAutoExpand(x, cur, d, slope, data, dataMin, dataMax)
					accepted = true
					break
				}
			} else if fv, ok := betaAutoValue(nx, data, dataMin, dataMax); ok &&
				fv <= cur.f+armijoC*step*slope {
				if next, ok := betaAutoEval(nx, data, dataMin, dataMax); ok {
					x, cur = nx, next
					accepted = true
					break
				}
			}
			step *= 0.5
		}

		if !accepted {
			// Nahe am Optimum reicht die Rundungsgenauigkeit der
			// Zielfunktion nicht mehr für eine Verbesserung aus.
			if gnorm > stallGradOK {
				return x, errors.New("Beta-MLE-Optimierung fehlgeschlagen: keine Verbesserung")
			}
			return x, nil
		}
		if betaAutoDegenerate(x, dataMax-dataMin) {
			return x, errBetaDegenerate
		}
		// Schritte verbessern f nur noch im Rundungsbereich: Stillstand nahe dem Optimum.
		if fOld-cur.f <= 1e-15*math.Max(1, math.Abs(cur.f)) && gnorm <= stallGradOK {
			return x, nil
		}
	}

	return x, errors.New("Beta-MLE nicht konvergiert: maximale Iterationen erreicht")
}

// fitBetaAutoBounds schätzt Alpha, Beta, Min und Max gemeinsam per
// Maximum-Likelihood. Aufgerufen, wenn BetaFitter keine Grenzen vorgibt.
func fitBetaAutoBounds(data []float64) (ContinuousDistribution, error) {
	dataMin := data[0]
	dataMax := data[0]
	for _, v := range data {
		if v < dataMin {
			dataMin = v
		}
		if v > dataMax {
			dataMax = v
		}
	}
	dataRange := dataMax - dataMin
	if !(dataRange > 0) || !IsFinite(dataRange) {
		return nil, errors.New("daten haben keine Variation für Beta-Fit")
	}

	// Startgrenzen: kleiner Rand um die Daten (wie bisherige Heuristik,
	// der ML optimiert sie anschließend als freie Parameter).
	margin0 := 0.01 * dataRange
	startMin := dataMin - margin0
	startMax := dataMax + margin0

	// Startwerte α, β aus der Momentenmethode auf [0, 1].
	// α, β müssen > 1 sein (Parametrisierung α = 1 + exp(z)); Werte
	// unter 1 werden auf 1.05 gekappt.
	mean := stat.Mean(data, nil)
	variance := stat.Variance(data, nil)
	scaledMean := (mean - startMin) / (startMax - startMin)
	scaledVariance := variance / ((startMax - startMin) * (startMax - startMin))

	alpha0, beta0 := 2.0, 2.0
	if scaledVariance > 0 && scaledVariance < scaledMean*(1.0-scaledMean) {
		commonFactor := (scaledMean*(1.0-scaledMean)/scaledVariance - 1.0)
		a := scaledMean * commonFactor
		b := (1.0 - scaledMean) * commonFactor
		if a > 1.05 && IsFinite(a) {
			alpha0 = a
		}
		if b > 1.05 && IsFinite(b) {
			beta0 = b
		}
	}

	x0 := [4]float64{alpha0, beta0, startMin, startMax}
	x, err := newtonBetaAuto(data, dataMin, dataMax, x0)
	if err != nil {
		return nil, err
	}
	if _, ok := betaAutoEval(x, data, dataMin, dataMax); !ok {
		return nil, errors.New("Beta-MLE lieferte ungültige Parameter")
	}

	// Degenerationsguard: Der Parameterraum ist nicht kompakt. Läuft die
	// Optimierung in den Grenzfall α, β → ∞ mit entsprechend unendlichen
	// Grenzen, konvergiert die Beta gegen eine andere Familie (z. B.
	// Gamma mit verschobenem Nullpunkt), die bereits eigener Kandidat
	// ist. Ein endlicher Fit wird daher abgelehnt.
	if betaAutoDegenerate(x, dataRange) {
		return nil, errBetaDegenerate
	}

	return Beta{
		Alpha:           x[0],
		Beta:            x[1],
		Min:             x[2],
		Max:             x[3],
		boundsEstimated: true,
	}, nil
}
