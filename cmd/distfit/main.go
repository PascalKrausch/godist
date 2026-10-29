package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/PascalKrausch/godist/internal/cli"
	"github.com/PascalKrausch/godist/internal/plotserver"
	"github.com/PascalKrausch/godist/internal/stats"
	"github.com/PascalKrausch/godist/pkg/distribution"
)

const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run führt die CLI aus und liefert den Exit-Code.
// Ergebnisse gehen nach stdout, Warnungen und Fehler nach stderr.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("distfit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { writeUsage(stderr, fs) }

	filePath := fs.String("file", "", "Pfad zur Datei (.csv, .json, .txt)")
	column := fs.String("column", "", "CSV: Spaltenname oder 0-basierter Index (ohne Angabe: einzige numerische Spalte)")
	allModels := fs.Bool("all", false, "Alle Modelle vergleichen statt nur das beste anzuzeigen")
	format := fs.String("format", "text", "Ausgabeformat: text, json oder plot (json und plot erfordern -all)")
	decimal := fs.String("decimal", ".", "Dezimaltrennzeichen: . oder ,")
	zeroInflated := fs.Bool("zero-inflated", false, "Nullwerte als Zero-Inflated-Modell (Punktmasse bei 0) modellieren; nur ZI-Kandidaten werden verglichen (Standard: Nullwerte lassen Kandidaten mit Träger (0, ∞) scheitern)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}

	if err := validateFlags(fs, *filePath, *column, *format, *decimal, *allModels); err != nil {
		fmt.Fprintf(stderr, "Fehler: %v\n\n", err)
		fs.Usage()
		return exitUsage
	}

	ds, err := cli.ReadDataset(*filePath, cli.ReadOptions{Column: *column, Decimal: *decimal})
	if err != nil {
		fmt.Fprintf(stderr, "Fehler beim Einlesen von %s: %v\n", *filePath, err)
		return exitFailure
	}
	ds.WriteWarnings(stderr)

	warning, err := cli.ValidateSample(ds.Values)
	if err != nil {
		fmt.Fprintf(stderr, "Fehler: %v\n", err)
		return exitFailure
	}
	if warning != "" {
		fmt.Fprintf(stderr, "Warnung: %s\n", warning)
	}
	policy := distribution.PolicyFastFail
	if *zeroInflated {
		policy = distribution.PolicyZeroInflated
	}
	if *allModels {
		return runAll(stdout, stderr, ds, *filePath, *format, policy)
	}
	return runBest(stdout, stderr, ds, policy)
}

// validateFlags prüft Flag-Kombinationen, bevor Daten gelesen werden.
func validateFlags(fs *flag.FlagSet, filePath, column, format, decimal string, allModels bool) error {
	if fs.NArg() > 0 {
		return fmt.Errorf("unerwartete Argumente: %s", strings.Join(fs.Args(), " "))
	}
	if filePath == "" {
		return errors.New("bitte eine Datei angeben: -file=daten.csv")
	}
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".csv", ".json", ".txt":
	default:
		return fmt.Errorf("nicht unterstütztes Dateiformat für %s (erlaubt: .csv, .json, .txt)", filePath)
	}
	if column != "" && strings.ToLower(filepath.Ext(filePath)) != ".csv" {
		return errors.New("-column wird nur für CSV-Dateien unterstützt")
	}
	switch format {
	case "text":
	case "json", "plot":
		if !allModels {
			return fmt.Errorf("-format=%s erfordert -all", format)
		}
	default:
		return fmt.Errorf("ungültiges -format %q (erlaubt: text, json, plot)", format)
	}
	if decimal != "." && decimal != "," {
		return fmt.Errorf("ungültiges -decimal %q (erlaubt: . oder ,)", decimal)
	}
	return nil
}

// runAll vergleicht alle Verteilungen und gibt den Bericht aus.
func runAll(stdout, stderr io.Writer, ds cli.Dataset, filePath, format string, policy distribution.ZeroPolicy) int {
	results, failures, err := distribution.SelectAllModelsWithPolicy(ds.Values, policy)
	if err != nil {
		fmt.Fprintf(stderr, "Fehler: Fitting fehlgeschlagen: %v\n", err)
		return exitFailure
	}

	if format == "plot" {
		return runPlot(stderr, ds.Values, results)
	}

	report := cli.BuildFitReport(results, failures, filePath, ds.Column, stats.Describe(ds.Values), len(ds.Discarded))

	if format == "json" {
		if err := cli.WriteJSONReport(stdout, report); err != nil {
			fmt.Fprintf(stderr, "Fehler: JSON-Ausgabe fehlgeschlagen: %v\n", err)
			return exitFailure
		}
	} else if err := cli.WriteTextReport(stdout, report); err != nil {
		fmt.Fprintf(stderr, "Fehler: Report-Erstellung fehlgeschlagen: %v\n", err)
		return exitFailure
	}

	if len(results) == 0 {
		fmt.Fprintln(stderr, "Fehler: Kein Modell konnte angepasst werden.")
		return exitFailure
	}
	return exitOK
}

// runPlot startet das interaktive Dashboard, öffnet den Browser und blockiert bis Strg+C.
func runPlot(stderr io.Writer, values []float64, results []distribution.FitResult) int {
	if len(results) == 0 {
		fmt.Fprintln(stderr, "Fehler: Kein Modell konnte angepasst werden.")
		return exitFailure
	}

	source := cli.BuildPlotSource(values, results)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := plotserver.Serve(ctx, source, func(url string) {
		fmt.Fprintf(stderr, "Dashboard: %s\nBeenden mit Strg+C.\n", url)
		if err := cli.OpenBrowser(url); err != nil {
			fmt.Fprintf(stderr, "Warnung: Browser konnte nicht geöffnet werden: %v\n", err)
		}
	})
	if err != nil {
		fmt.Fprintf(stderr, "Fehler: Dashboard fehlgeschlagen: %v\n", err)
		return exitFailure
	}
	return exitOK
}

// runBest gibt nur das beste Modell aus.
func runBest(stdout, stderr io.Writer, ds cli.Dataset, policy distribution.ZeroPolicy) int {
	best, metrics, err := distribution.SelectBestModelWithPolicy(ds.Values, policy)
	if err != nil {
		fmt.Fprintf(stderr, "Fehler: Fitting fehlgeschlagen: %v\n", err)
		return exitFailure
	}
	cli.WriteSummary(stdout, best, metrics, ds.Column, stats.Describe(ds.Values), len(ds.Discarded))
	return exitOK
}

func writeUsage(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprintln(w, "Verwendung: distfit -file=<datei> [optionen]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Passt Verteilungen an Daten an und vergleicht sie nach AIC.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Optionen:")
	fs.PrintDefaults()
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Beispiele:")
	fmt.Fprintln(w, "  distfit -file=werte.txt")
	fmt.Fprintln(w, "  distfit -file=daten.csv -column=density -all")
	fmt.Fprintln(w, "  distfit -file=daten.csv -column=2 -decimal=, -all -format=json")
	fmt.Fprintln(w, "  distfit -file=werte.txt -all -format=plot")
	fmt.Fprintln(w, "  distfit -file=daten.csv -column=citric -zero-inflated -all")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Exit-Codes: 0 Erfolg, 1 Daten- oder Fitting-Fehler, 2 Aufruf-Fehler")
}
