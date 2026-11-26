package cli

import (
	"fmt"
	"io"
)

// SilvermanReport contains the result for testing whether a density has more
// than the requested number of modes.
type SilvermanReport struct {
	Modes              int     `json:"modes"`
	CriticalBandwidth  float64 `json:"critical_bandwidth"`
	ReferenceBandwidth float64 `json:"reference_bandwidth"`
	BandwidthRatio     float64 `json:"bandwidth_ratio"`
	PValue             float64 `json:"p_value"`
	Simulations        int     `json:"simulations"`
	Seed               uint64  `json:"seed"`
}

func writeSilvermanSection(w io.Writer, result *SilvermanReport) {
	if result == nil {
		return
	}
	fmt.Fprintf(w, "Silverman critical-bandwidth test (H0: höchstens %d Modi)\n", result.Modes)
	fmt.Fprintln(w, "---------------------------------------------------------")
	fmt.Fprintf(w, "Kritische Bandbreite: %.6g\n", result.CriticalBandwidth)
	fmt.Fprintf(w, "Silverman-Referenz:   %.6g (Faktor %.4g)\n", result.ReferenceBandwidth, result.BandwidthRatio)
	fmt.Fprintf(w, "p-value:              %.4f (%d Simulationen, Seed %d)\n",
		result.PValue, result.Simulations, result.Seed)
	fmt.Fprintln(w)
}
