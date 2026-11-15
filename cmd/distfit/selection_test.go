package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDistributionFlagFitsSingleModel(t *testing.T) {
	path := writeSampleFile(t)

	code, stdout, stderr := runCLI(t, "fit", path, "-distribution=gamma")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Gamma") {
		t.Errorf("stdout lacks Gamma: %q", stdout)
	}
}

func TestIncludeLimitsAllModelsJSON(t *testing.T) {
	path := writeSampleFile(t)

	code, stdout, stderr := runCLI(t, "fit", path, "-all", "-format=json", "-include=weibull,gamma")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr)
	}
	var report struct {
		Candidates int `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("stdout ist kein JSON: %v", err)
	}
	if report.Candidates != 2 {
		t.Errorf("candidates = %d, want 2", report.Candidates)
	}
}

func TestSelectionFlagErrorsExitTwo(t *testing.T) {
	path := writeSampleFile(t)
	cases := map[string][]string{
		"unknown distribution":       {"fit", path, "-distribution=wibull"},
		"distribution with include":  {"fit", path, "-distribution=gamma", "-include=weibull"},
		"include with exclude":       {"fit", path, "-all", "-include=weibull", "-exclude=gamma"},
		"zero-inflated not capable":  {"fit", path, "-distribution=normal", "-zero-inflated"},
		"exclude removes everything": {"fit", path, "-all", "-exclude=Weibull,LogNormal,LogLogistic,BurrXII,Gamma,Normal,Beta,Exponential,Gumbel,Rayleigh,Laplace,Logistic,Pareto,ChiSquare,Cauchy,StudentT,SkewNormal,InverseGaussian"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if code, _, stderr := runCLI(t, args...); code != 2 {
				t.Fatalf("exit code = %d, want 2; stderr: %s", code, stderr)
			}
		})
	}
}
