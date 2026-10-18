package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/PascalKrausch/godist/internal/cli"
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
	format := fs.String("format", "text", "Ausgabeformat: text oder json (json erfordert -all)")
	decimal := fs.String("decimal", ".", "Dezimaltrennzeichen: . oder ,")

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
	if *allModels {
		return runAll(stdout, stderr, ds, *filePath, *format)
	}
	return runBest(stdout, stderr, ds)
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
	if format != "text" && format != "json" {
		return fmt.Errorf("ungültiges -format %q (erlaubt: text, json)", format)
	}
	if format == "json" && !allModels {
		return errors.New("-format=json erfordert -all")
	}
	if decimal != "." && decimal != "," {
		return fmt.Errorf("ungültiges -decimal %q (erlaubt: . oder ,)", decimal)
	}
	return nil
}

// runAll vergleicht alle Verteilungen und gibt den Bericht aus.
func runAll(stdout, stderr io.Writer, ds cli.Dataset, filePath, format string) int {
	results, failures, err := distribution.SelectAllModels(ds.Values)
	if err != nil {
		fmt.Fprintf(stderr, "Fehler: Fitting fehlgeschlagen: %v\n", err)
		return exitFailure
	}

	report := cli.BuildFitReport(results, failures, filePath, ds.Column, len(ds.Values), len(ds.Discarded))

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

// runBest gibt nur das beste Modell aus.
func runBest(stdout, stderr io.Writer, ds cli.Dataset) int {
	best, metrics, err := distribution.SelectBestModel(ds.Values)
	if err != nil {
		fmt.Fprintf(stderr, "Fehler: Fitting fehlgeschlagen: %v\n", err)
		return exitFailure
	}
	cli.WriteSummary(stdout, best, metrics, ds.Column, len(ds.Values), len(ds.Discarded))
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
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Exit-Codes: 0 Erfolg, 1 Daten- oder Fitting-Fehler, 2 Aufruf-Fehler")
}
