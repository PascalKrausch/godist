package distribution

import (
	"errors"
	"fmt"
	"math"

	"gonum.org/v1/gonum/optimize"
	"gonum.org/v1/gonum/stat"
)

// GEV repräsentiert eine verallgemeinerte Extremwertverteilung (GEV) mit
// Lageparameter Mu, Skalenparameter Sigma > 0 und Formparameter Xi. Die Dichte ist
//
//	f(x) = (1/σ) · t(x)^(ξ+1) · exp(−t(x))  mit  t(x) = (1 + ξ(x−μ)/σ)^(−1/ξ)
//
// für ξ ≠ 0; für ξ = 0 geht sie in die Gumbel-Verteilung über. Für ξ > 0
// (Fréchet-Typ) ist der Träger nach unten beschränkt, für ξ < 0 (Weibull-Typ)
// nach oben. Die Verteilung ist für Extremwerte (Blockmaxima) gedacht.
type GEV struct {
	Mu    float64 // Lageparameter μ
	Sigma float64 // Skalenparameter σ > 0
	Xi    float64 // Formparameter ξ; ξ = 0 ergibt die Gumbel-Verteilung
}

func (g GEV) valid() bool {
	return IsFinite(g.Mu) && IsFinite(g.Sigma) && IsFinite(g.Xi) && g.Sigma > 0
}

func (g GEV) Params() []float64 {
	return []float64{g.Mu, g.Sigma, g.Xi}
}

func (g GEV) ParamNames() []string {
	return []string{"mu", "sigma", "xi"}
}

// gevTerms berechnet z = (x−μ)/σ und, für ξ ≠ 0, s = ln(1+ξz)/ξ sowie t = e^(−s).
// ok ist false, wenn x außerhalb des Trägers liegt (1+ξz ≤ 0) oder die Rechnung
// nicht endlich bleibt.
func (g GEV) gevTerms(x float64) (z, s, t float64, ok bool) {
	z = (x - g.Mu) / g.Sigma
	if g.Xi == 0 {
		return z, z, math.Exp(-z), true
	}
	v := g.Xi * z
	if v <= -1 || math.IsNaN(v) {
		return 0, 0, 0, false
	}
	s = math.Log1p(v) / g.Xi
	t = math.Exp(-s)
	if math.IsNaN(t) {
		return 0, 0, 0, false
	}
	return z, s, t, true
}

func (g GEV) PDF(x float64) float64 {
	if !g.valid() {
		return 0
	}
	return math.Exp(g.LogPDF(x))
}

func (g GEV) LogPDF(x float64) float64 {
	if !g.valid() {
		return math.Inf(-1)
	}
	z, s, t, ok := g.gevTerms(x)
	if !ok {
		return math.Inf(-1)
	}
	if math.IsInf(t, 1) {
		// t → ∞ am endlichen Trägerrand: die Dichte geht gegen 0.
		return math.Inf(-1)
	}
	if g.Xi == 0 {
		return -math.Log(g.Sigma) - z - t
	}
	return -math.Log(g.Sigma) - (g.Xi+1)*s - t
}

// CDF: Mit t(x) = (1+ξz)^(−1/ξ) gilt F(x) = exp(−t(x)); für ξ = 0 die Gumbel-CDF.
func (g GEV) CDF(x float64) float64 {
	if !g.valid() {
		return 0
	}
	if g.Xi == 0 {
		return math.Exp(-math.Exp(-(x - g.Mu) / g.Sigma))
	}
	v := g.Xi * (x - g.Mu) / g.Sigma
	if v <= -1 {
		// Außerhalb des Trägers auf der endlichen Seite.
		if g.Xi > 0 {
			return 0
		}
		return 1
	}
	if math.IsNaN(v) {
		return math.NaN()
	}
	s := math.Log1p(v) / g.Xi
	return math.Exp(-math.Exp(-s))
}

