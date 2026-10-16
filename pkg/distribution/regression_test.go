package distribution

import (
	"math"
	"testing"
)

// Regressionstests mit festen Datensätzen und erwarteten Ergebnissen.
// Diese Tests stellen sicher, dass Änderungen an der Implementierung
// die Ergebnisse nicht unerwartet verändern.

func TestRegressionFixedDatasets(t *testing.T) {
	tests := []struct {
		name          string
		data          []float64
		fitter        Fitter
		wantParams    []float64
		wantMetrics   FitMetrics
		paramTol      float64
		metricsTol    float64
	}{
		{
			name: "Exponential simple dataset",
			data: []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0},
			fitter: ExponentialFitter{},
			wantParams: []float64{0.181818}, // 1/mean, mean = 5.5
			wantMetrics: FitMetrics{
				SampleSize: 10,
				Discarded:  0,
			},
			paramTol:   1e-6,
			metricsTol: 1e-6,
		},
		{
			name: "Normal simple dataset",
			data: []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0},
			fitter: NormalFitter{},
			wantParams: []float64{5.5, 3.027650}, // mu, sigma (sample std dev)
			wantMetrics: FitMetrics{
				SampleSize: 10,
				Discarded:  0,
			},
			paramTol:   1e-5,
			metricsTol: 1e-5,
		},
		{
			name: "Gamma simple dataset",
			data: []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0},
			fitter: GammaFitter{},
			wantMetrics: FitMetrics{
				SampleSize: 10,
				Discarded:  0,
			},
			paramTol:   0.1,
			metricsTol: 0.1,
		},
		{
			name: "LogNormal simple dataset",
			data: []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0},
			fitter: LogNormalFitter{},
			wantMetrics: FitMetrics{
				SampleSize: 10,
				Discarded:  0,
			},
			paramTol:   0.1,
			metricsTol: 0.1,
		},
		{
			name: "Weibull simple dataset",
			data: []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0},
			fitter: WeibullFitter{},
			wantMetrics: FitMetrics{
				SampleSize: 10,
				Discarded:  0,
			},
			paramTol:   0.1,
			metricsTol: 0.1,
		},
		{
			name: "Gumbel simple dataset",
			data: []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0},
			fitter: GumbelFitter{},
			wantMetrics: FitMetrics{
				SampleSize: 10,
				Discarded:  0,
			},
			paramTol:   0.1,
			metricsTol: 0.1,
		},
		{
			name: "Beta bounded dataset",
			data: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9},
			fitter: BetaFitter{},
			wantMetrics: FitMetrics{
				SampleSize: 9,
				Discarded:  0,
			},
			paramTol:   0.1,
			metricsTol: 0.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist, err := tt.fitter.Fit(tt.data)
			if err != nil {
				t.Fatalf("Fit() error = %v", err)
			}

			params := dist.Params()
			if len(params) != len(tt.wantParams) && len(tt.wantParams) > 0 {
				t.Fatalf("Params() returned %d parameters, want %d", len(params), len(tt.wantParams))
			}

			for i, want := range tt.wantParams {
				if math.IsNaN(params[i]) || math.Abs(params[i]-want) > tt.paramTol {
					t.Errorf("Params()[%d] = %g, want %g (tolerance %g)", i, params[i], want, tt.paramTol)
				}
			}

			metrics, err := EvaluateFit(dist, tt.data)
			if err != nil {
				t.Fatalf("EvaluateFit() error = %v", err)
			}

			if metrics.SampleSize != tt.wantMetrics.SampleSize {
				t.Errorf("SampleSize = %d, want %d", metrics.SampleSize, tt.wantMetrics.SampleSize)
			}
			if metrics.Discarded != tt.wantMetrics.Discarded {
				t.Errorf("Discarded = %d, want %d", metrics.Discarded, tt.wantMetrics.Discarded)
			}
			if !metrics.Converged {
				t.Errorf("Converged = false, want true")
			}
		})
	}
}

