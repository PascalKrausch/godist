package distribution

import (
	"errors"
	"math"
	"math/rand/v2"
	"runtime"
	"slices"
	"sort"
	"sync"
	"sync/atomic"

	"gonum.org/v1/gonum/stat"
)

// SilvermanTestResult enthält die kritische Bandbreite und den per geglättetem
// Bootstrap ermittelten p-Wert für den Test, ob eine Dichte mehr als Modes
// Modi besitzt.
type SilvermanTestResult struct {
	Modes              int     `json:"modes"`
	CriticalBandwidth  float64 `json:"critical_bandwidth"`
	ReferenceBandwidth float64 `json:"reference_bandwidth"`
	BandwidthRatio     float64 `json:"bandwidth_ratio"`
	PValue             float64 `json:"p_value"`
	Simulations        int     `json:"simulations"`
	Seed               uint64  `json:"seed"`
}

// SilvermanTest prüft die Nullhypothese, dass die zugrunde liegende Dichte
// höchstens modes Modi besitzt. Verwendet wird Silvermans Verfahren der
// kritischen Bandbreite mit geglättetem Bootstrap.
// Nicht-endliche Beobachtungen (NaN, ±Inf) werden verworfen. Der Bootstrap
// folgt dem originalen Silverman-Rescaling; eine Hall-York-Korrektur wird
// nicht angewendet.
func SilvermanTest(data []float64, modes, simulations int, seed uint64) (SilvermanTestResult, error) {
	if modes < 1 {
		return SilvermanTestResult{}, errors.New("modes must be at least 1")
	}
	if simulations < 1 {
		return SilvermanTestResult{}, errors.New("simulations must be at least 1")
	}

	// Nur endliche Werte übernehmen und sortieren.
	values := make([]float64, 0, len(data))
	for _, value := range data {
		if !math.IsNaN(value) && !math.IsInf(value, 0) {
			values = append(values, value)
		}
	}
	slices.Sort(values)

	// Standardergebnis mit p-Wert 1 (Vorgabe, falls kein Test möglich ist).
	result := SilvermanTestResult{Modes: modes, PValue: 1, Simulations: simulations, Seed: seed}
	// Bei weniger als zwei Werten oder konstanten Daten gibt es nichts zu testen.
	if len(values) < 2 || values[0] == values[len(values)-1] {
		return result, nil
	}
	// Stichprobenstandardabweichung als Referenz für die Bandbreite.
	stdDev := stat.StdDev(values, nil)
	result.ReferenceBandwidth = SilvermanBandwidth(values, stdDev)
	result.CriticalBandwidth = criticalBandwidth(values, modes)
	if result.ReferenceBandwidth > 0 {
		result.BandwidthRatio = result.CriticalBandwidth / result.ReferenceBandwidth
	}
	// Gibt es höchstens so viele verschiedene Werte wie Modi, ist der Test trivial.
	if modes >= uniqueCount(values) {
		return result, nil
	}

	observed := result.CriticalBandwidth
	// Parallelisierung: Ein Worker pro CPU-Kern.
	numWorkers := runtime.NumCPU()
	// Simulationen werden in Blöcke aufgeteilt, um die Arbeit auf Worker zu verteilen.
	const chunkSize = 16
	numChunks := (simulations + chunkSize - 1) / chunkSize

	var wg sync.WaitGroup
	var nextChunk atomic.Int64
	var exceedTotal atomic.Int64
	for worker := 0; worker < numWorkers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Puffer für eine Bootstrap-Stichprobe, wird pro Worker wiederverwendet.
			sample := make([]float64, len(values))
			localExceed := 0
			for {
				// Nächsten freien Block holen.
				chunk := int(nextChunk.Add(1) - 1)
				if chunk >= numChunks {
					break
				}
				start := chunk * chunkSize
				end := min(start+chunkSize, simulations)
				// Deterministischer RNG pro Block, damit Ergebnisse reproduzierbar sind.
				rng := rand.New(rand.NewPCG(seed, uint64(chunk)))
				for i := start; i < end; i++ {
					// Ziehen mit Zurücklegen aus den Originaldaten.
					for j := range sample {
						sample[j] = values[rng.IntN(len(values))]
					}
					// Geglättete Stichprobe mit Silverman-Rescaling erzeugen.
					rescaleBootstrap(sample, observed, stdDev, rng)
					slices.Sort(sample)
					// Zählen, ob die Bootstrap-Stichprobe mehr als modes Modi hat.
					if countModes(sample, observed) > modes {
						localExceed++
					}
				}
			}
			exceedTotal.Add(int64(localExceed))
		}()
	}
	wg.Wait()
	// p-Wert mit Korrektur (+1 in Zähler und Nenner), damit er nie exakt 0 ist.
	result.PValue = float64(exceedTotal.Load()+1) / float64(simulations+1)
	return result, nil
}

// SilvermanBandwidth liefert die Faustregel-Bandbreite nach Silverman
// h = 0.9 · min(σ, IQR/1.349) · n^(-1/5) für sortierte, aufsteigende Daten.
// Ist h nicht positiv, wird 0 zurückgegeben.
func SilvermanBandwidth(sorted []float64, stdDev float64) float64 {
	n := len(sorted)
	if n < 2 || stdDev <= 0 || math.IsNaN(stdDev) || math.IsInf(stdDev, 0) {
		return 0
	}
	sigma := stdDev
	if iqr := quantileSortedF(sorted, 0.75) - quantileSortedF(sorted, 0.25); iqr > 0 {
		if robustSigma := iqr / 1.349; robustSigma < sigma {
			sigma = robustSigma
		}
	}
	return 0.9 * sigma * math.Pow(float64(n), -0.2)
}

