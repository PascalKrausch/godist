package distribution

import (
	"errors"
	"math"
	"testing"

	"gonum.org/v1/gonum/mathext"
)

func TestFittersFitValidData(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	withInvalidValues := []float64{1, 2, math.NaN(), 3, math.Inf(1), 4, -1, 5, 6, 7, 8, 9, 10}
	withNonFiniteOnly := []float64{1, 2, math.NaN(), 3, math.Inf(1), 4, 5, 6, 7, 8, 9, 10}
	tests := []struct {
		name       string
		fitter     Fitter
		paramCount int
		// rejectsNegative: Trägerbedingung [0, ∞) bzw. (0, ∞); negative Werte führen zu ErrNegativeValues.
		rejectsNegative bool
	}{
		{"Beta", BetaFitter{}, 4, false},
		{"Exponential", ExponentialFitter{}, 1, true},
		{"Normal", NormalFitter{}, 2, false},
		{"Gumbel", GumbelFitter{}, 2, false},
		{"Gamma", GammaFitter{}, 2, true},
		{"LogNormal", LogNormalFitter{}, 2, true},
		{"Weibull", WeibullFitter{}, 2, true},
		{"Laplace", LaplaceFitter{}, 2, false},
		{"Logistic", LogisticFitter{}, 2, false},
		{"Rayleigh", RayleighFitter{}, 1, true},
		{"Pareto", ParetoFitter{}, 2, true},
		{"ChiSquare", ChiSquareFitter{}, 1, true},
		{"Cauchy", CauchyFitter{}, 2, false},
		{"StudentT", StudentTFitter{}, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist, err := tt.fitter.Fit(data)
			if err != nil {
				t.Fatalf("%s.Fit() error = %v", tt.name, err)
			}
			if len(dist.Params()) != tt.paramCount {
				t.Fatalf("%s.Fit() returned %d parameters, want %d", tt.name, len(dist.Params()), tt.paramCount)
			}
			for i, param := range dist.Params() {
				if math.IsNaN(param) || math.IsInf(param, 0) {
					t.Errorf("parameter %d = %g, want a finite value", i, param)
				}
			}

			metrics, err := EvaluateFit(dist, data)
			if err != nil {
				t.Fatalf("EvaluateFit() error = %v", err)
			}
			if metrics.SampleSize != len(data) || math.IsNaN(metrics.LogLikelihood) || math.IsInf(metrics.LogLikelihood, 0) {
				t.Errorf("EvaluateFit() returned invalid metrics: %+v", metrics)
			}

			filteredDist, err := tt.fitter.Fit(withNonFiniteOnly)
			if err != nil {
				t.Fatalf("%s.Fit() with NaN/Inf removed returned error: %v", tt.name, err)
			}
			for i, want := range dist.Params() {
				got := filteredDist.Params()[i]
				if math.IsNaN(got) || math.Abs(got-want) > 1e-12*(1+math.Abs(want)) {
					t.Errorf("parameter after cleaning[%d] = %.16g, want %.16g", i, got, want)
				}
			}

			if tt.rejectsNegative {
				if _, err := tt.fitter.Fit(withInvalidValues); !errors.Is(err, ErrNegativeValues) {
					t.Errorf("%s.Fit() with negative values error = %v, want ErrNegativeValues", tt.name, err)
				}
			}
		})
	}
}