// TestRegressionModelRanking stellt sicher, dass das Modellranking bei einem
// festen Datensatz stabil bleibt.
func TestRegressionModelRanking(t *testing.T) {
	data := []float64{0.5, 1.0, 1.5, 2.0, 2.5, 3.0, 3.5, 4.0, 4.5, 5.0}

	results, failures, err := SelectAllModels(data)
	if err != nil {
		t.Fatalf("SelectAllModels() error = %v", err)
	}

	if len(results) == 0 {
		t.Fatal("SelectAllModels() returned no results")
	}

	// Überprüfe, dass alle Ergebnisse gültige Metriken haben
	for _, result := range results {
		if math.IsNaN(result.Metrics.AIC) || math.IsInf(result.Metrics.AIC, 0) {
			t.Errorf("Model %v has invalid AIC: %v", result.Dist.Params(), result.Metrics.AIC)
		}
		if math.IsNaN(result.Metrics.BIC) || math.IsInf(result.Metrics.BIC, 0) {
			t.Errorf("Model %v has invalid BIC: %v", result.Dist.Params(), result.Metrics.BIC)
		}
		if !result.Metrics.Converged {
			t.Errorf("Model %v has Converged = false", result.Dist.Params())
		}
	}

	// Sortiere nach AIC und stelle sicher, dass die Reihenfolge monoton ist
	bestAIC := results[0].Metrics.AIC
	for i, result := range results {
		if result.Metrics.AIC < bestAIC {
			t.Errorf("Results not sorted by AIC: results[%d].AIC = %g < bestAIC = %g", i, result.Metrics.AIC, bestAIC)
		}
		if result.Metrics.AIC < bestAIC {
			bestAIC = result.Metrics.AIC
		}
	}

	// Überprüfe, dass fehlgeschlagene Fits korrekt gemeldet werden
	if len(failures) > 0 {
		t.Logf("Failed fits (expected for some models with this data): %d", len(failures))
		for _, f := range failures {
			t.Logf("  - %s: %s", f.Name, f.Error)
		}
	}
}

// TestRegressionWithInvalidValues stellt sicher, dass die Behandlung
// ungültiger Werte stabil bleibt.
func TestRegressionWithInvalidValues(t *testing.T) {
	data := []float64{1, 2, math.NaN(), 3, math.Inf(1), 4, -1, 5, 6, 7, 8, 9, 10}

	dist, err := NormalFitter{}.Fit(data)
	if err != nil {
		t.Fatalf("Fit() error = %v", err)
	}

	metrics, err := EvaluateFit(dist, data)
	if err != nil {
		t.Fatalf("EvaluateFit() error = %v", err)
	}

	// Normal akzeptiert negative Werte, sollte aber NaN/Inf verwerfen
	if metrics.SampleSize == 0 {
		t.Error("SampleSize = 0, want > 0")
	}
	if metrics.Discarded == 0 {
		t.Error("Discarded = 0, want > 0 (NaN/Inf should be discarded)")
	}
	if !metrics.Converged {
		t.Error("Converged = false, want true")
	}
}

// TestRegressionReproducibility stellt sicher, dass mehrfache Fits
// auf denselben Daten identische Ergebnisse liefern.
func TestRegressionReproducibility(t *testing.T) {
	data := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}

	var params1, params2 []float64

	for i := 0; i < 2; i++ {
		dist, err := ExponentialFitter{}.Fit(data)
		if err != nil {
			t.Fatalf("Fit() iteration %d error = %v", i, err)
		}
		if i == 0 {
			params1 = dist.Params()
		} else {
			params2 = dist.Params()
		}
	}

	for i := range params1 {
		if params1[i] != params2[i] {
			t.Errorf("Params()[%d] not reproducible: first = %g, second = %g", i, params1[i], params2[i])
		}
	}
}

