// Payloaddump gibt die Diagrammdaten des Dashboards als JSON aus, ohne Server und Browser.
// Dient dem Vergleich mit SciPy in evaluation/compare_plot.py.
//
// Aufruf aus dem Repository-Wurzelverzeichnis:
//
//	go run ./evaluation/payloaddump -file=daten.csv -column=x [-bins=0]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/PascalKrausch/godist/internal/cli"
	"github.com/PascalKrausch/godist/pkg/distribution"
)

func main() {
	filePath := flag.String("file", "", "Pfad zur Datei")
	column := flag.String("column", "", "CSV: Spaltenname oder 0-basierter Index")
	bins := flag.Int("bins", 0, "Klassenanzahl (0 = automatisch)")
	flag.Parse()

	if *filePath == "" {
		fmt.Fprintln(os.Stderr, "bitte -file angeben")
		os.Exit(2)
	}

	ds, err := cli.ReadDataset(*filePath, cli.ReadOptions{Column: *column})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fehler: %v\n", err)
		os.Exit(1)
	}

	results, _, err := distribution.SelectAllModels(ds.Values)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fehler: %v\n", err)
		os.Exit(1)
	}

	payload := cli.BuildPlotSource(ds.Values, results)(*bins)
	if err := json.NewEncoder(os.Stdout).Encode(payload); err != nil {
		fmt.Fprintf(os.Stderr, "Fehler: %v\n", err)
		os.Exit(1)
	}
}
