package cli

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/PascalKrausch/godist/internal/stats"
	"github.com/PascalKrausch/godist/pkg/distribution"

	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

// FitRow enthält die Kennzahlen und Parameter eines angepassten Modells.
type FitRow struct {
	Name        string             `json:"name"`
	Params      map[string]float64 `json:"params"`
	AIC         float64            `json:"aic"`
	DeltaAIC    float64            `json:"delta_aic"`
	BIC         float64            `json:"bic"`
	KSStatistic float64            `json:"ks_statistic"`
	RMSE        float64            `json:"rmse"`
}

// FitReport enthält die zusammengefassten Ergebnisse eines Verteilungsfits.
type FitReport struct {
	Input            string                    `json:"input"`
	Column           string                    `json:"column,omitempty"`
	SampleSize       int                       `json:"sample_size"`
	Discarded        int                       `json:"discarded"`
	Candidates       int                       `json:"candidates"`
	BestDistribution string                    `json:"best_distribution,omitempty"`
	Descriptive      stats.Summary             `json:"descriptive"`
	Models           []FitRow                  `json:"models"`
	Failures         []distribution.FitFailure `json:"failures,omitempty"`
}

// Öffentliche Berichtsfunktionen

// BuildFitReport erstellt einen Bericht aus erfolgreichen Fits und Fehlern.
// sampleSize ist die Anzahl verwendeter Werte, discarded die Anzahl verworfener Eingaben.
func BuildFitReport(results []distribution.FitResult, failures []distribution.FitFailure, input string, column string, descriptive stats.Summary, discarded int) FitReport {
	report := FitReport{
		Input:       input,
		Column:      column,
		SampleSize:  descriptive.N,
		Discarded:   discarded,
		Descriptive: descriptive,
		Models:      make([]FitRow, 0, len(results)),
		Failures:    failures,
	}

	for _, result := range results {
		report.Models = append(report.Models, FitRow{
			Name:        getDistributionName(result.Dist),
			Params:      paramsToMap(result.Dist),
			AIC:         result.Metrics.AIC,
			BIC:         result.Metrics.BIC,
			KSStatistic: result.Metrics.KSStatistic,
			RMSE:        result.Metrics.RMSE,
		})
	}

	return report
}

// WriteTextReport schreibt den Bericht als lesbare Textausgabe.
// Ohne erfolgreiches Modell werden nur die Fehlschläge ausgegeben; die Entscheidung über den Exit-Code trifft der Aufrufer.
func WriteTextReport(w io.Writer, report FitReport) error {
	report = prepareReport(report)

	fmt.Fprintln(w, "GODIST — Distribution Analysis")
	if len(report.Models) == 0 {
		fmt.Fprintln(w, "Kein Modell konnte erfolgreich angepasst werden.")
		for _, failure := range report.Failures {
			fmt.Fprintf(w, "  %s: %s\n", failure.Name, failure.Error)
		}
		fmt.Fprintln(w)
		writeDescriptive(w, report.Descriptive)
		return nil
	}
	fmt.Fprintln(w)

	if report.Input != "" {
		fmt.Fprintf(w, "Input:        %s\n", report.Input)
	}
	if report.Column != "" {
		fmt.Fprintf(w, "Column:       %s\n", report.Column)
	}

	successful := len(report.Models)
	failed := len(report.Failures)

	fmt.Fprintf(w, "Observations: %d%s\n", report.SampleSize, discardedSuffix(report.Discarded))
	fmt.Fprintf(
		w,
		"Candidates:   %d (%d successful, %d failed)\n",
		report.Candidates,
		successful,
		failed,
	)
	fmt.Fprintln(w, "Criterion:    AIC (lower is better)")
	fmt.Fprintln(w)

	writeDescriptive(w, report.Descriptive)

	best := report.Models[0]

	fmt.Fprintln(w, "Best fit")
	fmt.Fprintln(w, "--------")
	fmt.Fprintf(w, "Distribution: %s\n", best.Name)
	fmt.Fprintf(w, "Parameters:   %s\n", formatParams(best.Params))
	fmt.Fprintf(w, "AIC / BIC:    %.2f / %.2f\n", best.AIC, best.BIC)
	fmt.Fprintln(w)

	tableRows := make([][]string, 0, len(report.Models))

	for i, model := range report.Models {
		tableRows = append(tableRows, []string{
			fmt.Sprintf("%d", i+1),
			model.Name,
			formatParams(model.Params),
			fmt.Sprintf("%.2f", model.AIC),
			fmt.Sprintf("%.2f", model.DeltaAIC),
			fmt.Sprintf("%.2f", model.BIC),
			fmt.Sprintf("%.4f", model.KSStatistic),
			fmt.Sprintf("%.4f", model.RMSE),
		})
	}

	fmt.Fprintln(w, "Model comparison")

	writeTable(w, []string{
		"Rank",
		"Distribution",
		"Parameters",
		"AIC",
		"Delta AIC",
		"BIC",
		"KS",
		"RMSE",
	}, tableRows)

	if len(report.Failures) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Models that could not be fitted:")
		for _, failure := range report.Failures {
			fmt.Fprintf(w, "  - %s: %s\n", failure.Name, failure.Error)
		}
	}

	fmt.Fprintln(w)
	return nil
}

