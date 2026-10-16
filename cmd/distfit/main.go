package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
	column := flag.String("column", "", "Spaltenname oder Index (0-basiert) für CSV-Dateien")
	allModels := flag.Bool("all", false, "Zeige alle Modelle statt nur das beste")
	format := flag.String("format", "text", "Ausgabeformat: text oder json")
	flag.Parse()

	if *filePath == "" {
		log.Fatal("Bitte einen Dateipfad angeben: -file=daten.json")
	}

	data, columnName, err := readData(*filePath, *column)
	if err != nil {
		log.Fatalf("Fehler beim Einlesen der Daten: %v", err)
	}

	if *allModels {
		results, failures, err := distribution.SelectAllModels(data)
		if err != nil {
			log.Fatalf("Fitting fehlgeschlagen: %v", err)
		}

		report := cli.BuildFitReport(results, failures, *filePath, columnName, len(data))

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
		if columnName != "" {
			fmt.Printf("Spalte:         %s\n", columnName)
		}
		fmt.Printf("Datenpunkte:    %d (verworfen: %d)\n", metrics.SampleSize, metrics.Discarded)
		fmt.Printf("Beste Kurve:    %T\n", bestDist)
		fmt.Printf("Parameter:      %v\n", bestDist.Params())
		fmt.Printf("AIC / BIC:      %.2f / %.2f\n", metrics.AIC, metrics.BIC)
		fmt.Printf("KS-Statistik:   %.4f (RMSE: %.4f)\n", metrics.KSStatistic, metrics.RMSE)
	}
}

// readData entscheidet anhand der Endung, wie die Datei gelesen wird
func readData(path string, column string) ([]float64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		var data []float64
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&data); err != nil {
			return nil, "", fmt.Errorf("ungültiges JSON-Array: %w", err)
		}
		return data, "", nil

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
		return data, "", scanner.Err()

	case ".csv":
		buf := make([]byte, 200)
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return nil, "", err
		}
		fmt.Printf("Dateianfang: %q\n", buf[:n])

		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return nil, "", err
		}
		reader := csv.NewReader(file)
		reader.Comma = ';'
		reader.LazyQuotes = false
		reader.TrimLeadingSpace = true
		records, err := reader.ReadAll()
		fmt.Printf("Header: %#v\n", records[0])
		fmt.Printf("Erste Datenzeile: %#v\n", records[1])
		if err != nil {
			return nil, "", fmt.Errorf("Fehler beim Lesen der CSV-Datei: %w", err)
		}

		if len(records) == 0 {
			return nil, "", fmt.Errorf("CSV-Datei ist leer")
		}

		// Spaltenauswahl
		var columnIndex int
		var columnName string
		var hasHeader bool

		if column != "" {
			// Prüfen, ob es ein numerischer Index ist
			if idx, err := strconv.Atoi(column); err == nil {
				// Index-basierte Auswahl
				if idx < 0 {
					return nil, "", fmt.Errorf("Spaltenindex darf nicht negativ sein: %d", idx)
				}
				columnIndex = idx
				columnName = fmt.Sprintf("column_%d", idx)
			} else {
				// Namensbasierte Auswahl
				header := records[0]
				found := false
				for i, name := range header {
					if strings.TrimSpace(name) == column {
						columnIndex = i
						columnName = column
						found = true
						break
					}
				}
				if !found {
					// Verfügbare Spalten auflisten
					available := make([]string, 0, len(header))
					for _, name := range header {
						available = append(available, strings.TrimSpace(name))
					}
					return nil, "", fmt.Errorf("Spalte '%s' nicht gefunden. Verfügbare Spalten: %s", column, strings.Join(available, ", "))
				}
				hasHeader = true
			}
		}

		// Daten extrahieren
		var data []float64
		startRow := 0
		if hasHeader {
			startRow = 1
		}

		for i := startRow; i < len(records); i++ {
			row := records[i]
			if column != "" {
				// Spezifische Spalte
				if columnIndex >= len(row) {
					return nil, "", fmt.Errorf("Zeile %d hat nicht genug Spalten (erwartete Spalte %d, gefunden %d)", i+1, columnIndex, len(row))
				}
				cell := strings.TrimSpace(row[columnIndex])
				if cell == "" {
					continue
				}
				val, err := strconv.ParseFloat(cell, 64)
				if err != nil {
					continue
				}
				data = append(data, val)
			} else {
				// Alle Zellen (altes Verhalten)
				for _, cell := range row {
					if val, err := strconv.ParseFloat(strings.TrimSpace(cell), 64); err == nil {
						data = append(data, val)
					}
				}
			}
		}

		if len(data) == 0 {
			return nil, "", fmt.Errorf("keine gültigen numerischen Daten in Spalte '%s' gefunden", columnName)
		}

		return data, columnName, nil

	default:
		return nil, "", fmt.Errorf("nicht unterstütztes Dateiformat: %s", ext)
	}
}