// Quantile invertiert die CDF: mit y = −ln p gilt x = μ + σ·(y^(−ξ) − 1)/ξ
// für ξ ≠ 0 und x = μ − σ·ln(y) für ξ = 0. expm1 hält den Grenzfall ξ → 0 stabil.
func (g GEV) Quantile(p float64) float64 {
	if !g.valid() || !validProbability(p) {
		return math.NaN()
	}
	if p == 0 {
		if g.Xi > 0 {
			return g.Mu - g.Sigma/g.Xi
		}
		return math.Inf(-1)
	}
	if p == 1 {
		if g.Xi < 0 {
			return g.Mu - g.Sigma/g.Xi
		}
		return math.Inf(1)
	}
	y := -math.Log(p)
	if g.Xi == 0 {
		return g.Mu - g.Sigma*math.Log(y)
	}
	return g.Mu + g.Sigma*math.Expm1(-g.Xi*math.Log(y))/g.Xi
}

// ==========================================
// GEV FITTER
// ==========================================

// GEVFitter schätzt Mu, Sigma und Xi per Maximum-Likelihood. Die Likelihood ist
// für ξ < −1 unbeschränkt, daher wird ξ auf [gevXiMin, gevXiMax] begrenzt. Die
// Optimierung läuft per L-BFGS auf (μ, ln σ, ξ) mit analytischem Gradienten;
// aus mehreren Startwerten für ξ wird das beste Ergebnis gewählt.
type GEVFitter struct{}

func (f GEVFitter) Name() string {
	return "GEV"
}

const (
	// gevXiMin muss strikt größer als −1 bleiben: für ξ < −1 ist die Likelihood
	// unbeschränkt (Dichte divergiert am endlichen Trägerrand).
	gevXiMin = -0.99
	// gevXiMax begrenzt den Formparameter nach oben; größere Werte sind in der
	// Praxis für Blockmaxima-Daten nicht relevant.
	gevXiMax = 5
	// gevXiGradV ist die |v| = |ξz|-Schwelle, unterhalb derer ds/dξ per Potenzreihe
	// statt als Differenz (z/u − s)/ξ berechnet wird; dort wäre die Differenz
	// numerisch instabil (hebbares 0/0-Verhalten bei ξ → 0).
	gevXiGradV   = 0.05
	gevMaxIter   = 500
)

// gevLogLik berechnet die Log-Likelihood eines GEV-Modells. ok ist false, wenn
// ein Datenpunkt außerhalb des Trägers liegt oder die Rechnung nicht endlich bleibt.
func gevLogLik(data []float64, mu, sigma, xi float64) (ll float64, ok bool) {
	if !(sigma > 0) || !IsFinite(mu) || !IsFinite(sigma) || !IsFinite(xi) {
		return 0, false
	}
	for _, x := range data {
		term, finite := gevLogPDFTerm(x, mu, sigma, xi)
		if !finite {
			return 0, false
		}
		ll += term
	}
	return ll, IsFinite(ll)
}

// gevLogPDFTerm ist die Log-Dichte eines einzelnen Datenpunkts; finite ist false
// außerhalb des Trägers oder bei nicht endlichem Wert (t → ∞ am Trägerrand).
func gevLogPDFTerm(x, mu, sigma, xi float64) (term float64, finite bool) {
	z := (x - mu) / sigma
	if xi == 0 {
		t := math.Exp(-z)
		if math.IsInf(t, 1) {
			return math.Inf(-1), false
		}
		return -math.Log(sigma) - z - t, true
	}
	v := xi * z
	if v <= -1 || math.IsNaN(v) {
		return 0, false
	}
	s := math.Log1p(v) / xi
	t := math.Exp(-s)
	if math.IsNaN(t) || math.IsInf(t, 1) {
		return 0, false
	}
	term = -math.Log(sigma) - (xi+1)*s - t
	return term, IsFinite(term)
}

// gevDsDxiSeries ist die Potenzreihe von ds/dξ in v = ξz:
//
//	ds/dξ = z²·Σ_{j≥0} (−1)^{j+1}·(j+1)/(j+2)·v^j  =  −z²/2 + (2/3)z²v − (3/4)z²v² + …
//
// Sie liefert den hebbaren Grenzwert −z²/2 für ξ → 0 und konvergiert für |v| < 1;
// bei |v| ≤ gevXiGradV reichen acht Terme für ~1e-11 relative Genauigkeit.
func gevDsDxiSeries(v float64) float64 {
	return -0.5 + v*(2.0/3.0+v*(-0.75+v*(0.8+v*(-5.0/6.0+v*(6.0/7.0+v*(-7.0/8.0+v*(8.0/9.0)))))))
}