// WriteSummary schreibt das Ergebnis des Einzelmodells (bestes Modell) als Zusammenfassung.
func WriteSummary(w io.Writer, dist distribution.ContinuousDistribution, metrics distribution.FitMetrics, column string, descriptive stats.Summary, discarded int) {
	fmt.Fprintln(w, "=== GODIST FITTER ERGEBNIS ===")
	if column != "" {
		fmt.Fprintf(w, "Spalte:         %s\n", column)
	}
	fmt.Fprintf(w, "Datenpunkte:    %d%s\n", descriptive.N, discardedSuffix(discarded))
	fmt.Fprintf(w, "Beste Kurve:    %s\n", getDistributionName(dist))
	fmt.Fprintf(w, "Parameter:      %s\n", formatParams(paramsToMap(dist)))
	fmt.Fprintf(w, "AIC / BIC:      %.2f / %.2f\n", metrics.AIC, metrics.BIC)
	fmt.Fprintf(w, "KS-Statistik:   %.4f (RMSE: %.4f)\n", metrics.KSStatistic, metrics.RMSE)
	fmt.Fprintln(w)
	writeDescriptive(w, descriptive)
}

// writeDescriptive schreibt die deskriptiven Kennzahlen als Block. Nicht berechenbare Werte erscheinen als Gedankenstrich.
func writeDescriptive(w io.Writer, s stats.Summary) {
	fmt.Fprintln(w, "Descriptive statistics")
	fmt.Fprintln(w, "----------------------")
	fmt.Fprintf(w, "Mean:         %s\n", formatStat(s.Mean))
	fmt.Fprintf(w, "Median:       %s\n", formatStat(s.Median))
	fmt.Fprintf(w, "Variance:     %s\n", formatStat(s.Variance))
	fmt.Fprintf(w, "Std. dev.:    %s\n", formatStat(s.StdDev))
	fmt.Fprintf(w, "Min / Max:    %s / %s\n", formatStat(s.Min), formatStat(s.Max))
	fmt.Fprintf(w, "Q1 / Q3:      %s / %s\n", formatStat(s.Q1), formatStat(s.Q3))
	fmt.Fprintf(w, "Skewness:     %s\n", formatStat(s.Skewness))
	fmt.Fprintf(w, "Kurtosis:     %s (excess)\n", formatStat(s.Kurtosis))
	fmt.Fprintln(w)
}

