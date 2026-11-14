package distribution

import (
	"strings"
	"testing"
)

func fitterNames(fitters []Fitter) []string {
	names := make([]string, 0, len(fitters))
	for _, f := range fitters {
		names = append(names, f.Name())
	}
	return names
}

func TestSelectFittersIncludeKeepsOrderAndDeduplicates(t *testing.T) {
	fitters, err := SelectFitters(PolicyFastFail, []string{"gamma", "weibull", "GAMMA"}, nil)
	if err != nil {
		t.Fatalf("SelectFitters: %v", err)
	}
	got := strings.Join(fitterNames(fitters), ",")
	if got != "Gamma,Weibull" {
		t.Errorf("names = %q, want %q", got, "Gamma,Weibull")
	}
}

func TestSelectFittersExcludeRemovesFromCatalog(t *testing.T) {
	fitters, err := SelectFitters(PolicyFastFail, nil, []string{"Cauchy"})
	if err != nil {
		t.Fatalf("SelectFitters: %v", err)
	}
	if len(fitters) != len(Catalog())-1 {
		t.Fatalf("got %d fitters, want %d", len(fitters), len(Catalog())-1)
	}
	for _, name := range fitterNames(fitters) {
		if name == "Cauchy" {
			t.Error("Cauchy wurde nicht ausgeschlossen")
		}
	}
}

func TestSelectFittersZeroInflatedWrapsOnlyCapableDistributions(t *testing.T) {
	fitters, err := SelectFitters(PolicyZeroInflated, []string{"weibull"}, nil)
	if err != nil {
		t.Fatalf("SelectFitters: %v", err)
	}
	if _, ok := fitters[0].(ZeroInflatedFitter); !ok {
		t.Fatalf("fitter type = %T, want ZeroInflatedFitter", fitters[0])
	}

	if _, err := SelectFitters(PolicyZeroInflated, []string{"normal"}, nil); err == nil {
		t.Error("Normal mit ZI-Policy: want error, got nil")
	}
}

func TestSelectFittersRejectsInvalidSelections(t *testing.T) {
	cases := []struct {
		name    string
		include []string
		exclude []string
		wantSub string
	}{
		{"unknown include", []string{"wibull"}, nil, "unbekannte Verteilung"},
		{"unknown exclude", nil, []string{"wibull"}, "unbekannte Verteilung"},
		{"include and exclude", []string{"weibull"}, []string{"gamma"}, "schließen sich aus"},
		{"everything excluded", nil, []string{"Weibull", "LogNormal", "LogLogistic", "BurrXII", "Gamma", "Normal", "Beta", "Exponential", "Gumbel", "Rayleigh", "Laplace", "Logistic", "Pareto", "ChiSquare", "Cauchy", "StudentT"}, "keine Verteilung übrig"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SelectFitters(PolicyFastFail, tc.include, tc.exclude)
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantSub)
			}
		})
	}
}
