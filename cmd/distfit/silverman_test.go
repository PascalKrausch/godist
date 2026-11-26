package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunSilvermanTextAndJSON(t *testing.T) {
	path := writeFile(t, "sample.txt", "-3.2\n-3.1\n-3.0\n-2.9\n-2.8\n2.8\n2.9\n3.0\n3.1\n3.2\n")
	code, stdout, stderr := runCLI(t, "fit", path, "-silverman", "-silverman-simulations=12", "-silverman-seed=7")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	for _, want := range []string{"Silverman critical-bandwidth test", "Kritische Bandbreite:", "p-value:", "Seed 7"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout enthält %q nicht:\n%s", want, stdout)
		}
	}

	code, stdout, stderr = runCLI(t, "fit", path, "-all", "-format=json", "-silverman", "-silverman-modes=1", "-silverman-simulations=12", "-silverman-seed=7")
	if code != 0 {
		t.Fatalf("JSON exit = %d, stderr = %s", code, stderr)
	}
	var report struct {
		Silverman *struct {
			Modes              int     `json:"modes"`
			CriticalBandwidth  float64 `json:"critical_bandwidth"`
			ReferenceBandwidth float64 `json:"reference_bandwidth"`
			BandwidthRatio     float64 `json:"bandwidth_ratio"`
			PValue             float64 `json:"p_value"`
			Simulations        int     `json:"simulations"`
			Seed               uint64  `json:"seed"`
		} `json:"silverman_test"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if report.Silverman == nil || report.Silverman.Modes != 1 || report.Silverman.Simulations != 12 || report.Silverman.Seed != 7 {
		t.Errorf("silverman_test = %+v, want modes=1 simulations=12 seed=7", report.Silverman)
	}
	if report.Silverman.PValue < 0 || report.Silverman.PValue > 1 {
		t.Errorf("p-value out of range: %g", report.Silverman.PValue)
	}
}

func TestSilvermanFlagValidation(t *testing.T) {
	path := writeFile(t, "sample.txt", "1\n2\n3\n4\n5\n")
	tests := []struct {
		name string
		args []string
	}{
		{"modes without test", []string{"-silverman-modes=2"}},
		{"simulations without test", []string{"-silverman-simulations=10"}},
		{"seed without test", []string{"-silverman-seed=2"}},
		{"zero modes", []string{"-silverman", "-silverman-modes=0"}},
		{"zero simulations", []string{"-silverman", "-silverman-simulations=0"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"fit", path}, tc.args...)
			if code, _, _ := runCLI(t, args...); code != exitUsage {
				t.Fatalf("exit = %d, want %d", code, exitUsage)
			}
		})
	}
}

func TestSilvermanOmittedWhenDisabled(t *testing.T) {
	path := writeFile(t, "sample.txt", "1\n2\n3\n4\n5\n")
	code, stdout, stderr := runCLI(t, "fit", path, "-all", "-format=json")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	if strings.Contains(stdout, "silverman_test") {
		t.Errorf("JSON enthält silverman_test ohne -silverman:\n%s", stdout)
	}
}
