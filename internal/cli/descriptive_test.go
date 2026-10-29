package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/PascalKrausch/godist/internal/stats"
)

func TestWriteTextReportShowsDescriptiveStats(t *testing.T) {
	report := BuildFitReport(nil, nil, "data.csv", "x", stats.Describe([]float64{4, 1, 3, 2}), 0)

	var buf bytes.Buffer
	if err := WriteTextReport(&buf, report); err != nil {
		t.Fatalf("WriteTextReport: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Descriptive statistics") {
		t.Fatalf("report missing descriptive block:\n%s", out)
	}
	if !strings.Contains(out, "2.5000") {
		t.Errorf("report should contain median 2.5000:\n%s", out)
	}
}

func TestWriteDescriptiveShowsDashForUndefinedValues(t *testing.T) {
	var buf bytes.Buffer
	writeDescriptive(&buf, stats.Describe([]float64{1, 2}))
	out := buf.String()
	if !strings.Contains(out, "–") {
		t.Errorf("skewness/kurtosis for n=2 should render as '–':\n%s", out)
	}
}
