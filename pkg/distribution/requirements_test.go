package distribution

import (
	"errors"
	"math"
	"math/rand/v2"
	"testing"
)

// sampleSize ist groß genug, damit die MLE-Schätzungen stabil genug für
// Toleranzprüfungen sind, und klein genug für schnelle Tests.
const sampleSize = 5000

func newTestRNG() *rand.Rand {
	return rand.New(rand.NewPCG(42, 1337))
}

// Probengenerierung für verschiedene Verteilungen, um die Fit-Funktionalität zu testen.
func sampleExponential(rng *rand.Rand, lambda float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = -math.Log(1-rng.Float64()) / lambda
	}
	return out
}

func sampleNormal(rng *rand.Rand, mu, sigma float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = mu + sigma*rng.NormFloat64()
	}
	return out
}

func sampleLogNormal(rng *rand.Rand, mu, sigma float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = math.Exp(mu + sigma*rng.NormFloat64())
	}
	return out
}

// sampleGammaInt erzeugt Gamma(α, θ)-Daten für ganzzahliges α als Summe von
// α unabhängigen Exponentialvariablen mit Mittelwert θ.
func sampleGammaInt(rng *rand.Rand, alpha int, theta float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		sum := 0.0
		for j := 0; j < alpha; j++ {
			sum += -math.Log(1-rng.Float64()) * theta
		}
		out[i] = sum
	}
	return out
}

func sampleWeibull(rng *rand.Rand, shape, scale float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = scale * math.Pow(-math.Log(1-rng.Float64()), 1/shape)
	}
	return out
}

func sampleGumbel(rng *rand.Rand, mu, beta float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = mu - beta*math.Log(-math.Log(rng.Float64()))
	}
	return out
}

// Laplace: mu ± b·Exp(1) mit zufälligem Vorzeichen.
func sampleLaplace(rng *rand.Rand, mu, b float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		e := -math.Log(1 - rng.Float64())
		if rng.Float64() < 0.5 {
			e = -e
		}
		out[i] = mu + b*e
	}
	return out
}

// Rayleigh per Inversionsmethode: σ·sqrt(-2·ln(1-U)).
func sampleRayleigh(rng *rand.Rand, sigma float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = sigma * math.Sqrt(-2*math.Log(1-rng.Float64()))
	}
	return out
}

func sampleBeta(rng *rand.Rand, alpha, betaParam float64, n int) []float64 {
	out := make([]float64, n)

	for i := range out {
		x := sampleGammaUnit(rng, alpha)
		y := sampleGammaUnit(rng, betaParam)
		out[i] = x / (x + y)
	}

	return out
}

