package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/PascalKrausch/godist/internal/stats"
	"github.com/PascalKrausch/godist/pkg/distribution"
)

func TestGetDistributionNameZeroInflated(t *testing.T) {
	dist := distribution.ZeroInflated{
		Base:  distribution.Gamma{Alpha: 2, Beta: 1},
		PZero: 0.3,
	}
	if got := getDistributionName(dist); got != "ZI-Gamma" {
		t.Fatalf("getDistributionName = %q, want %q", got, "ZI-Gamma")
	}
}

// Anforderung: Ist GEV unter den angepassten Modellen, muss der Bericht auf die
// fachliche Einschränkung der Extremwertverteilung hinweisen.
func TestWriteTextReportWarnsForGEV(t *testing.T) {
	gev := distribution.GEV{Mu: 1, Sigma: 2, Xi: 0.2}
	report := BuildFitReport(
		[]distribution.FitResult{
			{Name: "GEV", Dist: gev, Metrics: distribution.FitMetrics{AIC: 10, BIC: 11}},
			{Name: "Normal", Dist: distribution.Normal{Mu: 0, Sigma: 1}, Metrics: distribution.FitMetrics{AIC: 20, BIC: 21}},
		},
		nil, "test.txt", "", stats.Summary{N: 10}, 0,
	)

	var buf bytes.Buffer
	if err := WriteTextReport(&buf, report); err != nil {
		t.Fatalf("WriteTextReport: %v", err)
	}
	if !strings.Contains(buf.String(), "Extremwertverteilung") {
		t.Error("Textbericht enthält keinen GEV-Hinweis")
	}

	buf.Reset()
	WriteSummary(&buf, gev, distribution.FitMetrics{AIC: 10, BIC: 11}, "", stats.Summary{N: 10}, 0)
	if !strings.Contains(buf.String(), "Extremwertverteilung") {
		t.Error("Zusammenfassung enthält keinen GEV-Hinweis")
	}
}

// Ohne GEV darf kein Hinweis erscheinen.
func TestWriteTextReportNoGEVNote(t *testing.T) {
	report := BuildFitReport(
		[]distribution.FitResult{
			{Name: "Normal", Dist: distribution.Normal{Mu: 0, Sigma: 1}, Metrics: distribution.FitMetrics{AIC: 10, BIC: 11}},
		},
		nil, "test.txt", "", stats.Summary{N: 10}, 0,
	)

	var buf bytes.Buffer
	if err := WriteTextReport(&buf, report); err != nil {
		t.Fatalf("WriteTextReport: %v", err)
	}
	if strings.Contains(buf.String(), "Extremwertverteilung") {
		t.Error("Textbericht enthält GEV-Hinweis ohne GEV-Modell")
	}
}
