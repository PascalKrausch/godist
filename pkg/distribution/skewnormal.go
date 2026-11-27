package distribution

import (
	"errors"
	"fmt"
	"math"

	"gonum.org/v1/gonum/optimize"
)

// SkewNormal repräsentiert eine schiefe Normalverteilung (Azzalini) mit Lageparameter Xi,
// Skalierungsparameter Omega (ω > 0) und Schiefeparameter Alpha (α ∈ ℝ). Die Dichte ist
// f(x) = 2/ω · φ(z) · Φ(αz) mit z = (x − ξ)/ω. Für α = 0 ergibt sich die Normalverteilung,
// für α → ±∞ eine halbseitige Normalverteilung.
type SkewNormal struct {
	Xi    float64 // Lageparameter ξ
	Omega float64 // Skalierungsparameter ω > 0
	Alpha float64 // Schiefeparameter α
}

func (s SkewNormal) Valid() bool {
	return s.Omega > 0 && IsFinite(s.Xi) && IsFinite(s.Omega) && IsFinite(s.Alpha)
}

func (s SkewNormal) Params() []float64 {
	return []float64{s.Xi, s.Omega, s.Alpha}
}

func (s SkewNormal) ParamNames() []string {
	return []string{"xi", "omega", "alpha"}
}

func (s SkewNormal) PDF(x float64) float64 {
	if !s.Valid() {
		return 0
	}
	return math.Exp(s.LogPDF(x))
}

func (s SkewNormal) LogPDF(x float64) float64 {
	if !s.Valid() {
		return math.Inf(-1)
	}
	z := (x - s.Xi) / s.Omega
	return math.Log(2) - math.Log(s.Omega) - 0.5*z*z - 0.5*math.Log(2*math.Pi) + logPhi(s.Alpha*z)
}

// CDF nutzt die Darstellung F(z) = Φ(z) − 2·T(z, α) mit Owen's T-Funktion T.
func (s SkewNormal) CDF(x float64) float64 {
	if !s.Valid() {
		return 0
	}
	return stdSkewNormalCDF((x-s.Xi)/s.Omega, s.Alpha)
}

// stdSkewNormalCDF ist die CDF der standardisierten Schiefnormalverteilung (ξ = 0, ω = 1).
func stdSkewNormalCDF(z, alpha float64) float64 {
	cdf := 0.5*math.Erfc(-z/math.Sqrt2) - 2*owenT(z, alpha)
	if cdf < 0 {
		return 0
	}
	if cdf > 1 {
		return 1
	}
	return cdf
}

// Quantile invertiert die CDF per Bisektion auf z; es gibt keine geschlossene Form.
func (s SkewNormal) Quantile(p float64) float64 {
	if !s.Valid() || !validProbability(p) {
		return math.NaN()
	}
	switch p {
	case 0:
		return math.Inf(-1)
	case 1:
		return math.Inf(1)
	}
	cdf := func(z float64) float64 { return stdSkewNormalCDF(z, s.Alpha) }
	lo, hi := -1.0, 1.0
	for cdf(lo) > p {
		lo *= 2
	}
	for cdf(hi) < p {
		hi *= 2
	}
	for i := 0; i < 200 && hi-lo > 1e-13*math.Max(1, math.Abs(lo)); i++ {
		mid := 0.5 * (lo + hi)
		if cdf(mid) < p {
			lo = mid
		} else {
			hi = mid
		}
	}
	return s.Xi + s.Omega*0.5*(lo+hi)
}

// millsRatio ist λ(t) = φ(t)/Φ(t), der Quotient aus Standarddichte und -verteilung.
// Für t → −∞ wächst λ wie −t; der Logarithmus wird daher nie als Differenz großer Terme
// mit entgegengesetzten Vorzeichen benötigt.
func millsRatio(t float64) float64 {
	logPDF := -0.5*t*t - 0.5*math.Log(2*math.Pi)
	return math.Exp(logPDF - logPhi(t))
}

// owenT berechnet Owen's T-Funktion
//
//	T(h, a) = 1/(2π) ∫₀^a exp(−h²(1+x²)/2) / (1+x²) dx.
//
// Der Integrand stirbt für x >~ 12/|h|; darüber ist der Rest kleiner als e^−72 und wird
// vernachlässigt. Für sehr großes a wird T(h, ∞) = ¼·erfc(|h|/√2) verwendet.
func owenT(h, a float64) float64 {
	if a == 0 || math.IsNaN(a) {
		return 0
	}
	if a < 0 {
		return -owenT(h, -a)
	}
	if math.IsInf(a, 1) || a > 1e15 {
		return 0.25 * math.Erfc(math.Abs(h)/math.Sqrt2)
	}
	upper := a
	if ah := math.Abs(h); ah > 0 {
		if lim := 12 / ah; lim < upper {
			upper = lim
		}
	}
	return simpsonAdaptive(owenTIntegrand(h), 0, upper) / (2 * math.Pi)
}

