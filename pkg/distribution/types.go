package distribution

import "errors"

// ContinuousDistribution beschreibt eine kontinuierliche Wahrscheinlichkeitsverteilung.
type ContinuousDistribution interface {
	PDF(x float64) float64
	CDF(x float64) float64
	LogPDF(x float64) float64
	// Quantile liefert das p-Quantil für p in [0, 1] (Umkehrung der CDF); sonst bzw. bei ungültigen Parametern NaN.
	Quantile(p float64) float64
	Params() []float64
	// ParamNames liefert die Namen der Parameter in derselben Reihenfolge wie Params().
	ParamNames() []string
}

// Fitter beschreibt eine Schnittstelle zur Parameterschätzung für Verteilungen.
type Fitter interface {
	Fit(data []float64) (ContinuousDistribution, error)
	Name() string
}

// FitResult enthält das gefittete Modell und die zugehörigen Qualitätsmetriken.
type FitResult struct {
	Dist    ContinuousDistribution
	Metrics FitMetrics
}

// FitMetrics enthält verschiedene Metriken zur Bewertung der Güte eines Fits.
type FitMetrics struct {
	SampleSize    int // Anzahl der verarbeiteten Datenpunkte
	Discarded     int // Anzahl der ausgeschlossenen Eingabewerte (ungültig bzw. außerhalb des Trägers)
	LogLikelihood float64
	AIC           float64
	BIC           float64
	KSStatistic   float64
	RMSE          float64
	Converged     bool // true wenn die Optimierung konvergiert ist
}

type FitFailure struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

// Fehlerdefinitionen
var (
	ErrEmptyData   = errors.New("daten-slice ist leer")
	ErrInvalidData = errors.New("daten enthalten ungültige Werte (<= 0, NaN oder Inf)")
	ErrFitFailed   = errors.New("parameter-schätzung fehlgeschlagen")
)
