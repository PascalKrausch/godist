# godist — Statistische Verteilungen und numerische Parameterschätzung

`godist` ist eine in Go entwickelte Bibliothek und ein Kommandozeilenwerkzeug (CLI) zur Analyse stetiger Wahrscheinlichkeitsverteilungen. Es passt mehrere Verteilungsmodelle an einen gegebenen Datensatz an, vergleicht deren Güte und wählt standardmäßig das Modell mit dem niedrigsten Akaike Information Criterion (AIC).

> **Projektstatus: noch nicht produktiv verwenden.**
> Vor einem produktiven Einsatz müssen insbesondere Parameterschätzung, numerische Stabilität, Gütemaße, Eingabevalidierung und Testabdeckung systematisch überprüft werden.
>
> **Aktueller Stand:** Neun Verteilungen sind implementiert. Die Tests in `pkg/distribution`, `internal/cli` und `cmd/distfit` laufen grün (`go test ./...`). Der CLI-Prototyp verarbeitet reale Datensätze (z. B. `winequality-red.csv`). Nullwerte werden strikt behandelt (siehe [Methodik])

## Ziele

- Stetige Verteilungen über eine gemeinsame Go-Schnittstelle anbieten.
- Verteilungsparameter aus Messdaten schätzen.
- Mehrere Modelle vergleichbar fitten und nach AIC sortieren.
- Zusätzliche Kennzahlen wie BIC, KS-Statistik und RMSE bereitstellen.
- Das gleiche Analyseergebnis als menschenlesbaren Terminalbericht und maschinenlesbare JSON-Ausgabe bereitstellen.
- Die mathematische Bibliothek unabhängig von CLI, Dateiformaten und Darstellung halten.
- Korrektheit und Robustheit zuerst absichern; anschließend Laufzeit und GC-Allokationen messen und gezielt optimieren.

## Unterstützte Verteilungen

| Verteilung | Parameter | Träger |
|---|---|---|
| Beta | `alpha`, `beta`, `min`, `max` | [min, max] |
| Exponential | `rate` | [0, ∞) |
| Gauß / Normal | `mu`, `sigma` | ℝ |
| Gumbel | `mu`, `beta` | ℝ |
| Gamma | `alpha`, `scale` | [0, ∞) |
| Lognormal | `mu`, `sigma` | (0, ∞) |
| Weibull | `shape`, `scale` | [0, ∞) |
| Laplace | `mu`, `scale` | ℝ |
| Rayleigh | `scale` | [0, ∞) |


## Methodik

### Nullwerte (Fast-Fail-Policy)

Die Verteilungen mit positivem Träger behandeln Nullwerte derzeit **streng**: Ein Fit scheitert, statt Nullen stillschweigend zu verwerfen oder den Träger zu verschieben.