// sampleGammaUnit zieht eine Gamma(α, 1)-Zufallszahl für beliebiges α > 0
// (Marsaglia-Tsang; für α < 1 mit der Boost-Transformation).
func sampleGammaUnit(rng *rand.Rand, alpha float64) float64 {
	if alpha < 1 {
		return sampleGammaUnit(rng, alpha+1) * math.Pow(rng.Float64(), 1/alpha)
	}
	d := alpha - 1.0/3.0
	c := 1 / math.Sqrt(9*d)
	for {
		x := rng.NormFloat64()
		v := 1 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := rng.Float64()
		if u < 1-0.0331*x*x*x*x || math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}

func assertRelative(t *testing.T, label string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol*math.Abs(want) {
		t.Errorf("%s = %.6g, want %.6g (relative tolerance %.2g)", label, got, want, tol)
	}
}

// Anforderung: Verteilungen werden anhand synthetischer Daten mit bekannten
// Parametern korrekt geschätzt.
func TestFittersRecoverKnownParameters(t *testing.T) {
	rng := newTestRNG()
	const tol = 0.05

	t.Run("Exponential", func(t *testing.T) {
		data := sampleExponential(rng, 0.5, sampleSize)
		dist, err := ExponentialFitter{}.Fit(data)
		if err != nil {
			t.Fatal(err)
		}
		assertRelative(t, "Lambda", dist.(Exponential).Lambda, 0.5, tol)
	})

	t.Run("Normal", func(t *testing.T) {
		data := sampleNormal(rng, 10, 2, sampleSize)
		dist, err := NormalFitter{}.Fit(data)
		if err != nil {
			t.Fatal(err)
		}
		n := dist.(Normal)
		assertRelative(t, "Mu", n.Mu, 10, tol)
		assertRelative(t, "Sigma", n.Sigma, 2, tol)
	})

	t.Run("LogNormal", func(t *testing.T) {
		data := sampleLogNormal(rng, 0.5, 0.4, sampleSize)
		dist, err := LogNormalFitter{}.Fit(data)
		if err != nil {
			t.Fatal(err)
		}
		l := dist.(LogNormal)
		assertRelative(t, "Mu", l.Mu, 0.5, tol)
		assertRelative(t, "Sigma", l.Sigma, 0.4, tol)
	})

	t.Run("Gamma", func(t *testing.T) {
		data := sampleGammaInt(rng, 2, 3, sampleSize)
		dist, err := GammaFitter{}.Fit(data)
		if err != nil {
			t.Fatal(err)
		}
		g := dist.(Gamma)
		assertRelative(t, "Alpha", g.Alpha, 2, 0.08)
		assertRelative(t, "Beta", g.Beta, 3, 0.08)
	})

	t.Run("Weibull", func(t *testing.T) {
		data := sampleWeibull(rng, 1.5, 3, sampleSize)
		dist, err := WeibullFitter{}.Fit(data)
		if err != nil {
			t.Fatal(err)
		}
		w := dist.(Weibull)
		assertRelative(t, "Shape", w.Shape, 1.5, 0.08)
		assertRelative(t, "Scale", w.Scale, 3, 0.08)
	})

	t.Run("Gumbel", func(t *testing.T) {
		data := sampleGumbel(rng, 10, 2, sampleSize)
		dist, err := GumbelFitter{}.Fit(data)
		if err != nil {
			t.Fatal(err)
		}
		g := dist.(Gumbel)
		assertRelative(t, "Mu", g.Mu, 10, tol)
		assertRelative(t, "Beta", g.Beta, 2, tol)
	})

	t.Run("Beta", func(t *testing.T) {
		data := sampleBeta(rng, 2, 3, sampleSize)
		dist, err := BetaFitter{}.Fit(data)
		if err != nil {
			t.Fatal(err)
		}
		b := dist.(Beta)
		assertRelative(t, "Alpha", b.Alpha, 2, 0.1)
		assertRelative(t, "Beta", b.Beta, 3, 0.1)
	})

	t.Run("Laplace", func(t *testing.T) {
		data := sampleLaplace(rng, 3, 2, sampleSize)
		dist, err := LaplaceFitter{}.Fit(data)
		if err != nil {
			t.Fatal(err)
		}
		l := dist.(Laplace)
		assertRelative(t, "Mu", l.Mu, 3, tol)
		assertRelative(t, "Scale", l.Scale, 2, tol)
	})

	t.Run("Rayleigh", func(t *testing.T) {
		data := sampleRayleigh(rng, 2, sampleSize)
		dist, err := RayleighFitter{}.Fit(data)
		if err != nil {
			t.Fatal(err)
		}
		assertRelative(t, "Scale", dist.(Rayleigh).Scale, 2, tol)
	})
}

// Anforderung: AIC = 2k − 2·LL und BIC = k·ln(n) − 2·LL mit k = Anzahl der
// Parameter und n = Stichprobengröße.
func TestEvaluateFitInformationCriteria(t *testing.T) {
	data := []float64{0.5, 1.2, 2.0, 0.7, 3.1, 1.8}
	dist := Exponential{Lambda: 0.6}

	metrics, err := EvaluateFit(dist, data)
	if err != nil {
		t.Fatal(err)
	}

	n := float64(len(data))
	sum := 0.0
	for _, x := range data {
		sum += x
	}
	wantLL := n*math.Log(0.6) - 0.6*sum
	if math.IsNaN(metrics.LogLikelihood) || math.Abs(metrics.LogLikelihood-wantLL) > 1e-12*(1+math.Abs(wantLL)) {
		t.Errorf("LogLikelihood = %.16g, want %.16g", metrics.LogLikelihood, wantLL)
	}
	if math.IsNaN(metrics.AIC) || math.Abs(metrics.AIC-(2*1-2*wantLL)) > 1e-12*(1+math.Abs(2*1-2*wantLL)) {
		t.Errorf("AIC = %.16g, want %.16g", metrics.AIC, 2*1-2*wantLL)
	}
	if math.IsNaN(metrics.BIC) || math.Abs(metrics.BIC-(math.Log(n)-2*wantLL)) > 1e-12*(1+math.Abs(math.Log(n)-2*wantLL)) {
		t.Errorf("BIC = %.16g, want %.16g", metrics.BIC, math.Log(n)-2*wantLL)
	}
	if metrics.SampleSize != len(data) {
		t.Errorf("SampleSize = %d, want %d", metrics.SampleSize, len(data))
	}
}

// Anforderung: Das Informationskriterium berücksichtigt die Anzahl der
// Parameter. Gleiche LL mit mehr Parametern muss zu schlechterem AIC führen.
func TestEvaluateFitPenalizesParameterCount(t *testing.T) {
	data := sampleNormal(newTestRNG(), 10, 2, 200)
	dist := Normal{Mu: 10, Sigma: 2}

	metrics, err := EvaluateFit(dist, data)
	if err != nil {
		t.Fatal(err)
	}
	wantAIC := 2*2 - 2*metrics.LogLikelihood
	wantBIC := 2*math.Log(float64(len(data))) - 2*metrics.LogLikelihood
	if math.IsNaN(metrics.AIC) || math.Abs(metrics.AIC-wantAIC) > 1e-12*(1+math.Abs(wantAIC)) {
		t.Errorf("AIC = %.16g, want %.16g", metrics.AIC, wantAIC)
	}
	if math.IsNaN(metrics.BIC) || math.Abs(metrics.BIC-wantBIC) > 1e-12*(1+math.Abs(wantBIC)) {
		t.Errorf("BIC = %.16g, want %.16g", metrics.BIC, wantBIC)
	}
}

// Anforderung: Die Dichte ist die Ableitung der CDF, daher muss das Integral
// der PDF über ein Intervall der Differenz der CDF-Werte entsprechen.
func TestPDFIntegratesToCDFDifference(t *testing.T) {
	tests := []struct {
		name string
		dist ContinuousDistribution
		a, b float64
	}{
		{"Normal", Normal{Mu: 0, Sigma: 1}, -2, 1.5},
		{"Exponential", Exponential{Lambda: 0.7}, 0.1, 4},
		{"Laplace", Laplace{Mu: 0.5, Scale: 1.2}, -4, 6},
		{"Rayleigh", Rayleigh{Scale: 1.5}, 0.05, 5},
		{"LogNormal", LogNormal{Mu: 0.2, Sigma: 0.5}, 0.2, 3},
		{"Gamma", Gamma{Alpha: 2.5, Beta: 1.5}, 0.1, 9},
		{"Weibull", Weibull{Shape: 1.8, Scale: 2}, 0.05, 5},
		{"Gumbel", Gumbel{Mu: 1, Beta: 2}, -4, 8},
	}

	const steps = 20000
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := (tt.b - tt.a) / steps
			integral := 0.0
			for i := 0; i <= steps; i++ {
				w := 2.0
				if i == 0 || i == steps {
					w = 1
				} else if i%2 == 1 {
					w = 4
				}
				integral += w * tt.dist.PDF(tt.a+float64(i)*h)
			}
			integral *= h / 3

			want := tt.dist.CDF(tt.b) - tt.dist.CDF(tt.a)
			if math.Abs(integral-want) > 1e-6 {
				t.Errorf("∫PDF = %.10f, CDF-Differenz = %.10f", integral, want)
			}
		})
	}
}

