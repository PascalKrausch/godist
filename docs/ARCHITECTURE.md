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

