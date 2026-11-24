package distribution

import (
	"math"
	"math/rand/v2"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
)

// DipTestResult enthält die Dip-Statistik und den berechneten p-Wert.
type DipTestResult struct {
	Dip    float64
	PValue float64
}

// dipWorkspace bündelt die internen Hilfsslices von DipStatistic, damit sie
// in Monte-Carlo-Simulationen vorallokiert und wiederverwendet werden können.
type dipWorkspace struct {
	sorted []float64 // Länge n
	xx     []float64 // Länge n+1, 1-basiert
	mn     []int     // Länge n+1
	mj     []int     // Länge n+1
	gcm    []int     // Länge n+2
	lcm    []int     // Länge n+2
}

func growSlice[T any](s []T, n int) []T {
	if cap(s) < n {
		return make([]T, n)
	}
	return s[:n]
}

// init stellt sicher, dass alle Felder mindestens Länge n haben.
func (ws *dipWorkspace) init(n int) {
	ws.sorted = growSlice(ws.sorted, n)
	ws.xx = growSlice(ws.xx, n+1)
	ws.mn = growSlice(ws.mn, n+1)
	ws.mj = growSlice(ws.mj, n+1)
	ws.gcm = growSlice(ws.gcm, n+2)
	ws.lcm = growSlice(ws.lcm, n+2)
}

// DipStatistic berechnet Hartigans Dip-Statistik für einen Slice von float64-Werten.
// Basierend auf dem Algorithmus AS 217 (Hartigan & Hartigan, 1985), inkl. der
// Mechler-Korrektur gegen Endlosschleifen bei unimodalen Eingaben: die
// Hauptschleife bricht ab, sobald sich das modale Intervall [low, high]
// nicht mehr verkleinert (siehe src/dip.c der R-Referenzimplementierung).
// Intern 1-basierte Indizierung wie im Fortran-Original.
func DipStatistic(data []float64) float64 {
	return dipStatistic(data, &dipWorkspace{})
}

func dipStatistic(data []float64, ws *dipWorkspace) float64 {
	n := len(data)
	if n < 4 {
		return 0.0
	}
	ws.init(n)

	// Nur endliche Werte verwenden: NaN/Inf würden Sortierung und
	// Hauptschleife korrumpieren bzw. in Endlosschleifen laufen.
	sorted := ws.sorted[:0]
	for _, v := range data {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			sorted = append(sorted, v)
		}
	}
	n = len(sorted)
	if n < 4 {
		return 0.0
	}
	slices.Sort(sorted)

	// Prüfen, ob alle Werte identisch sind
	if sorted[0] == sorted[n-1] {
		return 0.0
	}

	// 1-basierte Indizierung wie im Original-Algorithmus (AS 217)
	xx := ws.xx
	copy(xx[1:], sorted)

	fn := float64(n)

	mn := ws.mn
	mj := ws.mj
	gcm := ws.gcm // Change-Points der GCM (Positionen im Array)
	lcm := ws.lcm // Change-Points der LCM

	// Indizes für die konkave Minorante (mn) aufbauen
	mn[1] = 1
	for j := 2; j <= n; j++ {
		mn[j] = j - 1
		for {
			mnj := mn[j]
			mnmnj := mn[mnj]
			a := float64(mnj - mnmnj)
			b := float64(j - mnj)
			if mnj == 1 || (xx[j]-xx[mnj])*a < (xx[mnj]-xx[mnmnj])*b {
				break
			}
			mn[j] = mnmnj
		}
	}

	// Indizes für die konvexe Majorante (mj) aufbauen
	mj[n] = n
	for jk := 1; jk <= n-1; jk++ {
		k := n - jk
		mj[k] = k + 1
		for {
			mjk := mj[k]
			mjmjk := mj[mjk]
			a := float64(mjk - mjmjk)
			b := float64(k - mjk)
			if mjk == n || (xx[k]-xx[mjk])*a < (xx[mjk]-xx[mjmjk])*b {
				break
			}
			mj[k] = mjmjk
		}
	}

	// Zyklus: LOW/HIGH begrenzen das aktuelle modale Intervall
	dip := 1.0 / fn
	low := 1
	high := n

	for {
		// Change-Points der GCM von HIGH nach LOW sammeln
		ic := 1
		gcm[1] = high
		for {
			igcm1 := gcm[ic]
			ic++
			gcm[ic] = mn[igcm1]
			if gcm[ic] <= low {
				break
			}
		}
		icx := ic

		// Change-Points der LCM von LOW nach HIGH sammeln
		ic = 1
		lcm[1] = low
		for {
			lcm1 := lcm[ic]
			ic++
			lcm[ic] = mj[lcm1]
			if lcm[ic] >= high {
				break
			}
		}
		icv := ic

		ig := icx
		ih := icv
		ix := icx - 1
		iv := 2
		d := 0.0

		if icx == 2 && icv == 2 {
			d = 1.0 / fn
		} else {
			// Größten Abstand zwischen GCM und LCM von LOW nach HIGH suchen
			for {
				igcmx := gcm[ix]
				lcmiv := lcm[iv]
				if igcmx > lcmiv {
					// Nächster Punkt gehört zur GCM
					igcm := gcm[ix]
					igcm1 := gcm[ix+1]
					a := float64(lcmiv - igcm1 + 1)
					b := float64(igcm - igcm1)
					dx := a/fn - ((xx[lcmiv]-xx[igcm1])*b)/(fn*(xx[igcm]-xx[igcm1]))
					iv++
					if dx >= d {
						d = dx
						ig = ix + 1
						ih = iv - 1
					}
				} else {
					// Nächster Punkt gehört zur LCM
					lcmiv1 := lcm[iv-1]
					a := float64(lcmiv - lcmiv1)
					b := float64(igcmx - lcmiv1 - 1)
					dx := (xx[igcmx]-xx[lcmiv1])*a/(fn*(xx[lcmiv]-xx[lcmiv1])) - b/fn
					ix--
					if dx >= d {
						d = dx
						ig = ix + 1
						ih = iv
					}
				}
				if ix < 1 {
					ix = 1
				}
				if iv > icv {
					iv = icv
				}
				if gcm[ix] == lcm[iv] {
					break
				}
			}
		}

		if d < dip {
			break
		}

		// Dip für den konvexen Minoranten (außerhalb des modalen Intervalls)
		dl := 0.0
		if ig != icx {
			for j := ig; j <= icx-1; j++ {
				temp := 1.0 / fn
				jb := gcm[j+1]
				je := gcm[j]
				if je-jb > 1 && xx[je] != xx[jb] {
					a := float64(je - jb)
					const_ := a / (fn * (xx[je] - xx[jb]))
					for jr := jb; jr <= je; jr++ {
						b := float64(jr - jb + 1)
						t := b/fn - (xx[jr]-xx[jb])*const_
						if t > temp {
							temp = t
						}
					}
				}
				if dl < temp {
					dl = temp
				}
			}
		}

		// Dip für den konkaven Majoranten
		du := 0.0
		if ih != icv {
			for k := ih; k <= icv-1; k++ {
				temp := 1.0 / fn
				kb := lcm[k]
				ke := lcm[k+1]
				if ke-kb > 1 && xx[ke] != xx[kb] {
					a := float64(ke - kb)
					const_ := a / (fn * (xx[ke] - xx[kb]))
					for kr := kb; kr <= ke; kr++ {
						b := float64(kr - kb - 1)
						t := (xx[kr]-xx[kb])*const_ - b/fn
						if t > temp {
							temp = t
						}
					}
				}
				if du < temp {
					du = temp
				}
			}
		}

		// Aktuelles Maximum bestimmen und LOW/HIGH aktualisieren.
		// Der Guard ist die Mechler-Korrektur: Wenn sich das modale
		// Intervall nicht mehr verkleinert, terminiert die Schleife —
		// ohne ihn könnte sie bei unimodalen Eingaben endlos laufen.
		dipNew := math.Max(dl, du)
		if dip < dipNew {
			dip = dipNew
		}
		newLow := gcm[ig]
		newHigh := lcm[ih]
		if newLow == low && newHigh == high {
			break
		}
		low = newLow
		high = newHigh
	}

	return 0.5 * dip
}

