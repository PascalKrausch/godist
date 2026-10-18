package cli

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ReadOptions steuert, wie eine Datendatei gelesen wird.
type ReadOptions struct {
	// Column ist ein Spaltenname oder ein 0-basierter Index (nur CSV).
	// Ist Column leer, wird die einzige numerische Spalte verwendet; gibt es mehrere, schlägt das Einlesen fehl.
	Column string
	// Decimal ist das Dezimaltrennzeichen: "." (Standard) oder ",".
	Decimal string
}

// Discard beschreibt einen Eingabewert, der nicht in den Datensatz übernommen wurde.
type Discard struct {
	Location string
	Value    string
	Reason   string
}

// Dataset ist das Ergebnis des Einlesens. Values enthält nur übernommene Werte.
type Dataset struct {
	Values    []float64
	Column    string
	Discarded []Discard
}

// ReadDataset liest eine .txt-, .json- oder .csv-Datei und liefert die gültigen Werte
// zusammen mit allen verworfenen Eingaben.
func ReadDataset(path string, opts ReadOptions) (Dataset, error) {
	if opts.Decimal == "" {
		opts.Decimal = "."
	}

	ext := strings.ToLower(filepath.Ext(path))
	if opts.Column != "" && ext != ".csv" {
		return Dataset{}, fmt.Errorf("-column wird nur für CSV-Dateien unterstützt")
	}

	var ds Dataset
	var err error
	switch ext {
	case ".json":
		ds, err = readJSON(path)
	case ".txt":
		ds, err = readTXT(path, opts.Decimal)
	case ".csv":
		ds, err = readCSV(path, opts)
	default:
		return Dataset{}, fmt.Errorf("nicht unterstütztes Dateiformat: %q (erlaubt: .csv, .json, .txt)", ext)
	}
	if err != nil {
		return Dataset{}, err
	}

	if len(ds.Values) == 0 {
		return Dataset{}, fmt.Errorf("keine gültigen numerischen Werte gefunden (%d verworfen)", len(ds.Discarded))
	}
	return ds, nil
}

// WriteWarnings schreibt eine Zusammenfassung verworfener Werte, sofern vorhanden.
func (d Dataset) WriteWarnings(w io.Writer) {
	if len(d.Discarded) == 0 {
		return
	}

	const maxShown = 5
	total := len(d.Values) + len(d.Discarded)
	fmt.Fprintf(w, "Warnung: %d von %d Werten verworfen, %d verwendet\n", len(d.Discarded), total, len(d.Values))
	for i, discard := range d.Discarded {
		if i == maxShown {
			fmt.Fprintf(w, "  ... und %d weitere\n", len(d.Discarded)-maxShown)
			break
		}
		fmt.Fprintf(w, "  %s: %q (%s)\n", discard.Location, discard.Value, discard.Reason)
	}
}

func (d *Dataset) discard(location, value, reason string) {
	d.Discarded = append(d.Discarded, Discard{Location: location, Value: value, Reason: reason})
}

func readTXT(path, decimal string) (Dataset, error) {
	file, err := os.Open(path)
	if err != nil {
		return Dataset{}, err
	}
	defer file.Close()

	var ds Dataset
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		location := fmt.Sprintf("Zeile %d", line)
		if v, reason := parseCell(text, decimal); reason != "" {
			ds.discard(location, text, reason)
		} else {
			ds.Values = append(ds.Values, v)
		}
	}
	if err := scanner.Err(); err != nil {
		return Dataset{}, err
	}
	return ds, nil
}