// Anforderung: LogPDF muss dem Logarithmus der PDF entsprechen, ohne
// Überlauf oder Unterlauf im Vergleich zur direkten Berechnung.
func TestLogPDFMatchesLogOfPDF(t *testing.T) {
	dists := []ContinuousDistribution{
		Normal{Mu: 1, Sigma: 2},
		Exponential{Lambda: 1.3},
		Laplace{Mu: 0.5, Scale: 1.2},
		Rayleigh{Scale: 1.5},
		LogNormal{Mu: 0.1, Sigma: 0.6},
		Gamma{Alpha: 3, Beta: 2},
		Weibull{Shape: 2, Scale: 4},
		Gumbel{Mu: 0, Beta: 1.5},
	}
	points := []float64{0.1, 0.5, 1, 2, 3.5, 7}

	for _, dist := range dists {
		for _, x := range points {
			pdf := dist.PDF(x)
			if pdf <= 0 {
				continue
			}
			got := dist.LogPDF(x)
			if math.Abs(got-math.Log(pdf)) > 1e-9*(1+math.Abs(got)) {
				t.Errorf("%T LogPDF(%g) = %.12g, log(PDF) = %.12g", dist, x, got, math.Log(pdf))
			}
		}
	}
}

// Anforderung: Datenpunkte außerhalb des Trägers führen zu einer fehlgeschlagenen
// Bewertung und nicht zu einer stillschweigend falschen Metrik.
func TestEvaluateFitRejectsOutOfSupportData(t *testing.T) {
	dist := Beta{Alpha: 2, Beta: 3, Min: 0, Max: 1}
	_, err := EvaluateFit(dist, []float64{0.2, 0.4, 2.5})
	if !errors.Is(err, ErrFitFailed) {
		t.Fatalf("EvaluateFit() error = %v, want ErrFitFailed", err)
	}
}

// Anforderung: Ein Datensatz ohne gültige Werte liefert ErrEmptyData.
func TestEvaluateFitRejectsAllInvalidData(t *testing.T) {
	invalid := []float64{math.NaN(), math.Inf(1), -1, -2}
	_, err := EvaluateFit(Exponential{Lambda: 1}, invalid)
	if !errors.Is(err, ErrEmptyData) {
		t.Fatalf("EvaluateFit() error = %v, want ErrEmptyData", err)
	}
}

