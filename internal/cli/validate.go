package cli

import (
	"fmt"
)

const (
	// MinSampleSize ist die kleinste Stichprobe, für die ein Fit durchgeführt wird.
	MinSampleSize = 5
	// HighUncertaintySampleSize ist die größte Stichprobe, für die eine Warnung zur Parameterunsicherheit ausgegeben wird.
	HighUncertaintySampleSize = 20
)

// ValidateSample prüft, ob ein Datensatz gefittet werden darf.
// Zu kleine oder konstante Stichproben werden abgelehnt (Fast-Fail).
// Für 5 bis 20 Werte liefert die Funktion eine Warnung zur Parameterunsicherheit.
func ValidateSample(values []float64) (warning string, err error) {
	n := len(values)
	if n < MinSampleSize {
		return "", fmt.Errorf("zu wenige Werte: n=%d, mindestens %d erforderlich", n, MinSampleSize)
	}

	if isConstant(values) {
		return "", fmt.Errorf("alle %d Werte sind identisch; konstante Daten können nicht angepasst werden", n)
	}

	if n <= HighUncertaintySampleSize {
		return fmt.Sprintf("hohe Parameterunsicherheit bei n=%d (ab n>%d sind die Schätzungen stabiler); Parameter und AIC-Ranking sind nur eingeschränkt belastbar", n, HighUncertaintySampleSize), nil
	}
	return "", nil
}

func isConstant(values []float64) bool {
	for _, v := range values[1:] {
		if v != values[0] {
			return false
		}
	}
	return true
}