func readJSON(path string) (Dataset, error) {
	file, err := os.Open(path)
	if err != nil {
		return Dataset{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	var items []any
	if err := decoder.Decode(&items); err != nil {
		return Dataset{}, fmt.Errorf("ungültiges JSON-Array: %w", err)
	}

	var ds Dataset
	for i, item := range items {
		location := fmt.Sprintf("Element %d", i+1)
		switch v := item.(type) {
		case json.Number:
			if value, reason := parseCell(v.String(), "."); reason != "" {
				ds.discard(location, v.String(), reason)
			} else {
				ds.Values = append(ds.Values, value)
			}
		default:
			ds.discard(location, fmt.Sprint(v), "kein Zahlenwert")
		}
	}
	return ds, nil
}

func readCSV(path string, opts ReadOptions) (Dataset, error) {
	file, err := os.Open(path)
	if err != nil {
		return Dataset{}, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return Dataset{}, fmt.Errorf("CSV-Datei konnte nicht gelesen werden: %w", err)
	}
	if len(records) == 0 {
		return Dataset{}, fmt.Errorf("CSV-Datei ist leer")
	}

	// Ein UTF-8-BOM (z. B. aus Excel) würde den ersten Spaltennamen verfälschen.
	records[0][0] = strings.TrimPrefix(records[0][0], "\ufeff")

	hasHeader := !rowHasNumber(records[0], opts.Decimal)
	start := 0
	var header []string
	if hasHeader {
		start = 1
		header = trimAll(records[0])
	}

	colIdx, colName, err := resolveColumn(records[start:], header, opts)
	if err != nil {
		return Dataset{}, err
	}

	var ds Dataset
	ds.Column = colName
	for i := start; i < len(records); i++ {
		row := records[i]
		location := fmt.Sprintf("Zeile %d", i+1)
		if colIdx >= len(row) {
			ds.discard(location, "", fmt.Sprintf("Spalte %d fehlt in dieser Zeile", colIdx))
			continue
		}
		cell := row[colIdx]
		if v, reason := parseCell(cell, opts.Decimal); reason != "" {
			ds.discard(location, strings.TrimSpace(cell), reason)
		} else {
			ds.Values = append(ds.Values, v)
		}
	}
	return ds, nil
}

// resolveColumn bestimmt den Index und Namen der zu lesenden CSV-Spalte.
func resolveColumn(rows [][]string, header []string, opts ReadOptions) (int, string, error) {
	if opts.Column != "" {
		if idx, err := strconv.Atoi(opts.Column); err == nil {
			if idx < 0 {
				return 0, "", fmt.Errorf("Spaltenindex darf nicht negativ sein: %d", idx)
			}
			if idx < len(header) {
				return idx, header[idx], nil
			}
			return idx, fmt.Sprintf("column_%d", idx), nil
		}

		if header == nil {
			return 0, "", fmt.Errorf("Spalte '%s' angegeben, aber die Datei hat keine Kopfzeile; bitte einen 0-basierten Index verwenden", opts.Column)
		}
		for i, name := range header {
			if name == opts.Column {
				return i, name, nil
			}
		}
		return 0, "", fmt.Errorf("Spalte '%s' nicht gefunden. Verfügbare Spalten: %s", opts.Column, strings.Join(header, ", "))
	}

	numeric := numericColumns(rows, opts.Decimal)
	switch len(numeric) {
	case 0:
		return 0, "", fmt.Errorf("keine numerische Spalte gefunden")
	case 1:
		idx := numeric[0]
		if idx < len(header) {
			return idx, header[idx], nil
		}
		return idx, fmt.Sprintf("column_%d", idx), nil
	default:
		names := make([]string, len(numeric))
		for i, idx := range numeric {
			if idx < len(header) {
				names[i] = header[idx]
			} else {
				names[i] = strconv.Itoa(idx)
			}
		}
		return 0, "", fmt.Errorf("mehrere numerische Spalten gefunden (%s); bitte mit -column eine Spalte wählen", strings.Join(names, ", "))
	}
}

// numericColumns liefert die Indizes aller Spalten, die mindestens einen gültigen Zahlenwert enthalten.
func numericColumns(rows [][]string, decimal string) []int {
	found := map[int]bool{}
	for _, row := range rows {
		for j, cell := range row {
			if _, reason := parseCell(cell, decimal); reason == "" {
				found[j] = true
			}
		}
	}

	var indices []int
	for j := range found {
		indices = append(indices, j)
	}
	sort.Ints(indices)
	return indices
}

func rowHasNumber(row []string, decimal string) bool {
	for _, cell := range row {
		if _, reason := parseCell(cell, decimal); reason == "" {
			return true
		}
	}
	return false
}

func trimAll(row []string) []string {
	out := make([]string, len(row))
	for i, cell := range row {
		out[i] = strings.TrimSpace(cell)
	}
	return out
}

// parseCell wandelt eine Eingabe in eine endliche Zahl um.
// Ist reason nicht leer, wurde der Wert nicht übernommen.
func parseCell(raw, decimal string) (value float64, reason string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, "leer"
	}
	if decimal == "," {
		s = strings.ReplaceAll(s, ",", ".")
	}

	v, err := strconv.ParseFloat(s, 64)
	switch {
	case errors.Is(err, strconv.ErrRange):
		return 0, "außerhalb des Zahlenbereichs"
	case err != nil:
		return 0, "nicht als Zahl lesbar"
	case math.IsNaN(v) || math.IsInf(v, 0):
		return 0, "NaN oder Unendlich"
	}
	return v, ""
}
