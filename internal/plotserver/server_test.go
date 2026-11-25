package plotserver

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PascalKrausch/godist/internal/plot"
	"github.com/PascalKrausch/godist/pkg/distribution"
)

const testAddr = "127.0.0.1:8080"

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	models := []plot.Model{{Name: "Normal", Dist: distribution.Normal{Mu: 3, Sigma: 1}}}
	source := func(bins int, kdeFactor float64) plot.Payload {
		return plot.BuildPayloadWithBins([]float64{1, 2, 3, 4, 5}, models, bins, kdeFactor)
	}
	handler, err := Handler(source, testAddr)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	return handler
}

func TestAPIPlotBinsParameter(t *testing.T) {
	handler := newTestHandler(t)

	rec := get(handler, http.MethodGet, "/api/plot?bins=7", testAddr)
	if rec.Code != http.StatusOK {
		t.Fatalf("bins=7: status = %d, want 200", rec.Code)
	}
	var body struct {
		Histogram struct {
			Counts []int `json:"counts"`
		} `json:"histogram"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if len(body.Histogram.Counts) != 7 {
		t.Errorf("bins=7: got %d classes, want 7", len(body.Histogram.Counts))
	}

	for _, raw := range []string{"0", "-3", "abc", "501"} {
		if rec := get(handler, http.MethodGet, "/api/plot?bins="+raw, testAddr); rec.Code != http.StatusBadRequest {
			t.Errorf("bins=%s: status = %d, want 400", raw, rec.Code)
		}
	}
}

func TestAPIPlotKDEParameter(t *testing.T) {
	handler := newTestHandler(t)

	var body struct {
		KDE struct {
			Density   []float64 `json:"density"`
			Bandwidth *float64  `json:"bandwidth"`
			Silverman *float64  `json:"silverman"`
		} `json:"kde"`
		Grid []float64 `json:"grid"`
	}

	rec := get(handler, http.MethodGet, "/api/plot?kde=2", testAddr)
	if rec.Code != http.StatusOK {
		t.Fatalf("kde=2: status = %d, want 200", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if len(body.KDE.Density) != len(body.Grid) {
		t.Errorf("kde density has %d values, grid has %d", len(body.KDE.Density), len(body.Grid))
	}
	if body.KDE.Bandwidth == nil || body.KDE.Silverman == nil || *body.KDE.Silverman <= 0 {
		t.Errorf("kde bandwidth/silverman = %v/%v, want positive values", body.KDE.Bandwidth, body.KDE.Silverman)
	} else if want := 2 * *body.KDE.Silverman; math.Abs(*body.KDE.Bandwidth-want) > 1e-9 {
		t.Errorf("bandwidth = %g, want 2 × silverman %g", *body.KDE.Bandwidth, want)
	}

	rec = get(handler, http.MethodGet, "/api/plot?kde=0.5", testAddr)
	if rec.Code != http.StatusOK {
		t.Fatalf("kde=0.5: status = %d, want 200", rec.Code)
	}

	for _, raw := range []string{"0", "-1", "abc", "21", "NaN", "Inf"} {
		if rec := get(handler, http.MethodGet, "/api/plot?kde="+raw, testAddr); rec.Code != http.StatusBadRequest {
			t.Errorf("kde=%s: status = %d, want 400", raw, rec.Code)
		}
	}
}

func get(handler http.Handler, method, path, host string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Host = host
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestAPIPlotReturnsPayloadForLocalHosts(t *testing.T) {
	handler := newTestHandler(t)

	for _, host := range []string{testAddr, "localhost:8080"} {
		rec := get(handler, http.MethodGet, "/api/plot", host)
		if rec.Code != http.StatusOK {
			t.Fatalf("host %s: status = %d, want 200", host, rec.Code)
		}

		var body struct {
			SampleSize int       `json:"sample_size"`
			Grid       []float64 `json:"grid"`
			Models     []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("host %s: response is not valid JSON: %v", host, err)
		}
		if body.SampleSize != 5 || len(body.Grid) == 0 || len(body.Models) != 1 || body.Models[0].Name != "Normal" {
			t.Errorf("host %s: unexpected payload %+v", host, body)
		}
	}
}

func TestForeignHostsAreRejected(t *testing.T) {
	handler := newTestHandler(t)

	for _, host := range []string{"evil.example:8080", "127.0.0.1:9999", ""} {
		rec := get(handler, http.MethodGet, "/api/plot", host)
		if rec.Code != http.StatusForbidden {
			t.Errorf("host %q: status = %d, want 403", host, rec.Code)
		}
	}
}

func TestDashboardAssetsAreServed(t *testing.T) {
	handler := newTestHandler(t)

	index := get(handler, http.MethodGet, "/", testAddr)
	if index.Code != http.StatusOK || !strings.Contains(index.Body.String(), `id="pdf-chart"`) {
		t.Errorf("index: status = %d, want 200 with dashboard markup", index.Code)
	}
	for _, path := range []string{"/app.js", "/style.css", "/uPlot.iife.min.js", "/uPlot.min.css"} {
		if rec := get(handler, http.MethodGet, path, testAddr); rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", path, rec.Code)
		}
	}
}

func TestPostIsNotAllowed(t *testing.T) {
	handler := newTestHandler(t)

	if rec := get(handler, http.MethodPost, "/api/plot", testAddr); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/plot: status = %d, want 405", rec.Code)
	}
}

func TestHandlerRejectsInvalidAddress(t *testing.T) {
	if _, err := Handler(func(int, float64) plot.Payload { return plot.Payload{} }, "no-port-here"); err == nil {
		t.Error("Handler() with address without port returned no error")
	}
}
