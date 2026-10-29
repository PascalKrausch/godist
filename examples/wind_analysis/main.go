package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/PascalKrausch/godist/pkg/distribution"
)

// OpenMeteoResponse spiegelt die JSON-Struktur von Open-Meteo wider
type OpenMeteoResponse struct {
	Hourly struct {
		Time         []string  `json:"time"`
		WindSpeed10m []float64 `json:"wind_speed_10m"`
	} `json:"hourly"`
}

func main() {
	file, err := os.Open("wind_2023_2025.json")
	if err != nil {
		fmt.Printf("Fehler beim Öffnen der JSON-Datei: %v\n", err)
		return
	}
	defer file.Close()

	var payload OpenMeteoResponse
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&payload); err != nil {
		panic(err)
	}

	// Daten bereinigen (ungültige Werte oder NaNs filtern)
	var data []float64
	for _, val := range payload.Hourly.WindSpeed10m {
		if distribution.IsFinite(val) && val >= 0 {
			data = append(data, val)
		}
	}

	fmt.Printf("Erfolgreich %d Winddaten-Punkte aus JSON eingelesen.\n", len(data))

	// Parallele Modell-Selektion ausführen
	results, failures, err := distribution.SelectAllModels(data)
	if err != nil {
		panic(err)
	}

	fmt.Printf("\n--- Fitting-Ergebnisse (JSON Input) ---\n")
	for _, res := range results {
		fmt.Printf("Verteilung: %-12s | AIC: %.2f | Parameter: %v\n",
			res.Dist.(distribution.Fitter).Name(), res.Metrics.AIC, res.Dist.Params())
	}

	if len(failures) > 0 {
		fmt.Printf("\nFehlgeschlagene Fits: %d\n", len(failures))
	}
}
