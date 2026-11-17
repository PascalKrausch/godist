package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// Bootstrap ohne -all läuft nur für das beste Modell und erscheint in der Textausgabe.
func TestRunBootstrapSingleModelShowsCIs(t *testing.T) {
	code, stdout, stderr := runCLI(t,
		"-file="+wineCSV, "-column=density",
		"-include=weibull,gamma", "-bootstrap=30", "-bootstrap-seed=1")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	if !strings.Contains(stdout, "Bootstrap (") {
		t.Errorf("stdout missing bootstrap section:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Bootstrap: 30 Wiederholungen") {
		t.Errorf("stderr missing bootstrap status line: %q", stderr)
	}
}

// Mit -all -format=json liegt das Bootstrap-Objekt pro Modell im JSON-Bericht.
func TestRunBootstrapJSONReport(t *testing.T) {
	code, stdout, stderr := runCLI(t,
		"-file="+wineCSV, "-column=density",
		"-include=weibull,gamma", "-all", "-format=json",
		"-bootstrap=20", "-bootstrap-model=all", "-bootstrap-method=parametric",
		"-bootstrap-ci=0.9", "-bootstrap-seed=42")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	var report struct {
		Models []struct {
			Name      string `json:"name"`
			Bootstrap *struct {
				Method     string  `json:"method"`
				Requested  int     `json:"requested"`
				Replicates int     `json:"replicates"`
				Failed     int     `json:"failed"`
				Level      float64 `json:"level"`
				Seed       uint64  `json:"seed"`
				Params     []struct {
					Name string  `json:"name"`
					Mean float64 `json:"mean"`
					Lo   float64 `json:"lo"`
					Hi   float64 `json:"hi"`
				} `json:"params"`
				Metrics map[string]struct {
					Mean float64 `json:"mean"`
					Lo   float64 `json:"lo"`
					Hi   float64 `json:"hi"`
				} `json:"metrics"`
			} `json:"bootstrap"`
		} `json:"models"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	if len(report.Models) != 2 {
		t.Fatalf("models = %d, want 2", len(report.Models))
	}
	for _, m := range report.Models {
		b := m.Bootstrap
		if b == nil {
			t.Errorf("model %s: bootstrap missing", m.Name)
			continue
		}
		if b.Method != "parametric" || b.Requested != 20 || b.Level != 0.9 || b.Seed != 42 {
			t.Errorf("model %s: unexpected bootstrap metadata: %+v", m.Name, b)
		}
		if b.Replicates < 1 || len(b.Params) == 0 || len(b.Metrics) == 0 {
			t.Errorf("model %s: empty bootstrap result: %+v", m.Name, b)
		}
	}
}

// Gleicher Seed muss bei -bootstrap-model=all identische Intervalle für beide Modelle liefern.
func TestRunBootstrapSeedReproducible(t *testing.T) {
	args := []string{
		"-file=" + wineCSV, "-column=density",
		"-include=weibull,gamma", "-all", "-format=json",
		"-bootstrap=20", "-bootstrap-model=all", "-bootstrap-seed=7",
	}
	_, out1, _ := runCLI(t, args...)
	_, out2, _ := runCLI(t, args...)
	// duration_ms-Felder sind Laufzeitabhängig; nur die Bootstrap-Intervalle vergleichen.
	boots := func(out string) string {
		var report struct {
			Models []struct {
				Name      string          `json:"name"`
				Bootstrap json.RawMessage `json:"bootstrap"`
			} `json:"models"`
		}
		if err := json.Unmarshal([]byte(out), &report); err != nil {
			t.Fatalf("stdout is not valid JSON: %v", err)
		}
		var b strings.Builder
		for _, m := range report.Models {
			var boot map[string]any
			if err := json.Unmarshal(m.Bootstrap, &boot); err != nil {
				t.Fatalf("bootstrap ist kein Objekt: %v", err)
			}
			delete(boot, "duration_ms")
			canon, _ := json.Marshal(boot)
			b.WriteString(m.Name)
			b.Write(canon)
		}
		return b.String()
	}
	if boots(out1) != boots(out2) {
		t.Error("gleicher Seed lieferte unterschiedliche Bootstrap-Intervalle")
	}
}

// -bootstrap-model=best (Standard) bootstrapt nur das AIC-beste Modell.
func TestRunBootstrapBestOnlyWithAll(t *testing.T) {
	code, stdout, stderr := runCLI(t,
		"-file="+wineCSV, "-column=density",
		"-include=weibull,gamma", "-all", "-format=json",
		"-bootstrap=10", "-bootstrap-seed=3")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	var report struct {
		Models []struct {
			Bootstrap *json.RawMessage `json:"bootstrap"`
		} `json:"models"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	bootstrapped := 0
	for _, m := range report.Models {
		if m.Bootstrap != nil {
			bootstrapped++
		}
	}
	if bootstrapped != 1 {
		t.Errorf("bootstrap ran for %d models, want 1 (best only)", bootstrapped)
	}
}

func TestRunBootstrapFlagValidation(t *testing.T) {
	path := writeFile(t, "d.txt", "1.5\n2.5\n3.1\n4.2\n2.2\n3.3\n2.8\n")
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"negative B", []string{"-file=" + path, "-bootstrap=-1"}, "-bootstrap"},
		{"bad method", []string{"-file=" + path, "-bootstrap=5", "-bootstrap-method=bayes"}, "bootstrap-method"},
		{"bad ci", []string{"-file=" + path, "-bootstrap=5", "-bootstrap-ci=1.5"}, "bootstrap-ci"},
		{"bad model", []string{"-file=" + path, "-bootstrap=5", "-bootstrap-model=some"}, "bootstrap-model"},
		{"method without B", []string{"-file=" + path, "-bootstrap-method=parametric"}, "erfordert -bootstrap"},
		{"seed without B", []string{"-file=" + path, "-bootstrap-seed=9"}, "erfordert -bootstrap"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, stderr := runCLI(t, tt.args...)
			if code != 2 {
				t.Fatalf("exit = %d, want 2; stderr = %q", code, stderr)
			}
			if !strings.Contains(stderr, tt.want) {
				t.Errorf("stderr = %q, want mention of %q", stderr, tt.want)
			}
		})
	}
}
