// Package plot bereitet Fit-Ergebnisse als Datenpunkte für Diagramme auf.
// Es enthält keine Darstellung; Rendering übernimmt das eingebettete Frontend.
package plot

import (
	"math"
	"slices"
	"sort"

	"github.com/PascalKrausch/godist/pkg/distribution"
)

// maxPoints begrenzt die Anzahl der Punkte für ECDF und Q-Q, damit große Datensätze handhabbar bleiben.
const maxPoints = 2000

// gridPoints ist die Auflösung des gleichmäßigen Rasters für PDF und CDF.
const gridPoints = 300

// maxBins begrenzt die Klassenanzahl des Histogramms.
const maxBins = 500

// Model ist ein angepasstes Modell mit Anzeigename.
type Model struct {
	Name string
	Dist distribution.ContinuousDistribution
}

// Series ist eine Folge von (x, y)-Punkten.
type Series struct {
	X []float64 `json:"x"`
	Y []float64 `json:"y"`
}

// Histogram enthält gleich breite Klassen. Density ist so normiert, dass die Fläche 1 ergibt.
// GridDensity ist dieselbe Dichte, ausgewertet auf dem gemeinsamen Raster der Payload.
type Histogram struct {
	Edges       []float64 `json:"edges"`
	Counts      []int     `json:"counts"`
	Density     []float64 `json:"density"`
	GridDensity []float64 `json:"grid_density"`
}

// ModelPlot enthält die Kurven eines Modells auf dem gemeinsamen Raster.
// Ungültige Werte (NaN, Inf) sind null, damit die Ausgabe gültiges JSON bleibt.
type ModelPlot struct {
	Name string     `json:"name"`
	PDF  []*float64 `json:"pdf"`
	CDF  []*float64 `json:"cdf"`
	// QQ: X = theoretisches Quantil, Y = empirisches Quantil.
	QQ Series `json:"qq"`
}

// Payload ist alle Daten, die das Dashboard benötigt.
// Grid ist das gemeinsame X-Gitter; Histogram.GridDensity, ECDF und die Modellkurven haben dieselbe Länge.
type Payload struct {
	SampleSize int         `json:"sample_size"`
	Grid       []float64   `json:"grid"`
	Histogram  Histogram   `json:"histogram"`
	ECDF       []float64   `json:"ecdf"`
	Models     []ModelPlot `json:"models"`
}

// MaxBins ist die größte zulässige Klassenanzahl des Histogramms.
const MaxBins = maxBins

// BuildPayload erstellt alle Diagrammdaten mit automatischer Klassenanzahl.
// Die Werte müssen bereits bereinigt sein (endlich, im Träger der Modelle).
func BuildPayload(values []float64, models []Model) Payload {
	return BuildPayloadWithBins(values, models, 0)
}

// BuildPayloadWithBins wie BuildPayload, mit fester Klassenanzahl. bins <= 0 wählt die Anzahl automatisch,
// größere Werte als MaxBins werden begrenzt.
func BuildPayloadWithBins(values []float64, models []Model, bins int) Payload {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	payload := Payload{
		SampleSize: len(sorted),
		Models:     make([]ModelPlot, 0, len(models)),
	}
	if len(sorted) == 0 {
		return payload
	}
	if bins <= 0 {
		bins = FreedmanDiaconisBins(sorted)
	}
	payload.Histogram = NewHistogram(sorted, min(bins, maxBins))
	payload.Grid = buildGrid(sorted, payload.Histogram.Edges)
	payload.ECDF = ecdfOnGrid(sorted, payload.Grid)
	payload.Histogram.GridDensity = histogramOnGrid(payload.Histogram, payload.Grid)

	for _, m := range models {
		payload.Models = append(payload.Models, ModelPlot{
			Name: m.Name,
			PDF:  evalOnGrid(payload.Grid, m.Dist.PDF),
			CDF:  evalOnGrid(payload.Grid, m.Dist.CDF),
			QQ:   NewQQ(sorted, m.Dist),
		})
	}
	return payload
}

// FreedmanDiaconisBins wählt die Klassenanzahl nach der Freedman-Diaconis-Regel.
// Bei IQR null wird auf die Sturges-Regel zurückgegriffen. Das Ergebnis liegt in [1, maxBins].
func FreedmanDiaconisBins(sorted []float64) int {
	n := len(sorted)
	if n < 2 || sorted[0] == sorted[n-1] {
		return 1
	}
	iqr := quantileSorted(sorted, 0.75) - quantileSorted(sorted, 0.25)
	var bins int
	if iqr > 0 {
		width := 2 * iqr / math.Cbrt(float64(n))
		bins = int(math.Ceil((sorted[n-1] - sorted[0]) / width))
	} else {
		bins = int(math.Ceil(math.Log2(float64(n)) + 1))
	}
	return min(max(bins, 1), maxBins)
}

