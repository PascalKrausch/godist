package stats

import (
	"encoding/json"
	"math"
	"testing"
)

// Referenzwerte aus SciPy (skew/kurtosis mit bias=False) und NumPy (percentile linear, ddof=1).
func TestDescribeMatchesReference(t *testing.T) {
	tests := []struct {
		name string
		data []float64
		want Summary
	}{
		{
			name: "symmetrisch mit Wiederholungen",
			data: []float64{2, 4, 4, 4, 5, 5, 7, 9},
			want: Summary{
				N: 8, Mean: 5, Median: 4.5, Variance: 4.571428571428571, StdDev: 2.138089935299395,
				Min: 2, Q1: 4, Q3: 5.5, Max: 9,
				Skewness: 0.8184875533567996, Kurtosis: 0.9406249999999998,
			},
		},
		{
			name: "rechtsschief mit Ausreißer",
			data: []float64{1, 2, 3, 4, 10},
			want: Summary{
				N: 5, Mean: 4, Median: 3, Variance: 12.5, StdDev: 3.5355339059327378,
				Min: 1, Q1: 2, Q3: 4, Max: 10,
				Skewness: 1.6970562748477143, Kurtosis: 3.152000000000001,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Describe(tt.data)
			if got.N != tt.want.N {
				t.Errorf("N = %d, want %d", got.N, tt.want.N)
			}
			assertClose(t, "Mean", got.Mean, tt.want.Mean)
			assertClose(t, "Median", got.Median, tt.want.Median)
			assertClose(t, "Variance", got.Variance, tt.want.Variance)
			assertClose(t, "StdDev", got.StdDev, tt.want.StdDev)
			assertClose(t, "Min", got.Min, tt.want.Min)
			assertClose(t, "Q1", got.Q1, tt.want.Q1)
			assertClose(t, "Q3", got.Q3, tt.want.Q3)
			assertClose(t, "Max", got.Max, tt.want.Max)
			assertClose(t, "Skewness", got.Skewness, tt.want.Skewness)
			assertClose(t, "Kurtosis", got.Kurtosis, tt.want.Kurtosis)
		})
	}
}

func TestDescribeDoesNotModifyInput(t *testing.T) {
	data := []float64{5, 1, 4, 2, 3}
	Describe(data)
	want := []float64{5, 1, 4, 2, 3}
	for i := range data {
		if data[i] != want[i] {
			t.Fatalf("input modified: %v", data)
		}
	}
}

func TestDescribeEdgeCases(t *testing.T) {
	t.Run("leer", func(t *testing.T) {
		got := Describe(nil)
		if got.N != 0 {
			t.Errorf("N = %d, want 0", got.N)
		}
		assertNaN(t, "Mean", got.Mean)
		assertNaN(t, "Median", got.Median)
		assertNaN(t, "Max", got.Max)
	})

	t.Run("ein Wert", func(t *testing.T) {
		got := Describe([]float64{7})
		assertClose(t, "Mean", got.Mean, 7)
		assertClose(t, "Median", got.Median, 7)
		assertClose(t, "Min", got.Min, 7)
		assertClose(t, "Max", got.Max, 7)
		assertNaN(t, "Variance", got.Variance)
		assertNaN(t, "StdDev", got.StdDev)
		assertNaN(t, "Skewness", got.Skewness)
		assertNaN(t, "Kurtosis", got.Kurtosis)
	})

	t.Run("konstante Werte", func(t *testing.T) {
		got := Describe([]float64{3, 3, 3, 3})
		assertClose(t, "Variance", got.Variance, 0)
		assertClose(t, "StdDev", got.StdDev, 0)
		assertNaN(t, "Skewness", got.Skewness)
		assertNaN(t, "Kurtosis", got.Kurtosis)
	})

	t.Run("zu wenige Werte für Schiefe und Kurtosis", func(t *testing.T) {
		two := Describe([]float64{1, 2})
		assertClose(t, "StdDev", two.StdDev, math.Sqrt(0.5))
		assertNaN(t, "Skewness", two.Skewness)
		assertNaN(t, "Kurtosis", two.Kurtosis)

		three := Describe([]float64{1, 2, 4})
		assertClose(t, "Skewness", three.Skewness, 0.9352195295828235)
		assertNaN(t, "Kurtosis", three.Kurtosis)
	})

	t.Run("Quartile mit linearer Interpolation", func(t *testing.T) {
		got := Describe([]float64{4, 1, 3, 2})
		assertClose(t, "Q1", got.Q1, 1.75)
		assertClose(t, "Median", got.Median, 2.5)
		assertClose(t, "Q3", got.Q3, 3.25)
	})
}

func TestSummaryMarshalJSONWritesInvalidValuesAsNull(t *testing.T) {
	data, err := json.Marshal(Describe([]float64{1, 2}))
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got["skewness"] != nil || got["kurtosis"] != nil {
		t.Errorf("skewness/kurtosis for n=2 = %v/%v, want null", got["skewness"], got["kurtosis"])
	}
	if got["mean"] != 1.5 {
		t.Errorf("mean = %v, want 1.5", got["mean"])
	}
}

func assertClose(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.IsNaN(want) {
		assertNaN(t, name, got)
		return
	}
	if math.Abs(got-want) > 1e-9*math.Max(1, math.Abs(want)) {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func assertNaN(t *testing.T, name string, got float64) {
	t.Helper()
	if !math.IsNaN(got) {
		t.Errorf("%s = %v, want NaN", name, got)
	}
}