func TestFittersRejectInsufficientOrInvalidData(t *testing.T) {
	tests := []struct {
		name   string
		fitter Fitter
		data   []float64
	}{
		{"Beta needs at least three observations", BetaFitter{}, []float64{1, 2}},
		{"Exponential needs a positive mean", ExponentialFitter{}, []float64{0}},
		{"Normal needs variation", NormalFitter{}, []float64{1}},
		{"Gumbel needs at least two observations", GumbelFitter{}, []float64{1}},
		{"Gamma needs varying observations", GammaFitter{}, []float64{1}},
		{"LogNormal needs varying observations", LogNormalFitter{}, []float64{1}},
		{"Weibull needs at least two observations", WeibullFitter{}, []float64{1}},
		{"Laplace needs at least two observations", LaplaceFitter{}, []float64{1}},
		{"Laplace needs a positive scale", LaplaceFitter{}, []float64{2, 2, 2}},
		{"Logistic needs at least two observations", LogisticFitter{}, []float64{1}},
		{"Logistic needs varying observations", LogisticFitter{}, []float64{2, 2, 2}},
		{"Rayleigh needs positive observations", RayleighFitter{}, []float64{0, 0}},
		{"Pareto needs positive observations", ParetoFitter{}, []float64{0, 1, 2}},
		{"Pareto needs varying observations", ParetoFitter{}, []float64{2, 2, 2}},
		{"ChiSquare needs positive observations", ChiSquareFitter{}, []float64{0, 1, 2}},
		{"Cauchy needs varying observations", CauchyFitter{}, []float64{2, 2, 2}},
		{"StudentT needs at least three observations", StudentTFitter{}, []float64{1, 2}},
		{"StudentT needs varying observations", StudentTFitter{}, []float64{2, 2, 2}},
		{"Beta rejects values on specified boundaries", BetaFitter{Min: 0, Max: 1}, []float64{0, 0.3, 0.8}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist, err := tt.fitter.Fit(tt.data)
			if err == nil {
				t.Fatalf("Fit() = %v, want an error", dist)
			}
			if dist != nil {
				t.Errorf("Fit() returned a distribution with an error: %v", dist)
			}
		})
	}
}

func TestExponentialFitterRate(t *testing.T) {
	dist, err := (ExponentialFitter{}).Fit([]float64{1, 2, 3, 4})
	if err != nil {
		t.Fatalf("Fit() error = %v", err)
	}
	got, ok := dist.(Exponential)
	if !ok {
		t.Fatalf("Fit() returned %T, want Exponential", dist)
	}
	want := 0.4
	if math.IsNaN(got.Lambda) || math.Abs(got.Lambda-want) > 1e-12*(1+math.Abs(want)) {
		t.Errorf("rate = %.16g, want %.16g", got.Lambda, want)
	}
}

func TestLaplaceFitterClosedForm(t *testing.T) {
	tests := []struct {
		name      string
		data      []float64
		wantMu    float64
		wantScale float64
	}{
		{"even count uses mean of middle values", []float64{1, 2, 3, 4}, 2.5, 1},
		{"odd count uses middle value", []float64{1, 2, 10}, 2, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist, err := (LaplaceFitter{}).Fit(tt.data)
			if err != nil {
				t.Fatalf("Fit() error = %v", err)
			}
			got, ok := dist.(Laplace)
			if !ok {
				t.Fatalf("Fit() returned %T, want Laplace", dist)
			}
			assertNear(t, "Mu", got.Mu, tt.wantMu)
			assertNear(t, "Scale", got.Scale, tt.wantScale)
		})
	}
}

func TestLogisticFitterMLE(t *testing.T) {
	// Referenzwerte: scipy.stats.logistic.fit (MLE).
	tests := []struct {
		name      string
		data      []float64
		wantMu    float64
		wantScale float64
	}{
		{"symmetric dataset", []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 5.5, 1.7483251828050477},
		{"small symmetric dataset", []float64{1, 2, 3, 4}, 2.5, 0.6858941852076591},
		{"skewed dataset with outlier", []float64{1, 2, 10}, 3.759446011690468, 2.4496358294314975},
		{"wide dataset with large outlier", []float64{-5, 0.5, 3, 100}, 15.138879400332462, 23.88243928788377},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist, err := (LogisticFitter{}).Fit(tt.data)
			if err != nil {
				t.Fatalf("Fit() error = %v", err)
			}
			got, ok := dist.(Logistic)
			if !ok {
				t.Fatalf("Fit() returned %T, want Logistic", dist)
			}
			assertRelative(t, "Mu", got.Mu, tt.wantMu, 1e-6)
			assertRelative(t, "Scale", got.Scale, tt.wantScale, 1e-6)
		})
	}
}

