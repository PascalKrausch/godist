package plot

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/PascalKrausch/godist/pkg/distribution"
)

func TestNewHistogramCountsAndDensityIntegrateToOne(t *testing.T) {
	sorted := []float64{0, 0.5, 1, 1.5, 2, 2.5, 3, 3.5, 4, 4.5, 5}
	h := NewHistogram(sorted, 5)

	total := 0
	area := 0.0
	width := h.Edges[1] - h.Edges[0]
	for i, c := range h.Counts {
		total += c
		area += h.Density[i] * width
	}
	if total != len(sorted) {
		t.Errorf("sum of counts = %d, want %d", total, len(sorted))
	}
	if math.Abs(area-1) > 1e-12 {
		t.Errorf("density area = %g, want 1", area)
	}
	if h.Edges[len(h.Edges)-1] != 5 {
		t.Errorf("last edge = %g, want 5", h.Edges[len(h.Edges)-1])
	}
}

func TestBuildGridIsCappedSortedAndECDFEndsAtOne(t *testing.T) {
	n := 10 * maxPoints
	sorted := make([]float64, n)
	for i := range sorted {
		sorted[i] = float64(i)
	}
	payload := BuildPayload(sorted, nil)

	if limit := gridPoints + maxBins + maxPoints; len(payload.Grid) > limit {
		t.Errorf("grid has %d points, want at most %d", len(payload.Grid), limit)
	}
	for i := 1; i < len(payload.Grid); i++ {
		if payload.Grid[i] <= payload.Grid[i-1] {
			t.Fatalf("grid not strictly increasing at index %d", i)
		}
	}
	if len(payload.ECDF) != len(payload.Grid) {
		t.Fatalf("ECDF has %d values, grid has %d", len(payload.ECDF), len(payload.Grid))
	}
	if last := payload.ECDF[len(payload.ECDF)-1]; last != 1 {
		t.Errorf("last ECDF value = %g, want 1", last)
	}
}

func TestNewQQMatchesExactModelQuantiles(t *testing.T) {
	dist := distribution.Normal{Mu: 2, Sigma: 3}
	n := 200
	sorted := make([]float64, n)
	for i := range sorted {
		sorted[i] = dist.Quantile((float64(i) + 0.5) / float64(n))
	}

	qq := NewQQ(sorted, dist)
	if len(qq.X) != n {
		t.Fatalf("QQ has %d points, want %d", len(qq.X), n)
	}
	for i := range qq.X {
		if math.Abs(qq.X[i]-qq.Y[i]) > 1e-9 {
			t.Errorf("QQ point %d = (%g, %g), want x == y", i, qq.X[i], qq.Y[i])
		}
	}
}

// stubDist liefert vorgegebene Funktionen; nicht gesetzte Methoden dürfen nicht aufgerufen werden.
type stubDist struct {
	distribution.ContinuousDistribution
	pdf, cdf, quantile func(float64) float64
	params             []float64
	names              []string
}

func (s stubDist) PDF(x float64) float64      { return s.pdf(x) }
func (s stubDist) CDF(x float64) float64      { return s.cdf(x) }
func (s stubDist) Quantile(p float64) float64 { return s.quantile(p) }
func (s stubDist) Params() []float64          { return s.params }
func (s stubDist) ParamNames() []string       { return s.names }

func TestBuildPayloadSharesGridAndMarshalsInvalidValuesAsNull(t *testing.T) {
	dist := stubDist{
		pdf: func(x float64) float64 {
			if x > 1 {
				return math.NaN()
			}
			return 1
		},
		cdf:      func(x float64) float64 { return 0.5 },
		quantile: func(p float64) float64 { return p },
	}
	payload := BuildPayload([]float64{0, 0.25, 0.5, 0.75, 1}, []Model{{Name: "stub", Dist: dist}})

	if len(payload.Models) != 1 {
		t.Fatalf("got %d models, want 1", len(payload.Models))
	}
	m := payload.Models[0]
	if len(m.PDF) != len(payload.Grid) || len(m.CDF) != len(payload.Grid) {
		t.Fatalf("model curves have lengths %d/%d, grid has %d", len(m.PDF), len(m.CDF), len(payload.Grid))
	}
	nulls := 0
	for _, v := range m.PDF {
		if v == nil {
			nulls++
		}
	}
	if nulls == 0 {
		t.Error("expected null PDF values for NaN input")
	}
	if _, err := json.Marshal(payload); err != nil {
		t.Errorf("payload does not marshal to JSON: %v", err)
	}
}