// rescaleBootstrap ersetzt die Stichprobe in place durch eine geglättete
// Bootstrap-Stichprobe nach Silvermans Rescaling: Mittelwert und Varianz
// bleiben erhalten, die Bandbreite wird per Normalverteilungsrauschen addiert.
func rescaleBootstrap(sample []float64, bandwidth, fallbackStdDev float64, rng *rand.Rand) {
	// Mittelwert der Stichprobe berechnen.
	mean := 0.0
	for _, value := range sample {
		mean += value
	}
	mean /= float64(len(sample))

	// Stichprobenvarianz berechnen.
	variance := 0.0
	for _, value := range sample {
		delta := value - mean
		variance += delta * delta
	}
	variance /= float64(len(sample) - 1)
	// Fällt die Varianz weg (z. B. identische Werte), wird die Referenzstandardabweichung genutzt.
	if variance <= 0 {
		variance = fallbackStdDev * fallbackStdDev
	}
	// Skalierungsfaktor, der die zusätzliche Varianz der Glättung ausgleicht.
	scale := math.Sqrt(1 + bandwidth*bandwidth/variance)
	// Jeden Wert um den Mittelwert zentrieren, Rauschen addieren und skalieren.
	for i := range sample {
		sample[i] = mean + (sample[i]-mean+bandwidth*rng.NormFloat64())/scale
	}
}

// criticalBandwidth bestimmt per Bisektion die kleinste Bandbreite, bei der
// die KDE höchstens modes Modi besitzt.
func criticalBandwidth(sorted []float64, modes int) float64 {
	if len(sorted) < 2 || modes >= uniqueCount(sorted) {
		return 0
	}
	low := 0.0
	// Obere Schranke: Standardabweichung, bei Bedarf verdoppeln, bis die Bedingung erfüllt ist.
	high := stat.StdDev(sorted, nil)
	if high <= 0 || math.IsNaN(high) || math.IsInf(high, 0) {
		return 0
	}
	for countModes(sorted, high) > modes {
		high *= 2
		if math.IsInf(high, 0) {
			return 0
		}
	}
	// Bisektion mit höchstens 36 Schritten oder bei relativer Genauigkeit 1e-5.
	for i := 0; i < 36; i++ {
		mid := low + (high-low)/2
		if countModes(sorted, mid) <= modes {
			high = mid
		} else {
			low = mid
		}
		if high-low <= 1e-5*high {
			break
		}
	}
	return high
}

// countModes zählt die lokalen Maxima der Gauß-KDE mit Bandbreite bandwidth
// anhand der Vorzeichenwechsel der Ableitung (+ nach −).
func countModes(sorted []float64, bandwidth float64) int {
	if len(sorted) == 0 {
		return 0
	}
	if bandwidth <= 0 {
		// Ohne Glättung entspricht die Anzahl Modi der Anzahl verschiedener Werte.
		return uniqueCount(sorted)
	}

	minValue, maxValue := sorted[0], sorted[len(sorted)-1]
	span := maxValue - minValue
	if span <= 0 {
		return 1
	}
	// Abtastpunkte: mindestens 256, höchstens 4096.
	points := int(math.Ceil(span / (bandwidth / 10)))
	points = max(256, min(points, 4096))
	left := minValue - 4*bandwidth
	step := (span + 8*bandwidth) / float64(points)
	previous := kdeDerivative(sorted, left, bandwidth)
	modes := 0
	for i := 1; i <= points; i++ {
		x := left + float64(i)*step
		current := kdeDerivative(sorted, x, bandwidth)
		// Vorzeichenwechsel von positiv zu nicht positiv markiert ein Maximum.
		if previous > 0 && current <= 0 {
			modes++
		}
		previous = current
	}
	return modes
}

// kdeDerivative berechnet die Ableitung der Gauß-KDE an der Stelle x.
// Nur Werte im Bereich von 8 Bandbreiten werden berücksichtigt, da der Beitrag
// weiter entfernter Werte vernachlässigbar ist.
func kdeDerivative(sorted []float64, x, bandwidth float64) float64 {
	reach := 8 * bandwidth
	lo := sortSearch(sorted, x-reach)
	hi := sortSearch(sorted, x+reach)
	sum := 0.0
	for _, value := range sorted[lo:hi] {
		z := (x - value) / bandwidth
		sum += z * math.Exp(-0.5*z*z)
	}
	return -sum
}

// sortSearch liefert den ersten Index in sorted, dessen Wert mindestens value ist
// (binäre Suche).
func sortSearch(sorted []float64, value float64) int {
	return sort.Search(len(sorted), func(i int) bool { return sorted[i] >= value })
}

// uniqueCount zählt die verschiedenen Werte in sorted (Voraussetzung: sortiert).
func uniqueCount(sorted []float64) int {
	count := 0
	for i, value := range sorted {
		if i == 0 || value != sorted[i-1] {
			count++
		}
	}
	return count
}