func TestRayleighFitterClosedForm(t *testing.T) {
	// σ = sqrt(sum(x²) / (2n)) = sqrt(16 / 8) = sqrt(2)
	dist, err := (RayleighFitter{}).Fit([]float64{2, 2, 2, 2})
	if err != nil {
		t.Fatalf("Fit() error = %v", err)
	}
	got, ok := dist.(Rayleigh)
	if !ok {
		t.Fatalf("Fit() returned %T, want Rayleigh", dist)
	}
	assertNear(t, "Scale", got.Scale, math.Sqrt2)
}

func TestParetoFitterClosedForm(t *testing.T) {
	// xm = min(x) = 1, α = n / Σ ln(x/xm) = 3 / (3·ln 2) = 1/ln 2
	dist, err := (ParetoFitter{}).Fit([]float64{1, 2, 4})
	if err != nil {
		t.Fatalf("Fit() error = %v", err)
	}
	got, ok := dist.(Pareto)
	if !ok {
		t.Fatalf("Fit() returned %T, want Pareto", dist)
	}
	assertNear(t, "Shape", got.Shape, 1/math.Ln2)
	assertNear(t, "Scale", got.Scale, 1)
}

// weibullShapeBisection ist die Referenz-Bisektion für den Shape-MLE (vor der Newton-Umstellung).
func weibullShapeBisection(logs []float64) float64 {
	meanLog := 0.0
	for _, l := range logs {
		meanLog += l
	}
	meanLog /= float64(len(logs))
	maxLog := logs[0]
	for _, l := range logs[1:] {
		maxLog = math.Max(maxLog, l)
	}
	g := func(k float64) float64 {
		s0, s1 := 0.0, 0.0
		for _, l := range logs {
			w := math.Exp(k * (l - maxLog))
			s0 += w
			s1 += w * l
		}
		return s1/s0 - meanLog - 1/k
	}
	lo, hi := 1e-3, 1e3
	for hi-lo > 1e-14*hi {
		mid := 0.5 * (lo + hi)
		if g(mid) < 0 {
			lo = mid
		} else {
			hi = mid
		}
	}
	return 0.5 * (lo + hi)
}

func TestWeibullNewtonMatchesBisection(t *testing.T) {
	for _, shape := range []float64{0.3, 0.8, 1.0, 1.7, 3.5, 12} {
		for seed := uint64(1); seed <= 5; seed++ {
			data := make([]float64, 2000)
			state := seed * 0x9E3779B97F4A7C15
			for i := range data {
				state += 0x9E3779B97F4A7C15
				z := state
				z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
				z = (z ^ (z >> 27)) * 0x94D049BB133111EB
				z ^= z >> 31
				u := (float64(z>>11) + 0.5) / (1 << 53)
				data[i] = math.Pow(-math.Log(u), 1/shape)
			}
			logs := make([]float64, len(data))
			for i, x := range data {
				logs[i] = math.Log(x)
			}
			got, ok := solveWeibullShape(logs)
			if !ok {
				t.Fatalf("shape=%g seed=%d: solveWeibullShape failed", shape, seed)
			}
			want := weibullShapeBisection(logs)
			if math.Abs(got-want) > 1e-9*want {
				t.Errorf("shape=%g seed=%d: newton=%.15g bisection=%.15g", shape, seed, got, want)
			}
		}
	}
}

func TestWeibullSolverRejectsDegenerateData(t *testing.T) {
	logs := []float64{1, 1, 1, 1}
	if _, ok := solveWeibullShape(logs); ok {
		t.Fatal("solveWeibullShape accepted constant data, want failure")
	}
}