func TestBuildPayloadExposesParamsAndMetricsWithNullForInvalid(t *testing.T) {
	dist := stubDist{
		pdf:      func(x float64) float64 { return 1 },
		cdf:      func(x float64) float64 { return 0.5 },
		quantile: func(p float64) float64 { return p },
		params:   []float64{2.5, math.NaN()},
		names:    []string{"shape", "scale"},
	}
	metrics := distribution.FitMetrics{AIC: 10.5, BIC: 12.25, KSStatistic: 0.1, RMSE: math.Inf(1)}
	payload := BuildPayload([]float64{0.2, 0.4, 0.6, 0.8}, []Model{{Name: "stub", Dist: dist, Metrics: metrics}})

	m := payload.Models[0]
	if len(m.Params) != 2 || m.Params[0].Name != "shape" || m.Params[1].Name != "scale" {
		t.Fatalf("params = %+v, want shape, scale in order", m.Params)
	}
	if m.Params[0].Value == nil || *m.Params[0].Value != 2.5 {
		t.Errorf("shape value = %v, want 2.5", m.Params[0].Value)
	}
	if m.Params[1].Value != nil {
		t.Errorf("scale value = %v, want null for NaN", *m.Params[1].Value)
	}
	if m.Metrics.AIC == nil || *m.Metrics.AIC != 10.5 || m.Metrics.BIC == nil || *m.Metrics.BIC != 12.25 {
		t.Errorf("AIC/BIC = %v/%v, want 10.5/12.25", m.Metrics.AIC, m.Metrics.BIC)
	}
	if m.Metrics.KS == nil || *m.Metrics.KS != 0.1 {
		t.Errorf("KS = %v, want 0.1", m.Metrics.KS)
	}
	if m.Metrics.RMSE != nil {
		t.Errorf("RMSE = %v, want null for Inf", *m.Metrics.RMSE)
	}
	if _, err := json.Marshal(payload); err != nil {
		t.Errorf("payload does not marshal to JSON: %v", err)
	}
}

func TestBuildPayloadIncludesDescriptiveStats(t *testing.T) {
	payload := BuildPayload([]float64{4, 1, 3, 2}, nil)
	if payload.Descriptive.N != 4 || payload.Descriptive.Median != 2.5 || payload.Descriptive.Q1 != 1.75 || payload.Descriptive.Q3 != 3.25 {
		t.Errorf("descriptive = %+v, want N=4, median=2.5, q1=1.75, q3=3.25", payload.Descriptive)
	}
}

func TestBuildPayloadConstantAndEmptyData(t *testing.T) {
	models := []Model{{Name: "Normal", Dist: distribution.Normal{Mu: 2, Sigma: 1}}}

	constant := BuildPayload([]float64{2, 2, 2}, models)
	if len(constant.Histogram.Counts) != 1 || constant.Histogram.Counts[0] != 3 {
		t.Errorf("constant data histogram = %+v, want one bin with 3 counts", constant.Histogram)
	}
	if len(constant.Models) != 1 || len(constant.Models[0].PDF) != len(constant.Grid) {
		t.Errorf("constant data should still produce model curves on the grid, got %+v", constant.Models)
	}

	empty := BuildPayload(nil, models)
	if empty.SampleSize != 0 || len(empty.Models) != 0 {
		t.Errorf("empty data payload = %+v, want sample size 0 and no model curves", empty)
	}
}
