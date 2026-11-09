package distribution

import (
	"math"
	"math/rand"
	"testing"
)

func TestCatalogNamesAreUniqueAndMatchFitters(t *testing.T) {
	seen := map[string]bool{}
	for _, spec := range Catalog() {
		if seen[spec.Name] {
			t.Errorf("Name %q doppelt im Katalog", spec.Name)
		}
		seen[spec.Name] = true
		if spec.Fitter == nil {
			t.Errorf("%s: kein Fitter", spec.Name)
			continue
		}
		if got := spec.Fitter.Name(); got != spec.Name {
			t.Errorf("Katalogname %q passt nicht zu Fitter.Name() %q", spec.Name, got)
		}
	}
}

func TestLookupIsCaseInsensitive(t *testing.T) {
	spec, ok := Lookup("weibull")
	if !ok || spec.Name != "Weibull" {
		t.Fatalf("Lookup(weibull) = %q, %v; want Weibull, true", spec.Name, ok)
	}
	if _, ok := Lookup("unbekannt"); ok {
		t.Error("Lookup(unbekannt) meldet Treffer")
	}
}

// Parameternamen werden am gefitteten Modell geprüft. Jede Verteilung muss auf mindestens
// einem der beiden Datensätze erfolgreich fitten, damit ihr Eintrag überprüft werden kann.
func TestCatalogParamNamesMatchFittedModels(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	positive := make([]float64, 200)
	symmetric := make([]float64, 200)
	for i := range positive {
		positive[i] = 2 * math.Exp(rng.NormFloat64())
		symmetric[i] = 3*rng.NormFloat64() + 1
	}
	datasets := [][]float64{positive, symmetric}

	for _, spec := range Catalog() {
		checked := false
		for _, data := range datasets {
			dist, err := spec.Fitter.Fit(data)
			if err != nil {
				continue
			}
			checked = true
			names := dist.ParamNames()
			if len(names) != len(spec.Params) {
				t.Errorf("%s: Params %v, Modell liefert %v", spec.Name, spec.Params, names)
				continue
			}
			for i := range names {
				if names[i] != spec.Params[i] {
					t.Errorf("%s: Params %v, Modell liefert %v", spec.Name, spec.Params, names)
					break
				}
			}
		}
		if !checked {
			t.Errorf("%s: auf keinem Testdatensatz gefittet, Parameter nicht prüfbar", spec.Name)
		}
	}
}

func TestCandidateFittersFollowCatalog(t *testing.T) {
	var wantStandard, wantZI []string
	for _, spec := range Catalog() {
		wantStandard = append(wantStandard, spec.Name)
		if spec.ZeroInflatable {
			wantZI = append(wantZI, "ZI-"+spec.Name)
		}
	}
	assertNames(t, "standard", candidateFitters(PolicyFastFail), wantStandard)
	assertNames(t, "zero-inflated", candidateFitters(PolicyZeroInflated), wantZI)
}

func assertNames(t *testing.T, label string, fitters []Fitter, want []string) {
	t.Helper()
	if len(fitters) != len(want) {
		t.Fatalf("%s: %d Kandidaten, Katalog erwartet %d", label, len(fitters), len(want))
	}
	for i, f := range fitters {
		if f.Name() != want[i] {
			t.Errorf("%s[%d] = %q, want %q", label, i, f.Name(), want[i])
		}
	}
}
