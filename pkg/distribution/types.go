package distribution

import "errors"

// ContinuousDistribution beschreibt eine kontinuierliche Wahrscheinlichkeitsverteilung.
type ContinuousDistribution interface {
	PDF(x float64) float64
	CDF(x float64) float64
	LogPDF(x float64) float64
	Params() []float64
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
	LogLikelihood float64
	AIC           float64
	BIC           float64
	KSStatistic   float64
	RMSE          float64
}

// Fehlerdefinitionen
var (
	ErrEmptyData   = errors.New("daten-slice ist leer")
	ErrInvalidData = errors.New("daten enthalten ungültige Werte (<= 0, NaN oder Inf)")
	ErrFitFailed   = errors.New("parameter-schätzung fehlgeschlagen")
)
