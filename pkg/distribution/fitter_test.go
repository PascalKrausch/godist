package distribution

import (
	"math"
	"testing"
)

func TestSelectBestModel(t *testing.T) {
	tests := []struct {
		name        string
		data        []float64
		wantDist    bool
		wantMetrics bool
		wantErr     bool
	}{
		{
			name:        "gültige Daten",
			data:        []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0},
			wantDist:    true,
			wantMetrics: true,
			wantErr:     false,
		},
		{
			name:        "leere Daten",
			data:        []float64{},
			wantDist:    false,
			wantMetrics: false,
			wantErr:     true,
		},
		{
			name:        "ungleiche Daten",
			data:        []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 50.0, 100.0},
			wantDist:    true,
			wantMetrics: true,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist, metrics, err := SelectBestModel(tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("SelectBestModel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantDist && dist == nil {
				t.Error("SelectBestModel() dist ist nil, aber erwartet nicht-nil")
			}

			if tt.wantMetrics {
				if metrics.SampleSize == 0 {
					t.Error("SelectBestModel() metrics.SampleSize ist 0")
				}
				if math.IsNaN(metrics.AIC) || math.IsInf(metrics.AIC, 0) {
					t.Errorf("SelectBestModel() metrics.AIC ist ungültig: %v", metrics.AIC)
				}
				if math.IsNaN(metrics.BIC) || math.IsInf(metrics.BIC, 0) {
					t.Errorf("SelectBestModel() metrics.BIC ist ungültig: %v", metrics.BIC)
				}
			}
		})
	}
}

func TestSelectBestModelLowestAIC(t *testing.T) {
	data := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}

	_, bestMetrics, err := SelectBestModel(data)
	if err != nil {
		t.Fatalf("SelectBestModel() unexpected error: %v", err)
	}

	results, _, err := SelectAllModels(data)
	if err != nil {
		t.Fatalf("SelectAllModels() unexpected error: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("SelectAllModels() keine Ergebnisse")
	}

	lowestAIC := results[0].Metrics.AIC
	for _, result := range results {
		if result.Metrics.AIC < lowestAIC {
			lowestAIC = result.Metrics.AIC
		}
	}

	if bestMetrics.AIC != lowestAIC {
		t.Errorf("SelectBestModel() AIC = %v, aber niedrigster AIC ist %v", bestMetrics.AIC, lowestAIC)
	}

	for _, result := range results {
		if result.Metrics.AIC == lowestAIC {
			if result.Dist.Params() == nil {
				t.Error("SelectBestModel() Dist hat keine Parameter")
			}
			break
		}
	}
}

func TestSelectBestModelConsistencyWithSelectAllModels(t *testing.T) {
	data := []float64{0.5, 1.0, 1.5, 2.0, 2.5, 3.0, 3.5, 4.0, 4.5, 5.0}

	_, bestMetrics, err := SelectBestModel(data)
	if err != nil {
		t.Fatalf("SelectBestModel() unexpected error: %v", err)
	}

	results, _, err := SelectAllModels(data)
	if err != nil {
		t.Fatalf("SelectAllModels() unexpected error: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("SelectAllModels() keine Ergebnisse")
	}

	minAIC := math.Inf(1)
	var matchingResult *FitResult
	for i := range results {
		if results[i].Metrics.AIC < minAIC {
			minAIC = results[i].Metrics.AIC
			matchingResult = &results[i]
		}
	}

	if matchingResult == nil {
		t.Fatal("Kein Modell mit gültigem AIC gefunden")
	}

	if bestMetrics.AIC != matchingResult.Metrics.AIC {
		t.Errorf("SelectBestModel() AIC %v != SelectAllModels() niedrigster AIC %v",
			bestMetrics.AIC, matchingResult.Metrics.AIC)
	}

	if bestMetrics.BIC != matchingResult.Metrics.BIC {
		t.Errorf("SelectBestModel() BIC %v != SelectAllModels() BIC %v",
			bestMetrics.BIC, matchingResult.Metrics.BIC)
	}

	if bestMetrics.SampleSize != matchingResult.Metrics.SampleSize {
		t.Errorf("SelectBestModel() SampleSize %v != SelectAllModels() SampleSize %v",
			bestMetrics.SampleSize, matchingResult.Metrics.SampleSize)
	}
}
