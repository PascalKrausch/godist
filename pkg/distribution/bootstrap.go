package distribution

import (
	"errors"
	"math"
	"math/rand/v2"
	"runtime"
	"sort"
	"sync"
	"time"
)

// BootstrapMethod legt fest, wie die Bootstrap-Stichproben erzeugt werden.
type BootstrapMethod int

const (
	// BootNonParametric zieht mit Zurücklegen aus den beobachteten Daten (resampling).
	BootNonParametric BootstrapMethod = iota
	// BootParametric zieht aus der gefitteten Verteilung (Inversion über Quantile).
	BootParametric
)

// String liefert den kanonischen Namen für CLI und JSON.
func (m BootstrapMethod) String() string {
	switch m {
	case BootParametric:
		return "parametric"
	default:
		return "nonparametric"
	}
}

// ParseBootstrapMethod parst den Methodennamen; unbekannte Werte sind ein Fehler.
func ParseBootstrapMethod(s string) (BootstrapMethod, error) {
	switch s {
	case "nonparametric":
		return BootNonParametric, nil
	case "parametric":
		return BootParametric, nil
	default:
		return BootNonParametric, errors.New("ungültige bootstrap-methode (erlaubt: nonparametric, parametric)")
	}
}

// BootstrapConfig steuert die Bootstrap-Wiederholung. B ist die Anzahl der Replicates.
type BootstrapConfig struct {
	B      int
	Method BootstrapMethod
	// Level ist das Konfidenzniveau der Perzentil-Intervalle, z. B. 0.95.
	Level float64
	// Seed macht den Lauf reproduzierbar. 0 bedeutet: intern erzeugter, nicht reproduzierbarer Seed.
	Seed uint64
}

// ParameterCI ist das Bootstrap-Konfidenzintervall eines einzelnen Parameters.
type ParameterCI struct {
	Name string  `json:"name"`
	Mean float64 `json:"mean"`
	Lo   float64 `json:"lo"`
	Hi   float64 `json:"hi"`
}

// MetricCI ist das Bootstrap-Konfidenzintervall einer Gütemetrik.
type MetricCI struct {
	Mean float64 `json:"mean"`
	Lo   float64 `json:"lo"`
	Hi   float64 `json:"hi"`
}

// BootstrapResult fasst die Bootstrap-Auswertung eines einzelnen Fits zusammen.
// Replicates ist die Anzahl erfolgreich ausgewerteter Wiederholungen; Failed zählt
// die Replicates, deren Fit oder Metrikberechnung scheiterte.
type BootstrapResult struct {
	Method     string              `json:"method"`
	Requested  int                 `json:"requested"`
	Replicates int                 `json:"replicates"`
	Failed     int                 `json:"failed"`
	Level      float64             `json:"level"`
	Seed       uint64              `json:"seed"`
	DurationMs float64             `json:"duration_ms,omitempty"`
	Params     []ParameterCI       `json:"params"`
	Metrics    map[string]MetricCI `json:"metrics"`
}

// RandomBootstrapSeed liefert einen nichtdeterministischen Seed ungleich 0.
// Aufrufer, die mehrere BootstrapFit-Läufe reproduzierbar machen wollen, sollten
// den Seed einmal erzeugen und in jedem BootstrapConfig verwenden.
func RandomBootstrapSeed() uint64 {
	for {
		if s := rand.Uint64(); s != 0 {
			return s
		}
	}
}

// bootstrapMetricKeys sind die FitMetrics, für die Bootstrap-Intervalle berechnet werden.
var bootstrapMetricKeys = []struct {
	key string
	get func(FitMetrics) float64
}{
	{"aic", func(m FitMetrics) float64 { return m.AIC }},
	{"bic", func(m FitMetrics) float64 { return m.BIC }},
	{"ks", func(m FitMetrics) float64 { return m.KSStatistic }},
	{"rmse", func(m FitMetrics) float64 { return m.RMSE }},
}

