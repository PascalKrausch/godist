package cli

import (
	"fmt"
	"io"
)

// DipReport enthält das Ergebnis des Hartigan-Dip-Tests auf die
// Eingabestichprobe. Der Dip-Test prüft, ob die Daten aus einer
// unimodalen Verteilung stammen können (Nullhypothese: Unimodalität).
type DipReport struct {
	Dip         float64 `json:"dip"`
	PValue      float64 `json:"p_value"`
	Simulations int     `json:"simulations"`
	Seed        uint64  `json:"seed"`
}

// writeDipSection gibt das Dip-Test-Ergebnis als Block aus.
// dip == nil (Flag nicht gesetzt) lässt den Bericht unverändert.
func writeDipSection(w io.Writer, dip *DipReport) {
	if dip == nil {
		return
	}
	fmt.Fprintln(w, "Dip test (Hartigan)")
	fmt.Fprintln(w, "-------------------")
	fmt.Fprintf(w, "Dip:         %.4f\n", dip.Dip)
	fmt.Fprintf(w, "p-value:     %.4f (%d Simulationen, Seed %d)\n", dip.PValue, dip.Simulations, dip.Seed)
	fmt.Fprintln(w)
}
