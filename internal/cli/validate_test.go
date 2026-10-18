package cli

import (
	"strings"
	"testing"
)

func TestValidateSample(t *testing.T) {
	tests := []struct {
		name        string
		values      []float64
		wantErr     string
		wantWarning bool
	}{
		{"n=4 rejected", []float64{1, 2, 3, 4}, "zu wenige Werte", false},
		{"constant rejected", []float64{5, 5, 5, 5, 5, 5}, "identisch", false},
		{"n=5 warns", []float64{1, 2, 3, 4, 5}, "", true},
		{"n=20 warns", seq(20), "", true},
		{"n=21 no warning", seq(21), "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warning, err := ValidateSample(tt.values)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if (warning != "") != tt.wantWarning {
				t.Fatalf("warning = %q, wantWarning = %v", warning, tt.wantWarning)
			}
		})
	}
}

func seq(n int) []float64 {
	values := make([]float64, n)
	for i := range values {
		values[i] = float64(i + 1)
	}
	return values
}