// BootstrapFit wiederholt das Fitting und schätzt Perzentil-Konfidenzintervalle
// für die Parameter und die Gütemetriken. Die Replicates laufen parallel in einem
// Worker-Pool der Größe GOMAXPROCS; jeder Replicate hat einen eigenen RNG, daher
// ist der Lauf bei festem Seed reproduzierbar.
func BootstrapFit(fitter Fitter, dist ContinuousDistribution, data []float64, cfg BootstrapConfig) (*BootstrapResult, error) {
	if fitter == nil || dist == nil {
		return nil, errors.New("bootstrap benötigt fitter und gefittete verteilung")
	}
	if cfg.B < 1 {
		return nil, errors.New("bootstrap: B muss mindestens 1 sein")
	}
	if cfg.Method != BootNonParametric && cfg.Method != BootParametric {
		return nil, errors.New("bootstrap: unbekannte methode")
	}
	if cfg.Level <= 0 || cfg.Level >= 1 {
		return nil, errors.New("bootstrap: level muss in (0, 1) liegen")
	}
	cleaned := CleanDataWithZero(data)
	n := len(cleaned)
	if n == 0 {
		return nil, ErrEmptyData
	}

	seed := cfg.Seed
	if seed == 0 {
		seed = RandomBootstrapSeed()
	}

	reps := make([]replicateResult, cfg.B)
	failed := make([]bool, cfg.B)

	start := time.Now()
	workers := min(runtime.GOMAXPROCS(0), cfg.B)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				reps[i], failed[i] = runBootstrapReplicate(fitter, dist, cleaned, cfg.Method, seed, i)
			}
		}()
	}
	for i := 0; i < cfg.B; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	res := &BootstrapResult{
		Method:     cfg.Method.String(),
		Requested:  cfg.B,
		Level:      cfg.Level,
		Seed:       seed,
		DurationMs: float64(time.Since(start)) / float64(time.Millisecond),
	}

	names := dist.ParamNames()
	paramVals := make([][]float64, len(names))
	metricVals := make(map[string][]float64, len(bootstrapMetricKeys))
	for _, mk := range bootstrapMetricKeys {
		metricVals[mk.key] = make([]float64, 0, cfg.B)
	}

	for i := range reps {
		if failed[i] {
			res.Failed++
			continue
		}
		res.Replicates++
		for j, v := range reps[i].params {
			if j < len(paramVals) && IsFinite(v) {
				paramVals[j] = append(paramVals[j], v)
			}
		}
		for _, mk := range bootstrapMetricKeys {
			if v := mk.get(reps[i].metrics); IsFinite(v) {
				metricVals[mk.key] = append(metricVals[mk.key], v)
			}
		}
	}

	if res.Replicates < 2 {
		return nil, errors.New("bootstrap: weniger als 2 Wiederholungen erfolgreich")
	}

	for j, name := range names {
		if len(paramVals[j]) == 0 {
			continue
		}
		mean, lo, hi := bootstrapCI(paramVals[j], cfg.Level)
		res.Params = append(res.Params, ParameterCI{Name: name, Mean: mean, Lo: lo, Hi: hi})
	}

	res.Metrics = make(map[string]MetricCI, len(bootstrapMetricKeys))
	for _, mk := range bootstrapMetricKeys {
		vals := metricVals[mk.key]
		if len(vals) == 0 {
			continue
		}
		mean, lo, hi := bootstrapCI(vals, cfg.Level)
		res.Metrics[mk.key] = MetricCI{Mean: mean, Lo: lo, Hi: hi}
	}

	return res, nil
}

// runBootstrapReplicate erzeugt eine Bootstrap-Stichprobe, fitet sie und bewertet den Fit.
// Der zweite Rückgabewert ist true, wenn die Replicate fehlschlug.
func runBootstrapReplicate(fitter Fitter, dist ContinuousDistribution, data []float64, method BootstrapMethod, seed uint64, i int) (replicateResult, bool) {
	rng := rand.New(rand.NewPCG(seed, uint64(i)+1))
	sample := make([]float64, len(data))

	if method == BootParametric {
		for j := range sample {
			p := rng.Float64()
			if p <= 0 {
				p = math.SmallestNonzeroFloat64
			}
			sample[j] = dist.Quantile(p)
		}
		for _, v := range sample {
			if !IsFinite(v) {
				return replicateResult{}, true
			}
		}
	} else {
		for j := range sample {
			sample[j] = data[rng.IntN(len(data))]
		}
	}

	d, err := fitter.Fit(sample)
	if err != nil {
		return replicateResult{}, true
	}
	m, err := EvaluateFit(d, sample)
	if err != nil {
		return replicateResult{}, true
	}
	return replicateResult{params: d.Params(), metrics: m}, false
}

// replicateResult hält die Ergebnisse einer einzelnen Bootstrap-Wiederholung.
type replicateResult struct {
	params  []float64
	metrics FitMetrics
}

// bootstrapCI berechnet Mittelwert und Perzentil-Intervall (Typ-7-Quantile) aus den Werten.
// values wird sortiert und darf danach als sortiert betrachtet werden.
func bootstrapCI(values []float64, level float64) (mean, lo, hi float64) {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	var sum float64
	for _, v := range sorted {
		sum += v
	}
	mean = sum / float64(len(sorted))

	alpha := 1 - level
	lo = quantileSortedF(sorted, alpha/2)
	hi = quantileSortedF(sorted, 1-alpha/2)
	return mean, lo, hi
}

// quantileSortedF interpoliert ein Quantil (Typ 7) aus sortierten Werten.
func quantileSortedF(sorted []float64, p float64) float64 {
	if len(sorted) == 1 {
		return sorted[0]
	}
	pos := p * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	frac := pos - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}