// formatStat formatiert eine Kennzahl mit vier Nachkommastellen; NaN erscheint als Gedankenstrich.
func formatStat(v float64) string {
	if math.IsNaN(v) {
		return "–"
	}
	return fmt.Sprintf("%.4f", v)
}

// WriteJSONReport schreibt den Bericht als eingerücktes JSON.
func WriteJSONReport(w io.Writer, report FitReport) error {
	report = prepareReport(report)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// Interne Hilfsfunktionen

// prepareReport sortiert Modelle und berechnet Rangfolge sowie Delta-AIC.
func prepareReport(report FitReport) FitReport {
	report.Models = append(make([]FitRow, 0, len(report.Models)), report.Models...)

	sort.Slice(report.Models, func(i, j int) bool {
		return report.Models[i].AIC < report.Models[j].AIC
	})

	if report.Candidates == 0 {
		report.Candidates = len(report.Models) + len(report.Failures)
	}

	if len(report.Models) > 0 {
		bestAIC := report.Models[0].AIC
		report.BestDistribution = report.Models[0].Name

		for i := range report.Models {
			report.Models[i].DeltaAIC =
				report.Models[i].AIC - bestAIC
		}
	}

	return report
}

// discardedSuffix hängt bei verworfenen Eingaben einen Hinweis an die Beobachtungszahl an.
func discardedSuffix(discarded int) string {
	if discarded == 0 {
		return ""
	}
	return fmt.Sprintf(" (verworfen: %d)", discarded)
}

// formatParams formatiert Modellparameter in stabiler, sortierter Reihenfolge.
func formatParams(params map[string]float64) string {
	if len(params) == 0 {
		return "-"
	}

	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(
			parts,
			fmt.Sprintf("%s=%.4g", key, params[key]),
		)
	}

	return strings.Join(parts, ", ")
}

// writeTable schreibt eine Tabelle mit an den Inhalt angepassten Spaltenbreiten.
func writeTable(
	w io.Writer,
	headers []string,
	rows [][]string,
) {
	widths := make([]int, len(headers))

	for i, header := range headers {
		widths[i] = utf8.RuneCountInString(header)
	}

	for _, row := range rows {
		for i, cell := range row {
			width := utf8.RuneCountInString(cell)
			if width > widths[i] {
				widths[i] = width
			}
		}
	}

	printBorder := func() {
		fmt.Fprint(w, "+")
		for _, width := range widths {
			fmt.Fprintf(w, "%s+", strings.Repeat("-", width+2))
		}
		fmt.Fprintln(w)
	}

	printRow := func(cells []string) {
		fmt.Fprint(w, "|")
		for i, cell := range cells {
			fmt.Fprintf(w, " %-*s |", widths[i], cell)
		}
		fmt.Fprintln(w)
	}

	printBorder()
	printRow(headers)
	printBorder()

	for _, row := range rows {
		printRow(row)
	}

	printBorder()
}

// getDistributionName liefert den Anzeigenamen einer Verteilung.
func getDistributionName(dist distribution.ContinuousDistribution) string {
	switch dist.(type) {
	case distribution.Weibull:
		return "Weibull"
	case distribution.LogNormal:
		return "LogNormal"
	case distribution.Gamma:
		return "Gamma"
	case distribution.Normal:
		return "Normal"
	case distribution.Beta:
		return "Beta"
	case distribution.Exponential:
		return "Exponential"
	case distribution.Gumbel:
		return "Gumbel"
	case distribution.Laplace:
		return "Laplace"
	case distribution.Rayleigh:
		return "Rayleigh"
	default:
		return "Unknown"
	}
}

// paramsToMap ordnet Verteilungsparameter ihren benannten Schlüsseln zu.
func paramsToMap(dist distribution.ContinuousDistribution) map[string]float64 {
	values := dist.Params()
	names := dist.ParamNames()
	m := make(map[string]float64, len(values))
	for i, p := range values {
		m[names[i]] = p
	}
	return m
}
