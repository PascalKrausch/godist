package distribution

import (
	"math"
	"reflect"
	"testing"
)

func TestSilvermanCriticalBandwidthReducesModes(t *testing.T) {
	data := []float64{-3.2, -3.1, -3.0, -2.9, -2.8, 2.8, 2.9, 3.0, 3.1, 3.2}
	h := criticalBandwidth(data, 1)
	if h <= 0 {
		t.Fatalf("critical bandwidth = %g, want positive", h)
	}
	if got := countModes(data, h); got > 1 {
		t.Errorf("mode count at critical bandwidth = %d, want at most 1", got)
	}
	hTwoModes := criticalBandwidth(data, 2)
	if hTwoModes <= 0 || hTwoModes >= h {
		t.Errorf("critical bandwidth for at most 2 modes = %g, want between 0 and h_1=%g", hTwoModes, h)
	}
	if got := countModes(data, hTwoModes); got > 2 {
		t.Errorf("mode count at h_2 = %d, want at most 2", got)
	}
}

func TestSilvermanTestReproducibilityAndAffineScale(t *testing.T) {
	data := []float64{-3.2, -3.1, -3.0, -2.9, -2.8, 2.8, 2.9, 3.0, 3.1, 3.2}
	first, err := SilvermanTest(data, 1, 24, 17)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := SilvermanTest(data, 1, 24, 17)
	if err != nil {
		t.Fatal(err)
	}
	if first != repeated {
		t.Errorf("same seed produced different results: %+v vs %+v", first, repeated)
	}

	transformed := make([]float64, len(data))
	for i, value := range data {
		transformed[i] = 4 + 7*value
	}
	scaled, err := SilvermanTest(transformed, 1, 24, 17)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(scaled.CriticalBandwidth-7*first.CriticalBandwidth) > 1e-3 {
		t.Errorf("scaled critical bandwidth = %g, want %g", scaled.CriticalBandwidth, 7*first.CriticalBandwidth)
	}
	if math.Abs(scaled.ReferenceBandwidth-7*first.ReferenceBandwidth) > 1e-12 {
		t.Errorf("scaled reference bandwidth = %g, want %g", scaled.ReferenceBandwidth, 7*first.ReferenceBandwidth)
	}
	if scaled.PValue != first.PValue {
		t.Errorf("affine transform changed p-value: %g vs %g", scaled.PValue, first.PValue)
	}
}

func TestSilvermanTestFindsStrongBimodality(t *testing.T) {
	data := []float64{-3.2, -3.1, -3.0, -2.9, -2.8, -3.15, -3.05, -2.95, -2.85, 2.8, 2.9, 3.0, 3.1, 3.2, 2.85, 2.95, 3.05, 3.15}
	result, err := SilvermanTest(data, 1, 99, 31)
	if err != nil {
		t.Fatal(err)
	}
	if result.PValue >= 0.05 {
		t.Errorf("strongly bimodal sample p-value = %g, want < 0.05", result.PValue)
	}
}

func TestSilvermanTestFiltersNonFiniteAndHandlesConstantData(t *testing.T) {
	clean, err := SilvermanTest([]float64{1, 2, 3, 4, 5}, 1, 10, 4)
	if err != nil {
		t.Fatal(err)
	}
	dirty, err := SilvermanTest([]float64{1, math.NaN(), 2, math.Inf(1), 3, 4, 5}, 1, 10, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(clean, dirty) {
		t.Errorf("non-finite values changed result: clean=%+v dirty=%+v", clean, dirty)
	}

	constant, err := SilvermanTest([]float64{3, 3, 3, 3}, 1, 10, 4)
	if err != nil {
		t.Fatal(err)
	}
	if constant.PValue != 1 || constant.CriticalBandwidth != 0 || constant.BandwidthRatio != 0 {
		t.Errorf("constant-data result = %+v, want p=1 and zero bandwidths", constant)
	}
}

func TestSilvermanTestRejectsInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct {
		modes, simulations int
	}{
		{modes: 0, simulations: 10},
		{modes: 1, simulations: 0},
	} {
		if _, err := SilvermanTest([]float64{1, 2, 3}, tc.modes, tc.simulations, 1); err == nil {
			t.Errorf("SilvermanTest(%d, %d) returned no error", tc.modes, tc.simulations)
		}
	}
}
