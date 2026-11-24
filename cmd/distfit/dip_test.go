package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunDipTextOutput(t *testing.T) {
	path := writeSampleFile(t)

	code, stdout, stderr := runCLI(t, "fit", path, "-dip", "-dip-seed=7")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	for _, want := range []string{"Dip test (Hartigan)", "Dip:", "p-value:", "Seed 7"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout enthält %q nicht:\n%s", want, stdout)
		}
	}
}

func TestRunDipJSONOutput(t *testing.T) {
	path := writeSampleFile(t)

	code, stdout, stderr := runCLI(t, "fit", path, "-all", "-format=json", "-dip", "-dip-seed=3")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	var report struct {
		DipTest *struct {
			Dip         float64 `json:"dip"`
			PValue      float64 `json:"p_value"`
			Simulations int     `json:"simulations"`
			Seed        uint64  `json:"seed"`
		} `json:"dip_test"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("stdout ist kein JSON: %v", err)
	}
	if report.DipTest == nil {
		t.Fatalf("dip_test fehlt in JSON:\n%s", stdout)
	}
	if report.DipTest.Seed != 3 || report.DipTest.Simulations != 2000 {
		t.Errorf("dip_test = %+v, want seed=3 simulations=2000", report.DipTest)
	}
	if report.DipTest.Dip < 0 || report.DipTest.Dip > 0.25 || report.DipTest.PValue < 0 || report.DipTest.PValue > 1 {
		t.Errorf("dip_test Werte außerhalb des Definitionsbereichs: %+v", report.DipTest)
	}
}

func TestRunWithoutDipHasNoDipSection(t *testing.T) {
	path := writeSampleFile(t)

	code, stdout, _ := runCLI(t, "fit", path)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if strings.Contains(stdout, "Dip test") {
		t.Errorf("ohne -dip darf kein Dip-Abschnitt erscheinen:\n%s", stdout)
	}

	code, stdout, _ = runCLI(t, "fit", path, "-all", "-format=json")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if strings.Contains(stdout, "dip_test") {
		t.Errorf("ohne -dip darf dip_test nicht im JSON stehen:\n%s", stdout)
	}
}

func TestDipFlagValidation(t *testing.T) {
	path := writeSampleFile(t)
	tests := []struct {
		name string
		args []string
	}{
		{"seed ohne dip", []string{"fit", path, "-dip-seed=5"}},
		{"simulations ohne dip", []string{"fit", path, "-dip-simulations=100"}},
		{"simulations null", []string{"fit", path, "-dip", "-dip-simulations=0"}},
		{"simulations negativ", []string{"fit", path, "-dip", "-dip-simulations=-3"}},
		{"dip mit plot", []string{"fit", path, "-all", "-format=plot", "-dip"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code, _, _ := runCLI(t, tt.args...); code != 2 {
				t.Fatalf("exit = %d, want 2", code)
			}
		})
	}
}
