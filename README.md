## godist - Statistische Verteilungen und numerische Parameterschätzung

# Ziel des Projekts: 

CLI-Tool und Bibliothek für die Analyse stetiger statistischer Verteilungen, geschrieben in Go. godist soll für beliebige Daten den optimalsten Fit + Gütemaße zurückgeben und verschiedene Verteilungen vergleichen.

## Achtung: Noch nicht produktiv verwendbar!

# Projektarchitektur:

├── cmd
│   └── distfit
│       └── main.go
├── go.mod
├── go.sum
├── pkg
│   └── distribution
│       ├── beta.go
│       ├── data_cleaning.go
│       ├── exponential.go
│       ├── fitter.go
│       ├── gamma.go
│       ├── gaussian.go
│       ├── gumbel.go
│       ├── log_normal.go
│       ├── README.md
│       ├── types.go
│       └── weibull.go

Mögliche Erweiterungen der unterstützten Verteilungen könnten sein: 

* Log-Logistik
* Chi-Quadrat
* Student-t
* Cauchy
* Pareto
* Laplace
* Rayleigh
* GEV

Diese können einfach über das ContinousDistribution-Interface implementiert werden: 

// ContinuousDistribution beschreibt eine kontinuierliche Wahrscheinlichkeitsverteilung.
type ContinuousDistribution interface {
	PDF(x float64) float64
	CDF(x float64) float64
	LogPDF(x float64) float64
	Params() []float64
}

// Fitter beschreibt eine Schnittstelle zur Parameterschätzung für Verteilungen.
type Fitter interface {
	Fit(data []float64) (ContinuousDistribution, error)
	Name() string
}

Auswahlkriterium des "besten" Fits ist "niedrigstes AIC"

Die Arbeit am CLI-Tool steht noch am Anfang, am Ende soll die Terminalausgabe in etwa wie folgt aussehen:

GODIST — Distribution Analysis

Input:       measurements.csv
Column:      wind_speed
Observations: 1,248
Candidates:  7
Criterion:   AIC (lower is better)

Best fit
--------
Distribution: Weibull
Parameters:   shape = 2.14
              scale = 7.83

Model comparison
------------------------------------------------
Rank  Distribution  Parameters       AIC   ΔAIC
1     Weibull       k=2.14, λ=7.83  821.4   0.0
2     Gamma         α=4.20, θ=1.86  824.1   2.7
3     Lognormal     μ=1.71, σ=0.49  829.8   8.4
------------------------------------------------

Analysis completed.

Folgende Befehle sollen zunächst implementiert werden:

godist fit measurements.csv --column wind_speed_10m - Soll die obige Ausgabe reproduzieren

godist list distributions - listet alle Verteilungen samt Parametern auf

godist fit measurements.csv --column wind_speed_10m -distribution weibull - erzwingt Weibull 

Später soll noch eine Ausgabe als JSON implementiert werden

Zusätzlich müssen noch umfangreiche Unit-Tests geschrieben werden, Performance Messungen durchgeführt werden und GC-Allokationen minimiert werden