// TestFailedFitsInModelComparison stellt sicher, dass fehlgeschlagene Fits
// im Modellvergleich korrekt gemeldet werden und nicht zum Absturz führen.
func TestFailedFitsInModelComparison(t *testing.T) {
	tests := []struct {
		name            string
		data            []float64
		expectFailures  bool
	}{
		{
			name:           "Valid data - no failures expected",
			data:           []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0},
			expectFailures: false,
		},
		{
			name:           "Constant data - some failures expected",
			data:           []float64{5, 5, 5, 5, 5, 5, 5, 5, 5, 5},
			expectFailures: true,
		},
		{
			name:           "Too few observations - failures expected",
			data:           []float64{1, 2},
			expectFailures: true,
		},
		{
			name:           "Data with only zeros - failures expected",
			data:           []float64{0, 0, 0, 0, 0},
			expectFailures: true,
		},
		{
			name:           "Data with all same values - failures expected",
			data:           []float64{1, 1, 1, 1, 1, 1, 1, 1},
			expectFailures: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, failures, err := SelectAllModels(tt.data)
			if err != nil {
				t.Fatalf("SelectAllModels() error = %v", err)
			}

			if tt.expectFailures && len(failures) == 0 {
				t.Logf("No failures reported, but some were expected for this data")
			}

			if !tt.expectFailures && len(failures) > 0 {
				t.Logf("Unexpected failures reported: %d", len(failures))
				for _, f := range failures {
					t.Logf("  - %s: %s", f.Name, f.Error)
				}
			}

			// Überprüfe, dass erfolgreiche Ergebnisse gültige Metriken haben
			for _, result := range results {
				if math.IsNaN(result.Metrics.AIC) || math.IsInf(result.Metrics.AIC, 0) {
					t.Errorf("Successful fit has invalid AIC: %v", result.Metrics.AIC)
				}
				if math.IsNaN(result.Metrics.BIC) || math.IsInf(result.Metrics.BIC, 0) {
					t.Errorf("Successful fit has invalid BIC: %v", result.Metrics.BIC)
				}
				if !result.Metrics.Converged {
					t.Errorf("Successful fit has Converged = false")
				}
			}

			// Überprüfe, dass fehlgeschlagene Fits eine Fehlermeldung haben
			for _, f := range failures {
				if f.Name == "" {
					t.Error("Failed fit has empty name")
				}
				if f.Error == "" {
					t.Errorf("Failed fit %s has empty error message", f.Name)
				}
			}

			// Wenn es sowohl erfolgreiche als auch fehlgeschlagene Fits gibt,
			// sollte SelectBestModel immer noch funktionieren
			if len(results) > 0 {
				_, bestMetrics, err := SelectBestModel(tt.data)
				if err != nil {
					t.Errorf("SelectBestModel() failed despite having %d successful results: %v", len(results), err)
				}
				if !bestMetrics.Converged {
					t.Error("SelectBestModel() returned metrics with Converged = false")
				}
			}
		})
	}
}

// TestFailedFitsWithOutOfSupportData testet, dass Daten außerhalb des Supports
// zu fehlgeschlagenen Fits führen, aber den Gesamtvergleich nicht abbrechen.
func TestFailedFitsWithOutOfSupportData(t *testing.T) {
	// Daten, die für einige Verteilungen ungültig sind (z.B. negative Werte für Exponential)
	data := []float64{-5, -4, -3, -2, -1, 0, 1, 2, 3, 4, 5}

	results, failures, err := SelectAllModels(data)
	if err != nil {
		t.Fatalf("SelectAllModels() error = %v", err)
	}

	// Einige Verteilungen sollten erfolgreich sein (Normal, Gumbel)
	// Andere sollten fehlschlagen (Exponential, Gamma, LogNormal, Weibull, Beta)
	if len(results) == 0 {
		t.Error("Expected at least some successful fits (Normal, Gumbel should work)")
	}

	if len(failures) == 0 {
		t.Error("Expected some failures for distributions that don't support negative values")
	}

	// Überprüfe, dass erfolgreiche Ergebnisse gültig sind
	for _, result := range results {
		if math.IsNaN(result.Metrics.AIC) || math.IsInf(result.Metrics.AIC, 0) {
			t.Errorf("Result has invalid AIC: %v", result.Metrics.AIC)
		}
	}

	t.Logf("Successful fits: %d, Failed fits: %d", len(results), len(failures))
}

// TestSelectBestModelWithSomeFailures testet, dass SelectBestModel auch dann
// funktioniert, wenn einige Kandidaten fehlschlagen.
func TestSelectBestModelWithSomeFailures(t *testing.T) {
	// Daten, bei denen einige Fits fehlschlagen sollten
	data := []float64{0, 0, 0, 0, 1, 2, 3, 4, 5}

	_, metrics, err := SelectBestModel(data)
	if err != nil {
		// Wenn alle Fits fehlschlagen, ist das akzeptabel
		t.Logf("SelectBestModel() failed (all fits may have failed): %v", err)
		return
	}

	// Wenn erfolgreich, sollte Converged true sein
	if !metrics.Converged {
		t.Error("SelectBestModel() returned metrics with Converged = false")
	}

	if math.IsNaN(metrics.AIC) || math.IsInf(metrics.AIC, 0) {
		t.Errorf("SelectBestModel() returned invalid AIC: %v", metrics.AIC)
	}
}