// DipTest führt den Dip-Test durch und berechnet den p-Wert per paralleler Monte-Carlo-Simulation.
// simulations: Anzahl der Zufallsstichproben (empfohlen: 2000 bis 10000).
// seed: Startwert für reproduzierbare Ergebnisse.
// Nicht-endliche Werte (NaN/Inf) werden wie in DipStatistic verworfen; die
// Stichprobengröße n der Simulation entspricht daher der Anzahl der
// endlichen Werte, nicht len(data).
func DipTest(data []float64, simulations int, seed uint64) DipTestResult {
	finite := make([]float64, 0, len(data))
	for _, v := range data {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			finite = append(finite, v)
		}
	}
	obsDip := DipStatistic(finite)
	n := len(finite)
	if n < 4 || simulations <= 0 {
		return DipTestResult{Dip: obsDip, PValue: 1.0}
	}

	numWorkers := runtime.NumCPU()

	// Die Simulationen werden in Chunks fester Größe zerlegt. Jeder Chunk
	// bekommt einen eigenen, nur von seiner globalen Position abhängigen
	// PRNG-Seed. Dadurch ist das Ergebnis reproduzierbar – unabhängig von
	// der Worker-Anzahl und der Chunk-Verteilung auf die CPUs.
	const chunkSize = 64
	numChunks := (simulations + chunkSize - 1) / chunkSize

	var wg sync.WaitGroup
	var nextChunk atomic.Int64
	var exceedTotal atomic.Int64

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			simSample := make([]float64, n)
			ws := &dipWorkspace{}
			ws.init(n)
			localExceed := 0

			for {
				c := int(nextChunk.Add(1) - 1)
				if c >= numChunks {
					break
				}
				start := c * chunkSize
				end := min(start+chunkSize, simulations)
				rng := rand.New(rand.NewPCG(seed, uint64(c)))

				for i := start; i < end; i++ {
					for j := 0; j < n; j++ {
						simSample[j] = rng.Float64() // U(0, 1) Gleichverteilung
					}
					if dipStatistic(simSample, ws) >= obsDip {
						localExceed++
					}
				}
			}
			exceedTotal.Add(int64(localExceed))
		}()
	}

	wg.Wait()
	totalExceed := int(exceedTotal.Load())

	// (+1)-Korrektur verhindert p = 0.0 bei endlichen Simulationen
	pValue := float64(totalExceed+1) / float64(simulations+1)

	return DipTestResult{
		Dip:    obsDip,
		PValue: pValue,
	}
}