// gevDsDxi ist ds/dξ mit s = ln(1+ξz)/ξ. Geschlossene Form (z/u − s)/ξ; für
// |ξz| klein die Potenzreihe, weil die Differenz dortaus katastrophaler
// Kürzung leidet.
func gevDsDxi(z, v, s, xi, u float64) float64 {
	if math.Abs(v) <= gevXiGradV {
		return z * z * gevDsDxiSeries(v)
	}
	return (z/u - s) / xi
}

// gevLogLikGrad liefert Log-Likelihood und Summen-Gradienten bezüglich
// (μ, θ = ln σ, ξ). Die ξ-Komponente nutzt für |ξz| ≤ gevXiGradV eine
// Potenzreihe; sie ist überall analytisch, ein FD-Nachschlag ist nicht nötig.
func gevLogLikGrad(data []float64, mu, sigma, xi float64) (ll, gMu, gTheta, gXi float64, ok bool) {
	if !(sigma > 0) || !IsFinite(mu) || !IsFinite(sigma) || !IsFinite(xi) {
		return 0, 0, 0, 0, false
	}
	for _, x := range data {
		z := (x - mu) / sigma
		if xi == 0 {
			t := math.Exp(-z)
			if math.IsInf(t, 1) || math.IsNaN(t) {
				return 0, 0, 0, 0, false
			}
			ll += -math.Log(sigma) - z - t
			gMu += (1 - t) / sigma
			gTheta += -1 + z*(1-t)
			// ξ → 0: s → z und ds/dξ → −z²/2 (Reihe bei v = 0).
			gXi += -z - (t-1)*0.5*z*z
			continue
		}
		v := xi * z
		if v <= -1 || math.IsNaN(v) {
			return 0, 0, 0, 0, false
		}
		s := math.Log1p(v) / xi
		t := math.Exp(-s)
		if math.IsNaN(t) || math.IsInf(t, 1) {
			return 0, 0, 0, 0, false
		}
		ll += -math.Log(sigma) - (xi+1)*s - t
		u := 1 + v
		gMu += (xi + 1 - t) / (sigma * u)
		gTheta += -1 + z*(xi+1-t)/u
		gXi += -s + (t-xi-1)*gevDsDxi(z, v, s, xi, u)
	}
	ok = IsFinite(ll) && IsFinite(gMu) && IsFinite(gTheta) && IsFinite(gXi)
	return ll, gMu, gTheta, gXi, ok
}

// gevProfile cached das letzte Ergebnis von gevLogLikGrad: Func und Grad fragen im
// L-BFGS am selben Punkt (μ, σ, ξ) ab, sodass der O(n)-Pass pro Optimierungsschritt
// nur einmal läuft. Einträge sind bitgenau gekennzeichnet; abweichende Punkte sind
// schlicht Cache-Misses.
type gevProfile struct {
	data []float64

	cacheMu, cacheSigma, cacheXi             float64
	cacheLL, cacheGMu, cacheGTheta, cacheGXi float64
	cacheOK, cacheValid                      bool
}

func (p *gevProfile) eval(mu, sigma, xi float64) (ll, gMu, gTheta, gXi float64, ok bool) {
	if p.cacheValid && p.cacheMu == mu && p.cacheSigma == sigma && p.cacheXi == xi {
		return p.cacheLL, p.cacheGMu, p.cacheGTheta, p.cacheGXi, p.cacheOK
	}
	ll, gMu, gTheta, gXi, ok = gevLogLikGrad(p.data, mu, sigma, xi)
	p.cacheMu, p.cacheSigma, p.cacheXi = mu, sigma, xi
	p.cacheLL, p.cacheGMu, p.cacheGTheta, p.cacheGXi, p.cacheOK, p.cacheValid =
		ll, gMu, gTheta, gXi, ok, true
	return ll, gMu, gTheta, gXi, ok
}

