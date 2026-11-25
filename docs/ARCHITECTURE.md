# Architektur

## Überblick

```text
cmd/distfit/          CLI-Einstiegspunkt
internal/cli/         Kommandos, Eingabe, Terminal- und JSON-Bericht
internal/stats/       Deskriptive Kennzahlen
internal/plot/        Diagrammdaten (ohne Darstellung)
internal/plotserver/  HTTP-Server, JSON-API, eingebettetes Frontend
pkg/distribution/     Verteilungen, Schätzverfahren, Modellauswahl
evaluation/           Vergleich und Benchmarks gegen SciPy (Python)
examples/             Beispielprogramme
```

Abhängigkeiten zeigen nur nach unten: `cmd` → `internal` → `pkg`. `pkg/distribution` importiert keine Pakete aus `internal/` oder `cmd/` und bleibt ohne Kommandozeilenabhängigkeit verwendbar.

## Datenfluss

1. `cmd/distfit` startet ein Kommando aus `internal/cli`.
2. `internal/cli` liest und bereinigt die Eingabedaten.
3. `pkg/distribution` wählt das Modell. Die Kandidaten kommen aus `Catalog()`; jeder Eintrag ist ein `Fitter`.
4. Für die Ausgabe berechnet `internal/stats` die Kennzahlen und `internal/plot` die Diagrammdaten.
5. `internal/plotserver` liefert die Diagrammdaten als JSON an den Browser.

## Neue Verteilung hinzufügen

1. Typ implementieren, der `ContinuousDistribution` erfüllt, und einen `Fitter` dafür schreiben.
2. Eintrag in `Catalog()` (`pkg/distribution/catalog.go`) mit Name, Parametern, Wertebereich und `Fitter` ergänzen. Die Parameternamen müssen in derselben Reihenfolge stehen wie `ParamNames()`.

## Kerninterfaces

```go
// ContinuousDistribution beschreibt eine stetige Wahrscheinlichkeitsverteilung.
type ContinuousDistribution interface {
    PDF(x float64) float64
    CDF(x float64) float64
    LogPDF(x float64) float64
    Quantile(p float64) float64 // NaN für p außerhalb [0, 1] oder ungültige Parameter
    Params() []float64
    ParamNames() []string
}

// Fitter schätzt die Parameter einer Verteilung aus Daten.
type Fitter interface {
    Fit(data []float64) (ContinuousDistribution, error)
    Name() string
}
```

`Params() []float64` liefert die Werte, `ParamNames() []string` die zugehörigen Namen in gleicher Reihenfolge (z. B. `shape`, `scale` bei Weibull, `rate` bei Exponential). Die CLI nutzt diese Namen für die Ausgabe.

## Plot-Schicht

- `internal/plot` berechnet die Diagrammdaten (`BuildPayloadWithBins`): Histogramm, KDE (Gauss-Kernel mit Silverman-Bandbreite, skaliert über einen Faktor), ECDF, PDF- und CDF-Kurven der Modelle auf einem gemeinsamen Raster sowie Q-Q-Punkte. Die Darstellung findet dort nicht statt. Die deskriptiven Kennzahlen kommen aus `internal/stats` (`Describe`) und stehen im Feld `descriptive`.
- `internal/plotserver` bettet das Frontend (`internal/plotserver/web/`, uPlot) per `go:embed` ein und liefert es zusammen mit der JSON-API aus. Anfragen mit fremdem Host-Header werden abgelehnt.
- Die Datenquelle wird pro Anfrage aufgerufen (`plotserver.Source`), damit die Klassenanzahl im Browser geändert werden kann, ohne den Server neu zu starten.

API: `GET /api/plot` liefert das Payload als JSON. Der optionale Parameter `bins` (1 bis 500) setzt die Klassenanzahl des Histogramms; ohne Parameter wird sie nach Freedman-Diaconis gewählt. Der optionale Parameter `kde` (0 < F ≤ 20) setzt den KDE-Glättungsfaktor relativ zur Silverman-Bandbreite; ohne Parameter gilt Faktor 1. Ungültige Werte liefern `400 Bad Request`.

