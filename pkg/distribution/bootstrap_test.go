package distribution

import (
	"math"
	"testing"
)

func fitWeibullForBootstrap(t *testing.T, data []float64) (ContinuousDistribution, FitMetrics) {
	t.Helper()
	dist, err := WeibullFitter{}.Fit(data)
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	metrics, err := EvaluateFit(dist, data)
	if err != nil {
		t.Fatalf("EvaluateFit: %v", err)
	}
	return dist, metrics
}

func bootstrapConfigForTest(b int, method BootstrapMethod) BootstrapConfig {
	return BootstrapConfig{B: b, Method: method, Level: 0.95, Seed: 12345}
}

// Nicht-parametrisches Bootstrap auf Weibull-Daten: Die Intervalle der Parameter
// sollten die MLE-Schätzung einschließen und eine endliche Breite haben.
func TestBootstrapFitNonParametricWeibull(t *testing.T) {
	data := sampleWeibull(newTestRNG(), 2.0, 3.0, 500)
	dist, _ := fitWeibullForBootstrap(t, data)

	res, err := BootstrapFit(WeibullFitter{}, dist, data, bootstrapConfigForTest(200, BootNonParametric))
	if err != nil {
		t.Fatalf("BootstrapFit: %v", err)
	}
	if res.Replicates != 200 || res.Failed != 0 {
		t.Fatalf("Replicates = %d, Failed = %d; want 200/0", res.Replicates, res.Failed)
	}
	if res.Method != "nonparametric" || res.Level != 0.95 || res.Seed != 12345 {
		t.Errorf(" unerwartete Metadaten: %+v", res)
	}
	if len(res.Params) != 2 {
		t.Fatalf("Params = %d, want 2", len(res.Params))
	}
	for _, p := range res.Params {
		if !(p.Lo <= p.Mean && p.Mean <= p.Hi) || p.Lo >= p.Hi {
			t.Errorf("Param %s: ungültiges Intervall mean=%g [%g, %g]", p.Name, p.Mean, p.Lo, p.Hi)
		}
	}
	for _, key := range []string{"aic", "bic", "ks", "rmse"} {
		if _, ok := res.Metrics[key]; !ok {
			t.Errorf("Metrics fehlt %q", key)
		}
	}
}

// Parametrisches Bootstrap: Stichproben aus der gefitteten Verteilung, gleiche Prüfung.
func TestBootstrapFitParametricWeibull(t *testing.T) {
	data := sampleWeibull(newTestRNG(), 2.0, 3.0, 500)
	dist, _ := fitWeibullForBootstrap(t, data)

	res, err := BootstrapFit(WeibullFitter{}, dist, data, bootstrapConfigForTest(200, BootParametric))
	if err != nil {
		t.Fatalf("BootstrapFit: %v", err)
	}
	if res.Method != "parametric" {
		t.Errorf("Method = %q, want parametric", res.Method)
	}
	if res.Replicates < 190 {
		t.Fatalf("Replicates = %d, zu viele Fehlschläge", res.Replicates)
	}
	for _, p := range res.Params {
		if math.IsNaN(p.Mean) || math.IsNaN(p.Lo) || math.IsNaN(p.Hi) {
			t.Errorf("Param %s: NaN im Intervall", p.Name)
		}
	}
}

// Fester Seed muss identische Ergebnisse liefern (Reproduzierbarkeit).
func TestBootstrapFitDeterministicWithSeed(t *testing.T) {
	data := sampleNormal(newTestRNG(), 5, 2, 200)
	normal, err := NormalFitter{}.Fit(data)
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	cfg := bootstrapConfigForTest(50, BootNonParametric)
	r1, err := BootstrapFit(NormalFitter{}, normal, data, cfg)
	if err != nil {
		t.Fatalf("BootstrapFit 1: %v", err)
	}
	r2, err := BootstrapFit(NormalFitter{}, normal, data, cfg)
	if err != nil {
		t.Fatalf("BootstrapFit 2: %v", err)
	}
	if len(r1.Params) != len(r2.Params) {
		t.Fatal("unterschiedliche Parameterzahl")
	}
	for i := range r1.Params {
		if r1.Params[i] != r2.Params[i] {
			t.Errorf("Param %d unterscheidet sich: %+v vs %+v", i, r1.Params[i], r2.Params[i])
		}
	}
}

