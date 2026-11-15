package distribution

import (
	"math"
	"testing"
)

// Referenzwerte: scipy.stats.genextreme (c = −ξ) mit loc = μ, scale = σ.
func TestGEVMatchesScipyReference(t *testing.T) {
	tests := []struct {
		name string
		dist GEV
		x    float64
		pdf  float64
		cdf  float64
	}{
		{"positive xi", GEV{Mu: 1, Sigma: 2, Xi: 0.2}, 2.4, 0.1355133093693264, 0.5948960084892825},
		{"positive xi tail", GEV{Mu: 1, Sigma: 2, Xi: 0.2}, 6, 0.03847967604506793, 0.8766151199017037},
		{"negative xi", GEV{Mu: 0, Sigma: 1, Xi: -0.3}, 0.7, 0.3657517078789883, 0.6339521558500468},
		{"negative xi near endpoint", GEV{Mu: 0, Sigma: 1, Xi: -0.3}, 2.5, 0.03898688483016844, 0.9902051518779583},
		{"gumbel limit", GEV{Mu: 3, Sigma: 0.5, Xi: 0}, 3, 0.7357588823428847, 0.3678794411714423},
		{"gumbel limit upper", GEV{Mu: 3, Sigma: 0.5, Xi: 0}, 4.25, 0.151232359834853, 0.9211936551755158},
		{"large xi", GEV{Mu: -2, Sigma: 1.5, Xi: 0.5}, -0.95, 0.1565349876934173, 0.5777021555192873},
		{"large xi tail", GEV{Mu: -2, Sigma: 1.5, Xi: 0.5}, 1.75, 0.04803686120812908, 0.8207548082982681},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertNear(t, "PDF", tt.dist.PDF(tt.x), tt.pdf)
			assertNear(t, "CDF", tt.dist.CDF(tt.x), tt.cdf)
			assertNear(t, "LogPDF", tt.dist.LogPDF(tt.x), math.Log(tt.pdf))
		})
	}
}

func TestGEVQuantileMatchesScipyReference(t *testing.T) {
	tests := []struct {
		name string
		dist GEV
		p    float64
		want float64
	}{
		{"positive xi q05", GEV{Mu: 1, Sigma: 2, Xi: 0.2}, 0.05, -0.9702985100034487},
		{"positive xi q50", GEV{Mu: 1, Sigma: 2, Xi: 0.2}, 0.5, 1.760560851390051},
		{"positive xi q95", GEV{Mu: 1, Sigma: 2, Xi: 0.2}, 0.95, 9.11289549357503},
		{"negative xi q05", GEV{Mu: 0, Sigma: 1, Xi: -0.3}, 0.05, -1.299318315231664},
		{"negative xi q50", GEV{Mu: 0, Sigma: 1, Xi: -0.3}, 0.5, 0.3470818148440703},
		{"negative xi q95", GEV{Mu: 0, Sigma: 1, Xi: -0.3}, 0.95, 1.9659290901263},
		{"gumbel q05", GEV{Mu: 3, Sigma: 0.5, Xi: 0}, 0.05, 2.451405649817525},
		{"gumbel q50", GEV{Mu: 3, Sigma: 0.5, Xi: 0}, 0.5, 3.183256460290832},
		{"gumbel q95", GEV{Mu: 3, Sigma: 0.5, Xi: 0}, 0.95, 4.485097624521082},
		{"large xi q95", GEV{Mu: -2, Sigma: 1.5, Xi: 0.5}, 0.95, 8.246189328105388},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRelative(t, "Quantile", tt.dist.Quantile(tt.p), tt.want, 1e-9)
		})
	}
}

// Anforderung: Der GEV-Fitter erfüllt die Score-Gleichungen der Likelihood:
// die Summe der Log-Likelihood-Gradienten nach (μ, ln σ, ξ) ist null.
func TestGEVFitterSatisfiesScoreEquations(t *testing.T) {
	rng := newTestRNG()
	for _, truth := range []GEV{
		{Mu: 5, Sigma: 2, Xi: 0.25},
		{Mu: -1, Sigma: 1.5, Xi: -0.15},
	} {
		data := sampleGEV(rng, truth.Mu, truth.Sigma, truth.Xi, 3000)
		dist, err := GEVFitter{}.Fit(data)
		if err != nil {
			t.Fatal(err)
		}
		g := dist.(GEV)
		_, gMu, gTheta, gXi, ok := gevLogLikGrad(data, g.Mu, g.Sigma, g.Xi)
		if !ok {
			t.Fatal("Gradient an der Schätzung nicht endlich")
		}
		n := float64(len(data))
		for name, score := range map[string]float64{"Mu": gMu, "logSigma": gTheta, "Xi": gXi} {
			if math.Abs(score)/n > 1e-4 {
				t.Errorf("%s-Score = %g, want ~0 (fit: %+v)", name, score, g)
			}
		}
	}
}