func owenTIntegrand(h float64) func(float64) float64 {
	h2 := h * h
	return func(x float64) float64 {
		return math.Exp(-0.5*h2*(1+x*x)) / (1 + x*x)
	}
}

// simpsonAdaptive ist eine adaptive Simpson-Quadratur mit absoluter Toleranz
// auf dem Integralwert; maxDepth begrenzt die Rekursion für sehr große Intervalle.
// tol = 1e-12 reicht für die CDF-Genauigkeit (KS/RMSE); 1e-13 verdoppelte
// praktisch die Funktionsauswertungen pro T(h, a).
func simpsonAdaptive(f func(float64) float64, a, b float64) float64 {
	const tol = 1e-12
	const maxDepth = 200
	m := 0.5 * (a + b)
	fa, fm, fb := f(a), f(m), f(b)
	whole := (b - a) / 6 * (fa + 4*fm + fb)

	var rec func(a, b, fa, fm, fb, s float64, depth int) float64
	rec = func(a, b, fa, fm, fb, s float64, depth int) float64 {
		m := 0.5 * (a + b)
		lm := 0.5 * (a + m)
		rm := 0.5 * (m + b)
		flm, frm := f(lm), f(rm)
		sl := (m - a) / 6 * (fa + 4*flm + fm)
		sr := (b - m) / 6 * (fm + 4*frm + fb)
		if depth <= 0 || math.Abs(sl+sr-s) <= 15*tol {
			return sl + sr + (sl+sr-s)/15
		}
		return rec(a, m, fa, flm, fm, sl, depth-1) + rec(m, b, fm, frm, fb, sr, depth-1)
	}
	return rec(a, b, fa, fm, fb, whole, maxDepth)
}

// SkewNormalFitter schätzt Xi, Omega und Alpha per Maximum-Likelihood. Die Startwerte
// stammen aus den Momenten (Delta-Methode); die Optimierung läuft per L-BFGS auf
// (ξ, log ω, α) mit analytischem Gradienten der Log-Likelihood.
type SkewNormalFitter struct{}

func (f SkewNormalFitter) Name() string {
	return "SkewNormal"
}

const (
	// skewNormalAlphaBound begrenzt α in der Optimierung; darüber ist die Verteilung
	// praktisch halbseitig normal und die Likelihood profiling‐flach.
	skewNormalAlphaBound = 1e6
	skewNormalMaxIter    = 500
)

// skewNormalLogLikGrad liefert die Log-Likelihood und deren Summen-Gradienten
// bezüglich (ξ, log ω, α). ok ist false, wenn die Parameter ungültig sind oder
// die Rechnung nicht endlich bleibt.
func skewNormalLogLikGrad(data []float64, xi, omega, alpha float64) (ll, gXi, gTheta, gAlpha float64, ok bool) {
	if !(omega > 0) || !IsFinite(xi) || !IsFinite(omega) || !IsFinite(alpha) {
		return 0, 0, 0, 0, false
	}
	constPerObs := math.Log(2) - math.Log(omega) - 0.5*math.Log(2*math.Pi)
	for _, x := range data {
		z := (x - xi) / omega
		t := alpha * z
		// logΦ(t) wird einmal gebraucht: für den ll-Term und für die Mills-Ratio
		// λ(t) = φ(t)/Φ(t) = exp(−t²/2 − ½ln(2π) − logΦ(t)).
		logPhiT := logPhi(t)
		lam := math.Exp(-0.5*t*t - 0.5*math.Log(2*math.Pi) - logPhiT)
		if math.IsNaN(lam) || math.IsInf(lam, 0) {
			return 0, 0, 0, 0, false
		}
		ll += constPerObs - 0.5*z*z + logPhiT
		gXi += (z - alpha*lam) / omega
		gTheta += z*z - z*alpha*lam - 1
		gAlpha += z * lam
	}
	return ll, gXi, gTheta, gAlpha, IsFinite(ll) && IsFinite(gXi) && IsFinite(gTheta) && IsFinite(gAlpha)
}

// skewNormalProfile cached das letzte Ergebnis von skewNormalLogLikGrad: Func und
// Grad fragen im L-BFGS am selben Punkt (ξ, log ω, α) ab, sodass der O(n)-Pass pro
// Optimierungsschritt nur einmal läuft. Einträge sind bitgenau gekennzeichnet;
// abweichende Punkte sind schlicht Cache-Misses.
type skewNormalProfile struct {
	data []float64

	cacheXi, cacheTheta, cacheAlpha             float64
	cacheLL, cacheGXi, cacheGTheta, cacheGAlpha float64
	cacheOK, cacheValid                         bool
}

func (p *skewNormalProfile) eval(xi, theta, alpha float64) (ll, gXi, gTheta, gAlpha float64, ok bool) {
	if p.cacheValid && p.cacheXi == xi && p.cacheTheta == theta && p.cacheAlpha == alpha {
		return p.cacheLL, p.cacheGXi, p.cacheGTheta, p.cacheGAlpha, p.cacheOK
	}
	ll, gXi, gTheta, gAlpha, ok = skewNormalLogLikGrad(p.data, xi, math.Exp(theta), alpha)
	p.cacheXi, p.cacheTheta, p.cacheAlpha = xi, theta, alpha
	p.cacheLL, p.cacheGXi, p.cacheGTheta, p.cacheGAlpha, p.cacheOK, p.cacheValid =
		ll, gXi, gTheta, gAlpha, ok, true
	return ll, gXi, gTheta, gAlpha, ok
}

