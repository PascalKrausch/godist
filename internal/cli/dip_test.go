package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/PascalKrausch/godist/internal/stats"
	"github.com/PascalKrausch/godist/pkg/distribution"
)

// Mit DipReport muss der Textbericht den Dip-Abschnitt enthalten,
// ohne ihn nicht. JSON muss dip_test nur bei gesetztem Report tragen.
func TestDipReportRendering(t *testing.T) {
	results := []distribution.FitResult{
		{Name: "Normal", Dist: distribution.Normal{Mu: 0, Sigma: 1}, Metrics: distribution.FitMetrics{AIC: 10, BIC: 11}},
	}
	dip := &DipReport{Dip: 0.0312, PValue: 0.412, Simulations: 2000, Seed: 1}

	report := BuildFitReport(results, nil, "test.txt", "", stats.Summary{N: 10}, 0)
	report.Dip = dip

	var buf bytes.Buffer
	if err := WriteTextReport(&buf, report); err != nil {
		t.Fatalf("WriteTextReport: %v", err)
	}
	for _, want := range []string{"Dip test (Hartigan)", "Dip:         0.0312", "p-value:     0.4120", "2000 Simulationen", "Seed 1"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("Textbericht enthält %q nicht:\n%s", want, buf.String())
		}
	}

	buf.Reset()
	if err := WriteJSONReport(&buf, report); err != nil {
		t.Fatalf("WriteJSONReport: %v", err)
	}
	var decoded struct {
		DipTest *DipReport `json:"dip_test"`
	}
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("JSON unlesbar: %v", err)
	}
	if decoded.DipTest == nil || *decoded.DipTest != *dip {
		t.Errorf("dip_test = %+v, want %+v", decoded.DipTest, dip)
	}

	// Ohne Dip bleibt der Bericht unverändert.
	report.Dip = nil
	buf.Reset()
	if err := WriteTextReport(&buf, report); err != nil {
		t.Fatalf("WriteTextReport: %v", err)
	}
	if strings.Contains(buf.String(), "Dip test") {
		t.Error("Textbericht enthält Dip-Abschnitt ohne DipReport")
	}
	buf.Reset()
	if err := WriteJSONReport(&buf, report); err != nil {
		t.Fatalf("WriteJSONReport: %v", err)
	}
	if strings.Contains(buf.String(), "dip_test") {
		t.Error("JSON enthält dip_test ohne DipReport")
	}

	// WriteSummary zeigt den Dip-Abschnitt ebenfalls.
	buf.Reset()
	WriteSummary(&buf, distribution.Normal{Mu: 0, Sigma: 1}, distribution.FitMetrics{AIC: 10, BIC: 11}, "", stats.Summary{N: 10}, 0, nil, dip, nil)
	if !strings.Contains(buf.String(), "Dip test (Hartigan)") {
		t.Errorf("Zusammenfassung enthält keinen Dip-Abschnitt:\n%s", buf.String())
	}
}