// Anforderung: Der analytische Gradient stimmt mit zentraler Differenz überein,
// auch nahe ξ = 0, wo die ξ-Komponente per Differenz geschätzt wird.
func TestGEVLogLikGradMatchesFiniteDifference(t *testing.T) {
	data := []float64{-2.5, -1.0, 0.3, 1.2, 2.8, 4.1, 5.5}
	for _, xi := range []float64{-0.3, -1e-5, 0, 1e-5, 0.2} {
		mu, sigma := 0.5, 1.7
		_, gMu, gTheta, gXi, ok := gevLogLikGrad(data, mu, sigma, xi)
		if !ok {
			t.Fatalf("xi=%g: Gradient nicht endlich", xi)
		}
		const h = 1e-6
		fd := func(f func(float64) float64, x, step float64) float64 {
			return (f(x+step) - f(x-step)) / (2 * step)
		}
		ll := func(m, s, x float64) float64 {
			v, _ := gevLogLik(data, m, s, x)
			return v
		}
		wantMu := fd(func(m float64) float64 { return ll(m, sigma, xi) }, mu, h)
		wantTheta := fd(func(th float64) float64 { return ll(mu, math.Exp(th), xi) }, math.Log(sigma), h)
		wantXi := fd(func(x float64) float64 { return ll(mu, sigma, x) }, xi, h)
		for name, pair := range map[string][2]float64{
			"Mu":       {gMu, wantMu},
			"logSigma": {gTheta, wantTheta},
			"Xi":       {gXi, wantXi},
		} {
			got, want := pair[0], pair[1]
			if math.Abs(got-want) > 1e-5*(1+math.Abs(want)) {
				t.Errorf("xi=%g %s-Gradient = %.10g, FD = %.10g", xi, name, got, want)
			}
		}
	}
}

// Anforderung: Der Träger ist ξ-abhängig; außerhalb davon sind Dichte und
// Log-Dichte null, die CDF konstant.
func TestGEVSupportBoundaries(t *testing.T) {
	// ξ > 0: untere Schranke μ − σ/ξ = 1 − 2/0.2 = −9
	pos := GEV{Mu: 1, Sigma: 2, Xi: 0.2}
	if got := pos.PDF(-10); got != 0 {
		t.Errorf("PDF unterhalb des Trägers = %g, want 0", got)
	}
	if got := pos.LogPDF(-10); !math.IsInf(got, -1) {
		t.Errorf("LogPDF unterhalb des Trägers = %g, want -Inf", got)
	}
	if got := pos.CDF(-10); got != 0 {
		t.Errorf("CDF unterhalb des Trägers = %g, want 0", got)
	}
	// ξ < 0: obere Schranke μ − σ/ξ = 0 − 1/(−0.3) = 10/3
	neg := GEV{Mu: 0, Sigma: 1, Xi: -0.3}
	if got := neg.PDF(4); got != 0 {
		t.Errorf("PDF oberhalb des Trägers = %g, want 0", got)
	}
	if got := neg.CDF(4); got != 1 {
		t.Errorf("CDF oberhalb des Trägers = %g, want 1", got)
	}
	// ξ = 0: Träger ℝ
	gumbel := GEV{Mu: 0, Sigma: 1, Xi: 0}
	if got := gumbel.CDF(-5); got <= 0 || got >= 1 {
		t.Errorf("CDF(-5) = %g, want in (0, 1)", got)
	}
}

// Anforderung: Der Fit einer GEV-Stichprobe liefertParameter in der Nähe der
// Wahrheit; ξ wird dabei mit absoluter Toleranz geprüft, weil sein
// Standardfehler nur schwach von μ und σ abhängt.
func TestGEVFitRecoversParameters(t *testing.T) {
	rng := newTestRNG()
	for _, truth := range []GEV{
		{Mu: 5, Sigma: 2, Xi: 0.25},
		{Mu: -1, Sigma: 1.5, Xi: -0.15},
		{Mu: 10, Sigma: 3, Xi: 0},
	} {
		data := sampleGEV(rng, truth.Mu, truth.Sigma, truth.Xi, 5000)
		dist, err := GEVFitter{}.Fit(data)
		if err != nil {
			t.Fatal(err)
		}
		got := dist.(GEV)
		assertRelative(t, "Mu", got.Mu, truth.Mu, 0.05)
		assertRelative(t, "Sigma", got.Sigma, truth.Sigma, 0.05)
		if math.Abs(got.Xi-truth.Xi) > 0.08 {
			t.Errorf("Xi = %.4f, want ~%.4f (fit: %+v)", got.Xi, truth.Xi, got)
		}
	}
}