func (f SkewNormalFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	if len(cleaned) == 0 {
		return nil, ErrEmptyData
	}
	if len(cleaned) < 3 {
		return nil, fmt.Errorf("SkewNormal benötigt mindestens drei Beobachtungen: %w", ErrFitFailed)
	}

	lo, hi := cleaned[0], cleaned[0]
	var sum float64
	for _, x := range cleaned {
		lo = math.Min(lo, x)
		hi = math.Max(hi, x)
		sum += x
	}
	if hi == lo {
		return nil, fmt.Errorf("SkewNormal-Parameter konnten nicht geschätzt werden: %w", ErrFitFailed)
	}
	mean := sum / float64(len(cleaned))

	// Momenten-Startwerte: Var = ω²(1 − 2δ²/π), E[X] = ξ + ωδ√(2/π), δ = α/√(1+α²).
	// Die Schiefe bestimmt δ; u = (δ√(2/π))² muss unter 2/π bleiben, damit δ < 1 gilt.
	var m2, m3 float64
	for _, x := range cleaned {
		d := x - mean
		m2 += d * d
		m3 += d * d * d
	}
	n := float64(len(cleaned))
	m2 /= n
	m3 /= n
	sd := math.Sqrt(m2)
	g1 := m3 / (m2 * sd)

	b := math.Sqrt(2 / math.Pi)
	q := math.Pow(math.Abs(g1)*2/(4-math.Pi), 2.0/3.0)
	u := q / (1 + q)
	if u > 0.636 {
		u = 0.636 // flankiert die theoretisch maximale Schiefe der Familie
	}
	deltaB := math.Copysign(math.Sqrt(u), g1) // = δ·√(2/π)
	delta2 := u / (b * b)
	alpha0 := math.Copysign(math.Sqrt(delta2/(1-delta2)), g1)
	omega0 := sd / math.Sqrt(1-u)
	xi0 := mean - omega0*deltaB
	if !IsFinite(xi0) || !IsFinite(omega0) || !IsFinite(alpha0) || omega0 <= 0 {
		return nil, fmt.Errorf("SkewNormal-Startwerte ungültig: %w", ErrFitFailed)
	}

	norm := 1 / n
	profile := &skewNormalProfile{data: cleaned}
	problem := optimize.Problem{
		Func: func(v []float64) float64 {
			if math.Abs(v[2]) > skewNormalAlphaBound {
				return math.Inf(1)
			}
			ll, _, _, _, ok := profile.eval(v[0], v[1], v[2])
			if !ok {
				return math.Inf(1)
			}
			return -ll * norm
		},
		Grad: func(grad, v []float64) {
			_, gXi, gTheta, gAlpha, ok := profile.eval(v[0], v[1], v[2])
			if !ok {
				grad[0], grad[1], grad[2] = math.Inf(1), math.Inf(1), math.Inf(1)
				return
			}
			grad[0] = -gXi * norm
			grad[1] = -gTheta * norm
			grad[2] = -gAlpha * norm
		},
	}

	settings := &optimize.Settings{
		GradientThreshold: 1e-10,
		MajorIterations:   skewNormalMaxIter,
		FuncEvaluations:   20000,
		GradEvaluations:   20000,
	}
	initial := []float64{xi0, math.Log(omega0), alpha0}

	result, err := optimize.Minimize(problem, initial, settings, &optimize.LBFGS{})
	if err != nil && !errors.Is(err, optimize.ErrNoProgress) && !errors.Is(err, optimize.ErrLinesearcherFailure) {
		return nil, fmt.Errorf("SkewNormal-MLE-Optimierung fehlgeschlagen: %w: %v", ErrFitFailed, err)
	}

	v := result.Location.X
	dist := SkewNormal{Xi: v[0], Omega: math.Exp(v[1]), Alpha: v[2]}
	if !dist.Valid() || math.Abs(dist.Alpha) > skewNormalAlphaBound {
		return nil, fmt.Errorf("SkewNormal-Schätzung ungültig: %w", ErrFitFailed)
	}

	// Score-Prüfung: am Maximum müssen die mittleren Score-Komponenten klein sein.
	_, gXi, gTheta, gAlpha, ok := skewNormalLogLikGrad(cleaned, dist.Xi, dist.Omega, dist.Alpha)
	if !ok || math.Max(math.Abs(gXi), math.Max(math.Abs(gTheta), math.Abs(gAlpha)))/n > 1e-4 {
		return nil, fmt.Errorf("SkewNormal-MLE nicht konvergiert: %w", ErrFitFailed)
	}

	return dist, nil
}
