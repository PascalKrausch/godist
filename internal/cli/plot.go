package cli

import (
	"github.com/PascalKrausch/godist/internal/plot"
	"github.com/PascalKrausch/godist/pkg/distribution"
)

// BuildPlotSource liefert die Datenquelle des Dashboards. Die Payload wird pro Anfrage mit der gewünschten
// Klassenanzahl und dem KDE-Glättungsfaktor berechnet (kdeFactor <= 0 bedeutet Silverman-Standard).
// silverman ist optional und wird nicht neu berechnet; nil lässt den Modentest weg.
func BuildPlotSource(values []float64, results []distribution.FitResult, silverman *distribution.SilvermanTestResult) func(bins int, kdeFactor float64) plot.Payload {
	models := make([]plot.Model, 0, len(results))
	for _, result := range results {
		models = append(models, plot.Model{
			Name:    result.Name,
			Dist:    result.Dist,
			Metrics: result.Metrics,
		})
	}
	return func(bins int, kdeFactor float64) plot.Payload {
		return plot.BuildPayloadWithBins(values, models, bins, kdeFactor, silverman)
	}
}