// Seed 0 darf nicht reproduzierbar sein müssen, aber muss ein Ergebnis liefern.
func TestBootstrapFitRandomSeed(t *testing.T) {
	data := sampleWeibull(newTestRNG(), 1.5, 2.0, 200)
	dist, _ := fitWeibullForBootstrap(t, data)
	cfg := bootstrapConfigForTest(20, BootNonParametric)
	cfg.Seed = 0
	res, err := BootstrapFit(WeibullFitter{}, dist, data, cfg)
	if err != nil {
		t.Fatalf("BootstrapFit: %v", err)
	}
	if res.Seed == 0 {
		t.Error("Seed wurde nicht intern erzeugt")
	}
}

func TestBootstrapFitRejectsInvalidConfig(t *testing.T) {
	data := sampleWeibull(newTestRNG(), 2, 3, 100)
	dist, _ := fitWeibullForBootstrap(t, data)

	cases := []BootstrapConfig{
		{B: 0, Method: BootNonParametric, Level: 0.95, Seed: 1},
		{B: -5, Method: BootNonParametric, Level: 0.95, Seed: 1},
		{B: 10, Method: BootNonParametric, Level: 0, Seed: 1},
		{B: 10, Method: BootNonParametric, Level: 1, Seed: 1},
		{B: 10, Method: BootstrapMethod(99), Level: 0.95, Seed: 1},
	}
	for i, cfg := range cases {
		if _, err := BootstrapFit(WeibullFitter{}, dist, data, cfg); err == nil {
			t.Errorf("Fall %d: erwarteter Fehler", i)
		}
	}
}

// Zu wenig erfolgreiche Replicates müssen einen Fehler ergeben, nicht stille CIs.
func TestBootstrapFitFailsWhenAllReplicatesFail(t *testing.T) {
	// Pareto-Fit auf Daten mit Negativwerten schlägt in jeder Replicate fehl.
	data := []float64{-1, -2, -3, -4, -5}
	dist := Pareto{Shape: 2, Scale: 1}
	_, err := BootstrapFit(ParetoFitter{}, dist, data, bootstrapConfigForTest(10, BootNonParametric))
	if err == nil {
		t.Fatal("erwarteter Fehler bei durchgehend fehlgeschlagenen Replicates")
	}
}

// Leere Daten sind ein Fehler.
func TestBootstrapFitEmptyData(t *testing.T) {
	dist := Normal{Mu: 0, Sigma: 1}
	if _, err := BootstrapFit(NormalFitter{}, dist, nil, bootstrapConfigForTest(10, BootNonParametric)); err == nil {
		t.Fatal("erwarteter Fehler bei leeren Daten")
	}
}

// Benchmark: nicht-parametrisches Bootstrap auf realistic-sized Daten,
// damit die Performance der Worker-Pool-Implementierung messbar bleibt.
func BenchmarkBootstrapFitNonParametric(b *testing.B) {
	data := sampleWeibull(newTestRNG(), 2.0, 3.0, 1000)
	dist, err := WeibullFitter{}.Fit(data)
	if err != nil {
		b.Fatal(err)
	}
	cfg := bootstrapConfigForTest(100, BootNonParametric)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := BootstrapFit(WeibullFitter{}, dist, data, cfg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBootstrapFitParametric(b *testing.B) {
	data := sampleWeibull(newTestRNG(), 2.0, 3.0, 1000)
	dist, err := WeibullFitter{}.Fit(data)
	if err != nil {
		b.Fatal(err)
	}
	cfg := bootstrapConfigForTest(100, BootParametric)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := BootstrapFit(WeibullFitter{}, dist, data, cfg); err != nil {
			b.Fatal(err)
		}
	}
}
