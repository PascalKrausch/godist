# Architektur

```text
.
├── cmd/
│   └── distfit/
│       └── main.go           # CLI-Einstiegspunkt
├── evaluation/
│   ├── bench.py              # Benchmarks godist vs. SciPy (Zeit, Speicher)
│   └── compare.py            # Vergleich der Fits mit SciPy
├── internal/
│   └── cli/
│       └── report.go         # Terminal- und JSON-Bericht
│   └── stats/
│       └── stats.go          # Deskriptive Kennzahlen (Mittelwert, Quantile, Schiefe, Kurtosis)
├── pkg/
│   └── distribution/
│       ├── beta.go
│       ├── data_cleaning.go
│       ├── distributions_test.go
│       ├── exponential.go
│       ├── fitter.go
│       ├── fitter_test.go
│       ├── fitters_test.go
│       ├── gamma.go
│       ├── gaussian.go
│       ├── gumbel.go
│       ├── laplace.go
│       ├── log_normal.go
│       ├── numerical_stability_test.go
│       ├── rayleigh.go
│       ├── regression_test.go
│       ├── requirements_test.go
│       ├── types.go
│       └── weibull.go
├── go.mod
└── go.sum
```

Die CLI-Schicht in `cmd/` und `internal/cli/` soll die Verteilungsbibliothek aufrufen, aber keine Verteilungsformeln oder Schätzverfahren implementieren. Das Package `pkg/distribution` soll ohne Kommandozeilenabhängigkeiten verwendbar bleiben.

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

- `internal/plot` berechnet die Diagrammdaten (`BuildPayloadWithBins`): Histogramm, ECDF, PDF- und CDF-Kurven der Modelle auf einem gemeinsamen Raster sowie Q-Q-Punkte. Die Darstellung findet dort nicht statt. Die deskriptiven Kennzahlen kommen aus `internal/stats` (`Describe`) und stehen im Feld `descriptive`.
- `internal/plotserver` bettet das Frontend (`web/`, uPlot) per `go:embed` ein und liefert es zusammen mit der JSON-API aus. Anfragen mit fremdem Host-Header werden abgelehnt.
- Die Datenquelle wird pro Anfrage aufgerufen (`plotserver.Source`), damit die Klassenanzahl im Browser geändert werden kann, ohne den Server neu zu starten.

API: `GET /api/plot` liefert das Payload als JSON. Der optionale Parameter `bins` (1 bis 500) setzt die Klassenanzahl des Histogramms; ohne Parameter wird sie nach Freedman-Diaconis gewählt. Ungültige Werte liefern `400 Bad Request`.