// NewHistogram erstellt ein Histogramm mit bins gleich breiten Klassen über den Datenbereich.
func NewHistogram(sorted []float64, bins int) Histogram {
	if bins < 1 {
		bins = 1
	}
	lo, hi := histogramRange(sorted)
	width := (hi - lo) / float64(bins)

	h := Histogram{
		Edges:   make([]float64, bins+1),
		Counts:  make([]int, bins),
		Density: make([]float64, bins),
	}
	for i := range h.Edges {
		h.Edges[i] = lo + float64(i)*width
	}
	// Die letzte Grenze exakt setzen, damit der Maximalwert nicht durch Rundung aus dem Histogramm fällt.
	h.Edges[bins] = hi
	for _, v := range sorted {
		idx := min(max(int((v-lo)/width), 0), bins-1)
		h.Counts[idx]++
	}
	n := float64(len(sorted))
	for i, c := range h.Counts {
		h.Density[i] = float64(c) / (n * width)
	}
	return h
}

// buildGrid vereint das gleichmäßige Raster mit den Klassengrenzen und den ECDF-Sprungstellen.
// So liegen alle Serien auf demselben X-Gitter, und Stufen fallen exakt auf Punkte.
func buildGrid(sorted, edges []float64) []float64 {
	lo, hi := gridRange(sorted)
	step := (hi - lo) / float64(gridPoints-1)

	grid := make([]float64, 0, gridPoints+len(edges)+maxPoints)
	for k := 0; k < gridPoints; k++ {
		grid = append(grid, lo+float64(k)*step)
	}
	grid = append(grid, edges...)
	for _, i := range sampleIndices(len(sorted)) {
		grid = append(grid, sorted[i])
	}
	sort.Float64s(grid)
	return slices.Compact(grid)
}

// ecdfOnGrid wertet die empirische Verteilungsfunktion rechtsstetig auf dem Raster aus.
// Bei mehr als maxPoints Werten approximiert das Raster die Sprünge zwischen den gewählten Stützstellen.
func ecdfOnGrid(sorted, grid []float64) []float64 {
	n := float64(len(sorted))
	out := make([]float64, len(grid))
	for i, x := range grid {
		out[i] = float64(sort.Search(len(sorted), func(k int) bool { return sorted[k] > x })) / n
	}
	return out
}

// histogramOnGrid wertet die Klassendichte als Treppenfunktion auf dem Raster aus. Außerhalb der Klassen ist sie null.
func histogramOnGrid(h Histogram, grid []float64) []float64 {
	edges := h.Edges
	bins := len(h.Density)
	out := make([]float64, len(grid))
	for i, x := range grid {
		if x < edges[0] || x > edges[len(edges)-1] {
			continue
		}
		k := sort.Search(len(edges), func(j int) bool { return edges[j] > x }) - 1
		out[i] = h.Density[min(k, bins-1)]
	}
	return out
}

// NewQQ vergleicht die empirischen Quantile mit den Quantilen des Modells.
// Plotting-Positionen sind (i + 0.5) / n. Nicht endliche Modellquantile werden übersprungen.
func NewQQ(sorted []float64, dist distribution.ContinuousDistribution) Series {
	n := len(sorted)
	s := Series{}
	for _, i := range sampleIndices(n) {
		p := (float64(i) + 0.5) / float64(n)
		theo := dist.Quantile(p)
		if math.IsNaN(theo) || math.IsInf(theo, 0) {
			continue
		}
		s.X = append(s.X, theo)
		s.Y = append(s.Y, sorted[i])
	}
	return s
}

// sampleIndices liefert höchstens maxPoints gleichmäßig verteilte Indizes in [0, n).
func sampleIndices(n int) []int {
	if n <= maxPoints {
		idx := make([]int, n)
		for i := range idx {
			idx[i] = i
		}
		return idx
	}
	idx := make([]int, maxPoints)
	for k := range idx {
		idx[k] = k * (n - 1) / (maxPoints - 1)
	}
	return idx
}

// evalOnGrid wertet f auf dem Raster aus. Nicht endliche Werte werden zu null.
func evalOnGrid(grid []float64, f func(float64) float64) []*float64 {
	out := make([]*float64, len(grid))
	for i, x := range grid {
		y := f(x)
		if math.IsNaN(y) || math.IsInf(y, 0) {
			continue
		}
		out[i] = &y
	}
	return out
}

// histogramRange liefert den Datenbereich; bei konstanten Daten wird er künstlich um ±0.5 erweitert.
func histogramRange(sorted []float64) (float64, float64) {
	lo, hi := sorted[0], sorted[len(sorted)-1]
	if lo == hi {
		return lo - 0.5, hi + 0.5
	}
	return lo, hi
}

// gridRange erweitert den Datenbereich um 10 % auf beiden Seiten für die Kurvenauswertung.
func gridRange(sorted []float64) (float64, float64) {
	lo, hi := histogramRange(sorted)
	pad := 0.1 * (hi - lo)
	return lo - pad, hi + pad
}

// quantileSorted berechnet ein lineares Quantil (Typ 7) aus bereits sortierten Daten.
func quantileSorted(sorted []float64, p float64) float64 {
	pos := p * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	frac := pos - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}
