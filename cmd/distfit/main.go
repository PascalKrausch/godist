package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"godist/internal/cli"
	"godist/pkg/distribution"
)

func main() {
	filePath := flag.String("file", "", "Pfad zur Datei (.csv, .json, .txt)")
	allModels := flag.Bool("all", false, "Zeige alle Modelle statt nur das beste")
	format := flag.String("format", "text", "Ausgabeformat: text oder json")
	flag.Parse()

	if *filePath == "" {
		log.Fatal("Bitte einen Dateipfad angeben: -file=daten.json")
	}

	data, err := readData(*filePath)
	if err != nil {
		log.Fatalf("Fehler beim Einlesen der Daten: %v", err)
	}

	if *allModels {
		results, failures, err := distribution.SelectAllModels(data)
		if err != nil {
			log.Fatalf("Fitting fehlgeschlagen: %v", err)
		}

		report := cli.BuildFitReport(results, failures, *filePath, len(data))

		if *format == "json" {
			if err := cli.WriteJSONReport(os.Stdout, report); err != nil {
				log.Fatalf("Report-Erstellung fehlgeschlagen: %v", err)
			}
		} else {
			if err := cli.WriteTextReport(os.Stdout, report); err != nil {
				log.Fatalf("Report-Erstellung fehlgeschlagen: %v", err)
			}
		}
	} else {
		bestDist, metrics, err := distribution.SelectBestModel(data)
		if err != nil {
			log.Fatalf("Fitting fehlgeschlagen: %v", err)
		}
		fmt.Println("=== GODIST FITTER ERGEBNIS ===")
		fmt.Printf("Datenpunkte:    %d (verworfen: %d)\n", metrics.SampleSize, metrics.Discarded)
		fmt.Printf("Beste Kurve:    %T\n", bestDist)
		fmt.Printf("Parameter:      %v\n", bestDist.Params())
		fmt.Printf("AIC / BIC:      %.2f / %.2f\n", metrics.AIC, metrics.BIC)
		fmt.Printf("KS-Statistik:   %.4f (RMSE: %.4f)\n", metrics.KSStatistic, metrics.RMSE)
	}
}

// readData entscheidet anhand der Endung, wie die Datei gelesen wird
func readData(path string) ([]float64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		var data []float64
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&data); err != nil {
			return nil, fmt.Errorf("ungültiges JSON-Array: %w", err)
		}
		return data, nil

	case ".txt":
		var data []float64
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			text := strings.TrimSpace(scanner.Text())
			if text == "" {
				continue
			}
			val, err := strconv.ParseFloat(text, 64)
			if err != nil {
				// Ignoriere ungültige Zeilen oder liefere einen Fehler
				continue
			}
			data = append(data, val)
		}
		return data, scanner.Err()

	case ".csv":
		reader := csv.NewReader(file)
		records, err := reader.ReadAll()
		if err != nil {
			return nil, err
		}
		var data []float64
		for _, row := range records {
			for _, cell := range row {
				if val, err := strconv.ParseFloat(strings.TrimSpace(cell), 64); err == nil {
					data = append(data, val)
				}
			}
		}
		return data, nil

	default:
		return nil, fmt.Errorf("nicht unterstütztes Dateiformat: %s", ext)
	}
}