func (f GEVFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	if len(cleaned) < 3 {
		return nil, fmt.Errorf("GEV benötigt mindestens drei Beobachtungen: %w", ErrFitFailed)
	}

	lo, hi := cleaned[0], cleaned[0]
	for _, x := range cleaned {
		lo = math.Min(lo, x)
		hi = math.Max(hi, x)
	}
	if hi == lo {
		return nil, fmt.Errorf("GEV-Parameter konnten nicht geschätzt werden: %w", ErrFitFailed)
	}

	// Startwerte: der Gumbel-Fit (ξ = 0) liefert μ und σ; als Fallback
	// dienen Momentenwerte der Gumbel-Näherung.
	mu0, sigma0 := 0.0, 0.0
	if g, err := (GumbelFitter{}).Fit(cleaned); err == nil {
		if gd, ok := g.(Gumbel); ok && gd.Beta > 0 {
			mu0, sigma0 = gd.Mu, gd.Beta
		}
	}
	if !(sigma0 > 0) || !IsFinite(mu0) || !IsFinite(sigma0) {
		mean := stat.Mean(cleaned, nil)
		sigma0 = math.Sqrt(stat.Variance(cleaned, nil)) * math.Sqrt(6) / math.Pi
		mu0 = mean - 0.5772156649015329*sigma0 // Euler-Mascheroni-Konstante
	}
	if !(sigma0 > 0) || !IsFinite(mu0) {
		return nil, fmt.Errorf("GEV-Startwerte ungültig: %w", ErrFitFailed)
	}

	norm := 1 / float64(len(cleaned))
	profile := &gevProfile{data: cleaned}
	problem := optimize.Problem{
		Func: func(v []float64) float64 {
			if v[2] < gevXiMin || v[2] > gevXiMax {
				return math.Inf(1)
			}
			ll, _, _, _, ok := profile.eval(v[0], math.Exp(v[1]), v[2])
			if !ok {
				return math.Inf(1)
			}
			return -ll * norm
		},
		Grad: func(grad, v []float64) {
			if v[2] < gevXiMin || v[2] > gevXiMax {
				grad[0], grad[1], grad[2] = math.Inf(1), math.Inf(1), math.Inf(1)
				return
			}
			_, gMu, gTheta, gXi, ok := profile.eval(v[0], math.Exp(v[1]), v[2])
			if !ok {
				grad[0], grad[1], grad[2] = math.Inf(1), math.Inf(1), math.Inf(1)
				return
			}
			grad[0] = -gMu * norm
			grad[1] = -gTheta * norm
			grad[2] = -gXi * norm
		},
	}

	settings := &optimize.Settings{
		GradientThreshold: 1e-9,
		MajorIterations:   gevMaxIter,
		FuncEvaluations:   5000,
		GradEvaluations:   5000,
	}

	var best GEV
	bestLL := math.Inf(-1)
	fitted := false
	for _, xi0 := range []float64{0, 0.2, -0.2} {
		initial := []float64{mu0, math.Log(sigma0), xi0}
		result, err := optimize.Minimize(problem, initial, settings, &optimize.LBFGS{})
		if err != nil && !errors.Is(err, optimize.ErrNoProgress) && !errors.Is(err, optimize.ErrLinesearcherFailure) {
			continue
		}
		v := result.Location.X
		dist := GEV{Mu: v[0], Sigma: math.Exp(v[1]), Xi: v[2]}
		if !dist.valid() || dist.Xi < gevXiMin || dist.Xi > gevXiMax {
			continue
		}
		// Sicherheitscheck: die Likelihood muss an allen Datenpunkten endlich sein,
		// sonst schlägt EvaluateFit später fehl.
		ll, ok := gevLogLik(cleaned, dist.Mu, dist.Sigma, dist.Xi)
		if !ok || ll <= bestLL {
			continue
		}
		best, bestLL, fitted = dist, ll, true
	}
	if !fitted {
		return nil, fmt.Errorf("GEV-MLE-Optimierung fehlgeschlagen: %w", ErrFitFailed)
	}
	return best, nil
}
