package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const wineCSV = "../../testdata/winequality-red.csv"

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunExitCodes(t *testing.T) {
	txt := writeFile(t, "d.txt", "1.5\n2.5\n3.1\n4.2\n2.2\n")
	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{"help", []string{"-h"}, 0},
		{"single model ok", []string{"-file=" + txt}, 0},
		{"missing file flag", []string{}, 2},
		{"unknown flag", []string{"-nope"}, 2},
		{"positional argument", []string{"-file=" + txt, "extra"}, 2},
		{"invalid format", []string{"-file=" + txt, "-format=xml"}, 2},
		{"invalid decimal", []string{"-file=" + txt, "-decimal=;"}, 2},
		{"json without all", []string{"-file=" + txt, "-format=json"}, 2},
		{"column on txt", []string{"-file=" + txt, "-column=a"}, 2},
		{"unsupported extension", []string{"-file=d.xlsx"}, 2},
		{"ambiguous csv without column", []string{"-file=" + wineCSV}, 1},
		{"missing input file", []string{"-file=does-not-exist.txt"}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, _ := runCLI(t, tt.args...)
			if code != tt.wantCode {
				t.Fatalf("exit = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

func TestRunSingleModeSummaryHasNoGoTypeNames(t *testing.T) {
	code, stdout, _ := runCLI(t, "-file="+wineCSV, "-column=density")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.Contains(stdout, "distribution.") || strings.Contains(stdout, "%!") {
		t.Errorf("summary contains Go type info:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Spalte:         density") || !strings.Contains(stdout, "Datenpunkte:    1599") {
		t.Errorf("summary missing column or sample size:\n%s", stdout)
	}
}

func TestRunJSONReportContainsDiscardedCount(t *testing.T) {
	path := writeFile(t, "d.txt", "1.5\nabc\n2.5\n3.1\n4.2\n2.2\n")
	code, stdout, stderr := runCLI(t, "-file="+path, "-all", "-format=json")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	var report struct {
		SampleSize int `json:"sample_size"`
		Discarded  int `json:"discarded"`
		Models     []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout)
	}
	if report.SampleSize != 5 || report.Discarded != 1 {
		t.Errorf("sample_size=%d discarded=%d, want 5 and 1", report.SampleSize, report.Discarded)
	}
	if len(report.Models) == 0 {
		t.Error("expected models in JSON output")
	}
	if !strings.Contains(stderr, "Warnung: 1 von 6 Werten verworfen") {
		t.Errorf("stderr missing warning, got: %q", stderr)
	}
}

func TestRunSampleSizePolicy(t *testing.T) {
	small := writeFile(t, "small.txt", "1.5\n2.5\n3.1\n4.2\n")
	constant := writeFile(t, "const.txt", "5\n5\n5\n5\n5\n5\n")
	mid := writeFile(t, "mid.txt", "1.5\n2.5\n3.1\n4.2\n2.2\n3.3\n2.8\n")

	code, _, stderr := runCLI(t, "-file="+small)
	if code != 1 || !strings.Contains(stderr, "zu wenige Werte") {
		t.Errorf("n=4: exit=%d stderr=%q, want exit 1 with error", code, stderr)
	}

	code, _, stderr = runCLI(t, "-file="+constant)
	if code != 1 || !strings.Contains(stderr, "identisch") {
		t.Errorf("constant: exit=%d stderr=%q, want exit 1 with error", code, stderr)
	}

	code, stdout, stderr := runCLI(t, "-file="+mid)
	if code != 0 {
		t.Fatalf("n=7: exit=%d, stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "hohe Parameterunsicherheit") {
		t.Errorf("n=7: missing uncertainty warning, stderr=%q", stderr)
	}
	if !strings.Contains(stdout, "Datenpunkte:    7") {
		t.Errorf("n=7: stdout missing sample size:\n%s", stdout)
	}
}

func TestRunWarningsGoToStderrNotStdout(t *testing.T) {
	path := writeFile(t, "d.txt", "1.5\nabc\n2.5\n3.1\n4.2\n2.2\n")
	code, stdout, stderr := runCLI(t, "-file="+path, "-all", "-format=json")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if strings.Contains(stdout, "Warnung") {
		t.Error("warning leaked into stdout, which must stay parseable JSON")
	}
	if !strings.Contains(stderr, "Warnung") {
		t.Error("warning missing from stderr")
	}
}
