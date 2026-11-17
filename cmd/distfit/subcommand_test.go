package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSampleFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "werte.txt")
	data := "1.2\n2.3\n1.9\n3.1\n2.7\n1.5\n2.2\n3.8\n2.9\n1.7\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFitSubcommandAcceptsPositionalFileAndInterspersedFlags(t *testing.T) {
	path := writeSampleFile(t)

	code, stdout, stderr := runCLI(t, "fit", path, "-format=json", "-all")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr)
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("stdout ist kein JSON: %v", err)
	}
}

func TestFitSubcommandWithFlagFileMatchesLegacy(t *testing.T) {
	path := writeSampleFile(t)

	codeLegacy, outLegacy, _ := runCLI(t, "-file="+path)
	codeFit, outFit, _ := runCLI(t, "fit", "-file="+path)
	if codeLegacy != 0 || codeFit != 0 {
		t.Fatalf("exit codes = %d, %d; want 0, 0", codeLegacy, codeFit)
	}
	// Die Runtime-Zeile ist laufenzeitabhängig und wird beim Vergleich ignoriert.
	stripRuntime := func(s string) string {
		lines := strings.Split(s, "\n")
		kept := lines[:0]
		for _, l := range lines {
			if !strings.HasPrefix(l, "Runtime:") {
				kept = append(kept, l)
			}
		}
		return strings.Join(kept, "\n")
	}
	if stripRuntime(outLegacy) != stripRuntime(outFit) {
		t.Error("fit -file=... weicht von der Legacy-Ausgabe ab")
	}
}

func TestFitRejectsPositionalAndFlagFile(t *testing.T) {
	path := writeSampleFile(t)

	code, _, stderr := runCLI(t, "fit", path, "-file="+path)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "nicht beides") {
		t.Errorf("stderr = %q, want hint about both forms", stderr)
	}
}

func TestLegacyRejectsPositionalArgument(t *testing.T) {
	path := writeSampleFile(t)

	code, _, stderr := runCLI(t, "-file="+path, path)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "unerwartete Argumente") {
		t.Errorf("stderr = %q, want unerwartete Argumente", stderr)
	}
}

func TestFitRejectsMultiplePositionals(t *testing.T) {
	path := writeSampleFile(t)

	code, _, _ := runCLI(t, "fit", path, path)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestParseArgsDoubleDashEndsFlags(t *testing.T) {
	code, _, _ := runCLI(t, "fit", "--", "-file-that-does-not-exist.txt")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (Datei nicht lesbar, nicht als Flag interpretiert)", code)
	}
}

func TestListDistributionsText(t *testing.T) {
	code, stdout, stderr := runCLI(t, "list", "distributions")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Weibull") || !strings.Contains(stdout, "Fit-Hinweise") {
		t.Errorf("unerwartete Textausgabe: %q", stdout)
	}
}

func TestListDistributionsJSON(t *testing.T) {
	code, stdout, _ := runCLI(t, "list", "distributions", "-format=json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var entries []map[string]any
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatalf("stdout ist kein JSON-Array: %v", err)
	}
	if len(entries) == 0 {
		t.Error("JSON-Array ist leer")
	}
}

func TestListRejectsBadInput(t *testing.T) {
	cases := map[string][]string{
		"unknown subcommand": {"list"},
		"wrong target":       {"list", "models"},
		"invalid format":     {"list", "distributions", "-format=xml"},
		"extra positional":   {"list", "distributions", "weibull"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if code, _, _ := runCLI(t, args...); code != 2 {
				t.Fatalf("exit code = %d, want 2", code)
			}
		})
	}
}
