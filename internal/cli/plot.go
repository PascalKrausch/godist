package cli

import (
	"github.com/PascalKrausch/godist/internal/plot"
	"github.com/PascalKrausch/godist/pkg/distribution"
)

// BuildPlotSource liefert die Datenquelle des Dashboards. Die Payload wird pro Anfrage mit der gewünschten Klassenanzahl berechnet.
func BuildPlotSource(values []float64, results []distribution.FitResult) func(bins int) plot.Payload {
	models := make([]plot.Model, 0, len(results))
	for _, result := range results {
		models = append(models, plot.Model{
			Name:    getDistributionName(result.Dist),
			Dist:    result.Dist,
			Metrics: result.Metrics,
		})
	}
	return func(bins int) plot.Payload {
		return plot.BuildPayloadWithBins(values, models, bins)
	}
}