// Anforderung: Bei Nullwerten, die für Verteilungen mit Träger [0, ∞) gültig
// sind, dürfen diese nicht still verworfen werden; NaN/Inf dagegen schon.
func TestCleanDataWithZeroKeepsZeroAndDropsNonFinite(t *testing.T) {
	data := []float64{0, 1.5, math.NaN(), math.Inf(-1), 0, 2, math.Inf(1)}
	got := CleanDataWithZero(data)
	want := []float64{0, 1.5, 0, 2}

	if len(got) != len(want) {
		t.Fatalf("CleanDataWithZero() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("CleanDataWithZero()[%d] = %g, want %g", i, got[i], want[i])
		}
	}
}

// Anforderung: Konstante Daten dürfen für Weibull keinen erfolgreichen Fit liefern.
func TestWeibullFitRejectsConstantData(t *testing.T) {
	for _, data := range [][]float64{{0, 0, 0, 0}, {5, 5, 5, 5}} {
		dist, err := WeibullFitter{}.Fit(data)
		if err == nil {
			t.Errorf("Fit(%v) = %+v, want an error", data, dist)
		}
	}
}

// Anforderung: k zählt nur frei geschätzte Parameter. Feste Min/Max bei Beta
// dürfen nicht in AIC/BIC eingehen.
func TestBetaInformationCriteriaCountOnlyFreeParameters(t *testing.T) {
	data := []float64{0.2, 0.4, 0.5, 0.7, 0.9}
	dist := Beta{Alpha: 2, Beta: 3, Min: 0, Max: 1}

	metrics, err := EvaluateFit(dist, data)
	if err != nil {
		t.Fatal(err)
	}
	n := float64(len(data))
	wantAIC := 2*2 - 2*metrics.LogLikelihood
	wantBIC := 2*math.Log(n) - 2*metrics.LogLikelihood
	if math.IsNaN(metrics.AIC) || math.Abs(metrics.AIC-wantAIC) > 1e-12*(1+math.Abs(wantAIC)) {
		t.Errorf("AIC = %.16g, want %.16g", metrics.AIC, wantAIC)
	}
	if math.IsNaN(metrics.BIC) || math.Abs(metrics.BIC-wantBIC) > 1e-12*(1+math.Abs(wantBIC)) {
		t.Errorf("BIC = %.16g, want %.16g", metrics.BIC, wantBIC)
	}
}

// Anforderung: Ungültige Werte dürfen nicht stillschweigend verschwinden und
// müssen in den Metriken sichtbar sein.
func TestEvaluateFitReportsDiscardedValues(t *testing.T) {
	data := []float64{1, 2, math.NaN(), -3, math.Inf(1), 4}
	metrics, err := EvaluateFit(Exponential{Lambda: 0.5}, data)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.SampleSize != 3 || metrics.Discarded != 3 {
		t.Errorf("SampleSize = %d, Discarded = %d, want 3 and 3", metrics.SampleSize, metrics.Discarded)
	}
}

// Anforderung: Normal- und Gumbel-Verteilungen haben Träger ℝ, negative
// Werte gehen in die Schätzung ein.
func TestNormalAndGumbelUseNegativeValues(t *testing.T) {
	data := []float64{-5, -4, -3, 1, 2, 3, 4}
	dist, err := NormalFitter{}.Fit(data)
	if err != nil {
		t.Fatal(err)
	}
	wantMu := -2.0 / 7.0
	if math.IsNaN(dist.(Normal).Mu) || math.Abs(dist.(Normal).Mu-wantMu) > 1e-12*(1+math.Abs(wantMu)) {
		t.Errorf("Mu = %.16g, want %.16g", dist.(Normal).Mu, wantMu)
	}

	metrics, err := EvaluateFit(dist, data)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.SampleSize != len(data) || metrics.Discarded != 0 {
		t.Errorf("Normal: SampleSize = %d, Discarded = %d, want %d and 0", metrics.SampleSize, metrics.Discarded, len(data))
	}
}

// endliche und positive Parameter.
func TestWeibullFitReturnsPositiveFiniteParameters(t *testing.T) {
	data := sampleWeibull(newTestRNG(), 0.8, 5, 1000)
	dist, err := WeibullFitter{}.Fit(data)
	if err != nil {
		t.Fatal(err)
	}
	w := dist.(Weibull)
	if !(w.Shape > 0) || !(w.Scale > 0) || math.IsInf(w.Shape, 0) || math.IsInf(w.Scale, 0) {
		t.Errorf("Weibull fit = %+v, want positive finite Shape and Scale", w)
	}
}