- **LogNormal:** scheitert bei jedem Nullwert, da `log(0)` nicht definiert ist.
- **Gamma und Weibull:** scheitern bei jedem Nullwert. Die Dichte an der Stelle 0 ist für Formparameter ungleich 1 entweder 0 oder unendlich. Für Formparameter genau 1 entspricht die Verteilung der Exponentialverteilung, die Nullwerte behält.
- **Rayleigh:** Nullen werden bei der Schätzung ignoriert, die Dichte an der Stelle 0 ist aber 0. Der Kandidat scheitert daher bei jedem Nullwert.
- **Exponential:** behält Nullwerte; die Dichte an der Stelle 0 ist endlich, der Kandidat ist also nicht betroffen.
- **Normal, Gumbel, Laplace:** behalten Nullwerte (`CleanDataWithZero`) und sind von dieser Regel nicht betroffen.
- **Beta:** Der Träger wird aus den Daten abgeleitet ( Ob Nullwerte scheitern, entscheidet die Dichteprüfung.

**Negative Werte** werden ebenfalls über die Trägerprüfung behandelt: Exponential, Gamma, Weibull, LogNormal und Rayleigh lehnen jeden negativen Wert mit `ErrNegativeValues` ab. Normal, Gumbel und Laplace sowie Beta (Träger aus den Daten) verarbeiten negative Werte. Alle Kandidaten werten so dieselbe Stichprobe aus.

Ein fehlgeschlagener Kandidat wird nicht gerankt. Die Nullzahl ist damit ein bewusst sichtbarer Befund und kein stiller Datenverlust.

Die Policy ist eine Zwischenlösung. Als Alternative existiert der Typ `ZeroInflated` (Mischung aus einer Punktmasse bei 0 und einer stetigen Verteilung auf (0, ∞)) mit `ZeroInflatedFitter{Base: ...}`. π₀ wird als Anteil der Nullen geschätzt, die Basisverteilung auf den positiven Werten. Der Fitter verlangt mindestens einen Nullwert und zählt π₀ als zusätzlichen Parameter in `k`. Die Modelle sind noch nicht in die Standardauswahl (`SelectAllModels`) aufgenommen, da sie auf einem gemischten Maß liegen und nur mit Modellen gleichen Maßes vergleichbar sind.

### Auswahlkriterium und Gütemaße

Die Standardauswahl erfolgt nach dem niedrigsten AIC:

\[
\mathrm{AIC} = 2k - 2\log L(\hat{\theta})
\]

Dabei ist `k` die Anzahl der tatsächlich frei geschätzten Parameter und `L` die Likelihood des angepassten Modells. Der Vergleich ist nur sinnvoll, wenn Modelle auf denselben Beobachtungen und mit konsistent definierter Likelihood verglichen werden.

Weitere aktuell ausgegebene Kennzahlen:

- **BIC**: zusätzliches Informationskriterium mit stärkerer Stichprobengrößenstrafe.
- **KS-Statistik**: maximaler Abstand zwischen empirischer und angepasster Verteilungsfunktion.
- **RMSE**: muss in der Dokumentation eindeutig als Fehlermaß der tatsächlich verglichenen Größen definiert werden.

Ein niedrigerer AIC bedeutet, dass ein Modell relativ zu den betrachteten Kandidaten bevorzugt wird. Er beweist nicht, dass das Modell die Daten absolut gut beschreibt oder dass die Daten tatsächlich aus dieser Verteilung stammen. Werden Parameter aus denselben Daten geschätzt, sind Standard-p-Werte beziehungsweise kritische Werte des Kolmogorow-Smirnow-Tests nicht ohne Weiteres anwendbar; bei Bedarf sollte eine geeignete Bootstrap-Kalibrierung geprüft werden.

Für kleine Stichproben kann ergänzend AICc untersucht werden. Dabei sind die Anwendbarkeit und die Regularitätsannahmen – insbesondere bei geschätzten Support-Grenzen – gesondert zu berücksichtigen.

## Erledigt

### Datenvalidierung und Edge-Cases

Getestet werden unter anderem 

- Leere Daten
- Konstante Daten
- NaN, Inf und 0.0
- Werte außerhalb des Supports
- Sehr kleine und große Werte
- Numerischer Über- und Unterlauf

### Tests

- Unit-Tests für alle bisher unterstützten Verteilungen 
- Eigenschaftstests (CDF monoton, PDF ≥ 0, LogPDF = log(PDF))
- Numerische Integrationstests
- Regressionstests
- Modellranking-Tests
- CLI-Tests
- CI-Pipeline integriert

### Performancemessungen und Optimierungen

- Benchmarks für alle Fitter und vollständigen Modellvergleich mit synthetischen Datensätzen
- Beta-Fitter von 82 allocs/op auf 70 allocs/ops optimiert
- Weibull von Bisektionsverfahren (konvergiert linear) auf Newton-Raphson (konvergiert quadratisch) umgestellt. Bisektion wird als Fallback beibehalten

## To-Do

### CLI und Einleselogik

- Unterbefehle fit und list distributions müssen noch implementiert werden
- Einzelne Verteilung erzwingen ist noch nicht möglich

### Weitere Verteilungen

- Logistic, LogLogistic, Student-t, Chi-Quadrat, GEV, Cauchy und Pareto sind weiterhin geplant

### Dokumentation

- README-Beispiele mit tatsächlich ausführbaren Befehlen synchron halten.
- Parameterkonventionen und Support jeder Verteilung dokumentieren.
- Grenzen von AIC, BIC, KS und RMSE erklären (siehe [methodik.md](methodik.md)).
- Versionierung und kompatible Änderungen der Bibliotheks-API festlegen.
- CI für Tests, Formatierung und gegebenenfalls statische Analyse ergänzen.
- Erst nach bestandenen Validierungs- und Regressionstests einen produktiven Einsatz empfehlen.



## Schnellstart

```bash
# Bauen
go build -o distfit ./cmd/distfit

# Alle Kandidaten vergleichen (Text oder JSON)
./distfit -file=measurements.txt -all
./distfit -file=measurements.txt -all -format=json
./distfit -file=measurements.txt -all -format=plot

# Tests und Benchmarks
go test ./...
go test -bench=. -benchmem ./...
```

## Dokumentation

| Thema | Datei |
|---|---|
| Architektur, Verzeichnisstruktur und Kerninterfaces | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| CLI: Flags, Eingabeformate, Mindeststichprobe, Zielinterface | [docs/CLI.md](docs/CLI.md) |
| JSON-Ausgabe und Einbindung in Go, Python, TypeScript | [docs/JSON.md](docs/JSON.md) |
| Evaluation: Vergleich mit SciPy, Benchmarks | [docs/EVALUATION.md](docs/EVALUATION.md) |
| Roadmap und nächste Schritte | [docs/roadmap.md](docs/roadmap.md) |

## Grundsatz

`godist` soll nicht einfach die größtmögliche Zahl von Verteilungen anbieten, sondern für jeden Kandidaten nachvollziehbare und numerisch belastbare Fits liefern. **Korrektheit, Diagnose und Reproduzierbarkeit haben Vorrang vor der Zahl der Verteilungen und vor Mikrooptimierungen.**
