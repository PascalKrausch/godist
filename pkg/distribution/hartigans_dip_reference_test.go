package distribution

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// dipFixture ist ein Referenzfall aus evaluation/diptest_fixtures.json,
// erzeugt von evaluation/diptest_reference.py mit dem diptest-Paket
// (Python-Port der R-Referenzimplementierung, AS 217).
type dipFixture struct {
	Data   []float64 `json:"data"`
	Dip    float64   `json:"dip"`
	PValue float64   `json:"pvalue"`
	DipTol float64   `json:"dip_tol"` // optional: gelockerte Toleranz für dokumentierte Abweichungen
}

// TestDipStatisticAgainstReference vergleicht DipStatistic und den p-Wert
// von DipTest gegen Referenzwerte der etablierten diptest-Implementierung.
func TestDipStatisticAgainstReference(t *testing.T) {
	path := filepath.Join("..", "..", "evaluation", "diptest_fixtures.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Referenz-Fixtures nicht lesbar (%s): %v", path, err)
	}

	var fixtures map[string]dipFixture
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatalf("Referenz-Fixtures ungültig: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("keine Referenz-Fixtures gefunden")
	}

	const (
		dipTol  = 1e-9 // Dip-Statistik ist deterministisch
		pvalTol = 0.1  // beide Seiten Monte-Carlo: Referenz n_boot=10000, hier sims=2000
		sims    = 2000
		simSeed = 99
	)

	for name, fx := range fixtures {
		t.Run(name, func(t *testing.T) {
			tol := fx.DipTol
			if tol == 0 {
				tol = dipTol
			}

			got := DipStatistic(fx.Data)
			if math.Abs(got-fx.Dip) > tol {
				t.Errorf("DipStatistic = %.12g, Referenz = %.12g (Tol %v)", got, fx.Dip, tol)
			}

			res := DipTest(fx.Data, sims, simSeed)
			if math.Abs(res.Dip-fx.Dip) > tol {
				t.Errorf("DipTest.Dip = %.12g, Referenz = %.12g (Tol %v)", res.Dip, fx.Dip, tol)
			}
			if math.IsNaN(fx.PValue) || math.IsInf(fx.PValue, 0) {
				return
			}
			if math.Abs(res.PValue-fx.PValue) > pvalTol {
				t.Errorf("DipTest.PValue = %.4f, Referenz = %.4f (Tol %v)", res.PValue, fx.PValue, pvalTol)
			}
		})
	}
}