// Anforderung: Der Chi-Quadrat-Fitter erfüllt die MLE-Gleichung ψ(k/2) = mean(ln x) − ln 2
// und liefert für bekannte Freiheitsgrade eine konsistente Schätzung.
func TestChiSquareFitterSatisfiesMLEEquation(t *testing.T) {
	rng := newTestRNG()
	data := make([]float64, 5000)
	for i := range data {
		sum := 0.0
		for j := 0; j < 6; j++ {
			z := rng.NormFloat64()
			sum += z * z
		}
		data[i] = sum
	}
	dist, err := ChiSquareFitter{}.Fit(data)
	if err != nil {
		t.Fatal(err)
	}
	got := dist.(ChiSquare).DF
	meanLog := 0.0
	for _, x := range data {
		meanLog += math.Log(x)
	}
	meanLog /= float64(len(data))
	if residual := mathext.Digamma(got/2) - (meanLog - math.Ln2); math.Abs(residual) > 1e-9 {
		t.Errorf("MLE-Residuum = %g, want ~0 (DF = %g)", residual, got)
	}
	if math.Abs(got-6) > 0.3 {
		t.Errorf("DF = %g, want ~6", got)
	}
}

// Anforderung: Der Cauchy-Fitter erfüllt die Score-Gleichungen der Likelihood:
// Σ 2d/(s²+d²) = 0 für μ und n = Σ 2d²/(s²+d²) für s, mit d = x − μ.
func TestCauchyFitterSatisfiesScoreEquations(t *testing.T) {
	rng := newTestRNG()
	data := make([]float64, 5000)
	for i := range data {
		data[i] = 3 + 2*math.Tan(math.Pi*(rng.Float64()-0.5))
	}
	dist, err := CauchyFitter{}.Fit(data)
	if err != nil {
		t.Fatal(err)
	}
	c := dist.(Cauchy)
	n := float64(len(data))
	s2 := c.Scale * c.Scale
	muScore, scaleScore := 0.0, n
	for _, x := range data {
		d := x - c.Mu
		muScore += 2 * d / (s2 + d*d)
		scaleScore -= 2 * d * d / (s2 + d*d)
	}
	if math.Abs(muScore)/n > 1e-8 {
		t.Errorf("μ-Score = %g, want ~0", muScore)
	}
	if math.Abs(scaleScore)/n > 1e-8 {
		t.Errorf("Scale-Score = %g, want ~0", scaleScore)
	}
}

// Anforderung: Der StudentT-Fitter erfüllt für das geschätzte ν die Score-Gleichungen der Likelihood:
// Σ (ν+1)d/(νs²+d²) = 0 für μ und Σ (ν+1)d²/(νs²+d²) = n für s, mit d = x − μ.
func TestStudentTFitterSatisfiesScoreEquations(t *testing.T) {
	rng := newTestRNG()
	const nu = 6.0
	data := make([]float64, 5000)
	for i := range data {
		chi := 0.0
		for k := 0; k < int(nu); k++ {
			z := rng.NormFloat64()
			chi += z * z
		}
		data[i] = 3 + 2*rng.NormFloat64()/math.Sqrt(chi/nu)
	}
	dist, err := StudentTFitter{}.Fit(data)
	if err != nil {
		t.Fatal(err)
	}
	st := dist.(StudentT)
	n := float64(len(data))
	nuS2 := st.Nu * st.Scale * st.Scale
	muScore, scaleScore := 0.0, -n
	for _, x := range data {
		d := x - st.Mu
		muScore += (st.Nu + 1) * d / (nuS2 + d*d)
		scaleScore += (st.Nu + 1) * d * d / (nuS2 + d*d)
	}
	if math.Abs(muScore)/n > 1e-6 {
		t.Errorf("μ-Score = %g, want ~0", muScore)
	}
	if math.Abs(scaleScore)/n > 1e-6 {
		t.Errorf("Scale-Score = %g, want ~0", scaleScore)
	}
}

func TestPositiveSupportFittersRejectZeros(t *testing.T) {
	withZero := []float64{0, 0.5, 1.2, 2.3, 3.1, 4.8, 0.9, 1.7}
	fitters := []Fitter{WeibullFitter{}, GammaFitter{}, RayleighFitter{}, LogNormalFitter{}, ParetoFitter{}, ChiSquareFitter{}}
	for _, f := range fitters {
		if _, err := f.Fit(withZero); err == nil {
			t.Errorf("%s.Fit() accepted data containing 0, want failure", f.Name())
		}
	}
}
