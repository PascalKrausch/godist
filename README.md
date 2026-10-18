# godist — Statistische Verteilungen und numerische Parameterschätzung

`godist` ist eine in Go entwickelte Bibliothek und ein Kommandozeilenwerkzeug (CLI) zur Analyse stetiger Wahrscheinlichkeitsverteilungen. Es passt mehrere Verteilungsmodelle an einen gegebenen Datensatz an, vergleicht deren Güte und wählt standardmäßig das Modell mit dem niedrigsten Akaike Information Criterion (AIC).

> **Projektstatus: noch nicht produktiv verwenden.**  
> Vor einem produktiven Einsatz müssen insbesondere Parameterschätzung, numerische Stabilität, Gütemaße, Eingabevalidierung und Testabdeckung systematisch überprüft werden.
>
> **Aktueller Stand:** Neun Verteilungen sind implementiert. Die Tests in `pkg/distribution`, `internal/cli` und `cmd/distfit` laufen grün (`go test ./...`). Der CLI-Prototyp verarbeitet reale Datensätze (z. B. `winequality-red.csv`). Die Behandlung von Nullwerten folgt einer strikten Fast-Fail-Policy (siehe [Nullwerte](#nullwerte-fast-fail-policy)). Offene Punkte stehen unter [Bekannte Probleme](#bekannte-probleme-stand-aktueller-prototyp).

## Ziele

- Stetige Verteilungen über eine gemeinsame Go-Schnittstelle anbieten.
- Verteilungsparameter aus Messdaten schätzen.
- Mehrere Modelle vergleichbar fitten und nach AIC sortieren.
- Zusätzliche Kennzahlen wie BIC, KS-Statistik und RMSE bereitstellen.
- Das gleiche Analyseergebnis als menschenlesbaren Terminalbericht und maschinenlesbare JSON-Ausgabe bereitstellen.
- Die mathematische Bibliothek unabhängig von CLI, Dateiformaten und Darstellung halten.
- Korrektheit und Robustheit zuerst absichern; anschließend Laufzeit und GC-Allokationen messen und gezielt optimieren.

## Aktuell unterstützte Verteilungen

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

Weibull ist eine Zweiparameter-Verteilung. Beim Beta-Fit werden nur `alpha` und `beta` geschätzt; `min` und `max` werden aus den Daten übernommen (siehe [Bekannte Probleme](#bekannte-probleme-stand-aktueller-prototyp)).


## Architektur

```text
.
├── cmd/
│   └── distfit/
│       └── main.go           # CLI-Einstiegspunkt
├── evaluation/
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

### Kerninterfaces

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

## Nullwerte (Fast-Fail-Policy)

Die Verteilungen mit positivem Träger behandeln Nullwerte derzeit **streng**: Ein Fit scheitert, statt Nullen stillschweigend zu verwerfen oder den Träger zu verschieben.

- **LogNormal:** scheitert bei jedem Nullwert, da `log(0)` nicht definiert ist.
- **Gamma und Weibull:** scheitern bei Nullwerten, wenn der Formparameter kleiner als 1 ist. Bei Formparametern größer als 1 ist die Dichte an der Stelle 0 gleich 0 (`LogPDF(0) = −∞`), und der Kandidat scheitert bei der Likelihood-Berechnung. Bei Formparameter genau 1 bleibt der Kandidat gültig.
- **Rayleigh:** Nullen werden bei der Schätzung ignoriert, die Dichte an der Stelle 0 ist aber 0. Der Kandidat scheitert daher bei jedem Nullwert.
- **Exponential:** behält Nullwerte; die Dichte an der Stelle 0 ist endlich, der Kandidat ist also nicht betroffen.
- **Normal, Gumbel, Laplace:** behalten Nullwerte (`CleanDataWithZero`) und sind von dieser Regel nicht betroffen.
- **Beta:** Der Träger wird aus den Daten abgeleitet (siehe [Bekannte Probleme](#bekannte-probleme-stand-aktueller-prototyp)). Ob Nullwerte scheitern, entscheidet die Dichteprüfung.

**Negative Werte** werden ebenfalls über die Trägerprüfung behandelt: Exponential, Gamma, Weibull, LogNormal und Rayleigh lehnen jeden negativen Wert mit `ErrNegativeValues` ab. Normal, Gumbel und Laplace sowie Beta (Träger aus den Daten) verarbeiten negative Werte. Alle Kandidaten werten so dieselbe Stichprobe aus.

Ein fehlgeschlagener Kandidat wird nicht gerankt. Die Nullzahl ist damit ein bewusst sichtbarer Befund und kein stiller Datenverlust.

Die Policy ist eine Zwischenlösung. Später kann ein **Zero-Inflated-Modell** (Mischung aus einer Punktmasse bei 0 und einer stetigen Verteilung für positive Werte) in Betracht kommen. Dieses würde die Nullen als eigenen Mechanismus modellieren und die Vergleichbarkeit mit reinen stetigen Modellen gesondert betrachten.

## Aktueller CLI-Prototyp

Der Prototyp liest Dateien und wählt standardmäßig das beste Modell nach AIC aus.

| Flag | Bedeutung |
|------|-----------|
| `-file` | Pflicht. Endung `.csv`, `.json` oder `.txt` |
| `-column` | Nur CSV: Spaltenname oder 0-basierter Index. Ohne Angabe muss genau eine numerische Spalte vorhanden sein |
| `-decimal` | `.` (Standard) oder `,` für Dezimalkommas |
| `-all` | Alle Kandidaten vergleichen, inklusive fehlgeschlagener Fits |
| `-format` | `text` (Standard) oder `json`; `json` erfordert `-all` |

Unterstützte Eingabeformate sind `.txt` (ein Wert pro Zeile), `.json` (Array von Zahlen) und `.csv` (Semikolon als Trennzeichen; die erste Zeile gilt als Header, wenn sie keine Zahlen enthält).

Verworfene Eingaben (leere Zellen, Text, `NaN`, `±Inf`, `null`) werden nicht stillschweigend übernommen: Die CLI meldet sie als Warnung auf stderr, und `Observations` bzw. `Datenpunkte` zeigt die Anzahl verworfener Werte. Im JSON-Bericht steht die Anzahl im Feld `discarded`.

Exit-Codes: `0` Erfolg, `1` Daten- oder Fitting-Fehler (inklusive mehrdeutiger CSV ohne `-column` und zu kleiner oder konstanter Stichproben), `2` Aufruffehler (ungültige Flags).

### Mindeststichprobe (Fast-Fail-Policy)

Die CLI prüft die Stichprobengröße `n` vor dem Fit:

| Bedingung | Verhalten |
|-----------|-----------|
| `n < 5` | Abbruch mit Fehlermeldung, Exit-Code `1` |
| konstante Daten (alle Werte identisch) | Abbruch mit Fehlermeldung, Exit-Code `1` |
| `5 ≤ n ≤ 20` | Fit läuft; Warnung auf stderr wegen hoher Parameterunsicherheit |
| `n > 20` | Fit läuft ohne Warnung |

Die Grenzen sind in `internal/cli/validate.go` als `MinSampleSize` (5) und `HighUncertaintySampleSize` (20) festgelegt. Die Schwellen sind eine vorläufige Entscheidung; die Warnung soll die Belastbarkeit der Parameter und des AIC-Rankings kennzeichnen.


```bash
./distfit -file=measurements.txt -all
./distfit -file=measurements.txt -all -format=json

# Spalte nach Name auswählen
./distfit -file=winequality-red.csv -column="alcohol" -all

# Spalte nach Index auswählen (0-basiert)
./distfit -file=winequality-red.csv -column=10 -all

# Dezimalkomma
./distfit -file=messwerte.csv -column=2 -decimal=, -all -format=json

# Ohne -column schlägt winequality-red.csv fehl (mehrere numerische Spalten)
./distfit -file=winequality-red.csv
```

## Vergleich mit Scipy

Zur Evaluation der Implementierungen wurde die Python-Bibliothek Scipy als Referenz hinzugezogen.
`evaluation/compare.py` vergleicht alle neun Verteilungen mit SciPy. Das Skript muss aus dem Repository-Wurzelverzeichnis gestartet werden, da es `go run ./cmd/distfit` aufruft:

```bash
evaluation/.venv/bin/python evaluation/compare.py --path=testdata/winequality-red.csv --column=density
```

Pfad und Spalte lassen sich anpassen. Im Virtualenv werden `scipy` und `pandas` benötigt.

Das Flag `-format=json` wird nur zusammen mit `-all` ausgewertet (siehe [Bekannte Probleme](#bekannte-probleme-stand-aktueller-prototyp)).

## Bekannte Probleme (Stand: aktueller Prototyp)

- **Nullwerte im Modellvergleich:** `CleanDataWithZero` entfernt nur `NaN` und `±Inf`; `0.0` und negative Werte bleiben erhalten. Die Fast-Fail-Policy (siehe [Nullwerte](#nullwerte-fast-fail-policy)) verhindert, dass Modelle ihren Träger verschieben.
- **Beispiel `winequality-red.csv`, Spalte `citric acid`:** 132 von 1599 Zeilen sind `0`. In der aktuellen Ausgabe scheitern Weibull, LogNormal, Gamma und Rayleigh. Beta gewinnt mit AIC −1130.61 vor Exponential (−975.71).
- **Vergleichbarkeit der Modelle:** Alle Kandidaten werten dieselbe Stichprobe aus. Bei negativen Werten scheiden die Positivträger-Modelle aus (`ErrNegativeValues`), sodass der Vergleich dann nur Normal, Gumbel, Laplace und Beta umfasst. Der Bericht zeigt die Stichprobengröße pro Modell nicht an.
- **Beta-Fit:** `alpha` und `beta` werden per MLE (L-BFGS) bei festem Träger geschätzt. `min` und `max` sind keine geschätzten Parameter: Sind beide `0`, übernimmt der Fitter die Datenspanne mit 1 % Rand. Für AIC/BIC gilt daher `k = 2`. Bleibt der Optimierer stehen (`ErrNoProgress` / `ErrLinesearcherFailure`), wird der beste Punkt nur übernommen, wenn der Gradient pro Beobachtung unter `1e-6` liegt.

Die endgültige CLI-Oberfläche soll mit Unterbefehlen leichter auffindbar und erweiterbar werden. Die folgenden Befehle sind das Zielinterface; sie sind nicht zwingend bereits in jedem Build verfügbar.

### Geplantes Zielinterface

**Automatische Modellwahl**

```bash
godist fit measurements.csv --column wind_speed_10m
```

Der Befehl soll:

1. die gewählte Spalte einlesen und validieren,
2. die verfügbaren Kandidaten fitten,
3. fehlgeschlagene Fits gesondert ausweisen,
4. erfolgreiche Modelle nach AIC sortieren,
5. Gewinner, Parameter und Vergleichsmetriken ausgeben.

**Verfügbare Verteilungen auflisten**

```bash
godist list distributions
```

Die Liste soll neben dem Verteilungsnamen auch Parameter, Support (zulässiger Wertebereich), eine kurze Beschreibung und gegebenenfalls besondere Fit-Anforderungen nennen.

**Eine bestimmte Verteilung erzwingen**

```bash
godist fit measurements.csv --column wind_speed_10m --distribution weibull
```

Wenn eine Verteilung explizit angegeben wird, soll der Befehl genau dieses Modell fitten und dessen Kennzahlen ausgeben, statt es gegen alle anderen Kandidaten antreten zu lassen.

### Gewünschte Textausgabe

```text
GODIST — Distribution Analysis

Input:        measurements.csv
Column:       wind_speed_10m
Observations: 1,248
Candidates:   7
Criterion:    AIC (lower is better)

Best fit
--------
Distribution: Weibull
Parameters:   shape = 2.14
              scale = 7.83

Model comparison
Rank  Distribution  Parameters       AIC   ΔAIC
1     Weibull       shape=2.14, ...  821.4   0.0
2     Gamma         shape=4.20, ...  824.1   2.7
3     Lognormal     mu=1.71, ...     829.8   8.4

Analysis completed.
```

Die Zahlen dienen nur der Illustration. Die konkrete Tabelle soll auch bei längeren Verteilungsnamen und Parametern lesbar bleiben; bei Bedarf sollen Spaltenbreiten dynamisch angepasst oder eine kompaktere Tabellenansicht verwendet werden.

### JSON-Ausgabe

JSON wird im aktuellen Prototyp nur mit `-all -format=json` ausgegeben. Der nächste Schritt ist, das JSON-Format als stabile Schnittstelle zu definieren und zu testen. Es soll mindestens Eingabemetadaten, Stichprobengröße, Auswahlkriterium, erfolgreiche Modelle, benannte Parameter, Metriken und fehlgeschlagene Fits enthalten.

Beispiel für die geplante Verwendung:

```bash
godist fit measurements.csv --column wind_speed_10m --format json
```

Bei JSON-Ausgabe gehört ausschließlich das JSON-Dokument auf `stdout`; Statusmeldungen und Fehler gehören auf `stderr`. So lässt sich die Ausgabe zuverlässig an andere Werkzeuge weiterleiten.

## Auswahlkriterium und Gütemaße

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

## Nächste Entwicklungsschritte

Die Arbeit sollte zunächst die Zuverlässigkeit der vorhandenen Verteilungen absichern und anschließend die CLI und den Verteilungskatalog ausbauen.

### 1. Statistischen Kern validieren — höchste Priorität

- Für jede Verteilung PDF, CDF und LogPDF prüfen: Wertebereiche, endliche Ergebnisse und konsistentes Verhalten an den Support-Grenzen.
- Synthetische Datensätze mit bekannten Parametern erzeugen und die Schätzungen gegen erwartete Werte oder eine unabhängige Referenzimplementierung prüfen.
- Log-Likelihood und AIC/BIC anhand unabhängiger Berechnungen validieren.
- Sicherstellen, dass `k` alle tatsächlich frei geschätzten Parameter zählt.
- Konvergenzstatus und Optimierungsfehler explizit erfassen.
- Verhindern, dass ein Fallback auf Momentenschätzer fälschlich als erfolgreicher MLE ausgegeben wird.
- Startwerte, Parametergrenzen und Support-Bedingungen für jeden Fitter dokumentieren.

### 2. Randfälle und Datenvalidierung

Mindestens folgende Fälle automatisiert testen:

- leere Daten und zu wenige Beobachtungen,
- konstante Daten beziehungsweise Varianz null,
- `NaN`, positive und negative Unendlichkeit,
- viele exakte `0.0`-Werte, zum Beispiel `citric acid` in `winequality-red.csv` (132 von 1599 Zeilen), inklusive ihrer Auswirkung auf Support-Grenzen, Log-Likelihood und die Vergleichbarkeit der AIC-Werte,
- Werte außerhalb des Supports einer Verteilung,
- sehr kleine und sehr große Werte beziehungsweise Skalen,
- Parameter nahe erlaubten Grenzen,
- numerischer Überlauf oder Unterlauf bei Dichteberechnungen,
- nicht konvergierende Fits und fehlgeschlagene einzelne Kandidaten.

Ungültige Werte sollen nicht stillschweigend verschwinden. Die CLI soll anzeigen, wie viele Beobachtungen gelesen, verwendet, verworfen oder wegen ungültiger Daten abgelehnt wurden.

### 3. Automatisierte Tests ausbauen

- Unit-Tests für alle neun Verteilungen und Fitter sind in `pkg/distribution` vorhanden; ergänzt werden Grenzfälle gemäß Punkt 2.
- Eigenschaften testen: CDF ist monoton und liegt in `[0, 1]`; PDF ist auf dem Support nicht negativ; LogPDF stimmt numerisch mit `log(PDF)` überein, soweit die Dichte nicht unterläuft.
- Geeignete numerische Integrations- und Grenzwerttests für die Dichte implementieren.
- Regressionstests mit festen Beispieldatensätzen und erwarteten Ergebnissen ergänzen.
- Tests für Modellranking, AIC-Parameterzählung und fehlgeschlagene Fits hinzufügen.
- CLI-Tests für Textausgabe, JSON-Schema, Exit-Codes, CSV-Spaltenauswahl und ungültige Eingaben ergänzen.
- Testläufe mit `go test ./...` in die Entwicklungsroutine integrieren.

### 4. CLI und Einleselogik vervollständigen

- Unterbefehle `fit` und `list distributions` implementieren.
- `--column` nach Spaltenname und – wenn sinnvoll – nach Index unterstützen.
- CSV-Dateien mit mehreren numerischen Spalten niemals ungefragt zu einem einzigen Datensatz vermischen.
- Für mehrdeutige Eingaben eine klare Fehlermeldung mit verfügbaren Spalten anzeigen.
- Unterstützung für Standard-Input (`-`) und klare Regeln für leere Zeilen, Header, Trennzeichen und Dezimalformate ergänzen.
- `--distribution` zum Fit einer einzelnen Verteilung anbieten.
- Fehler und Warnungen verständlich ausgeben und geeignete Exit-Codes definieren.
- Ausgabeparameter mit stabilen, verteilungsspezifischen Namen bereitstellen.
- JSON-Schema dokumentieren und mit Tests gegen unbeabsichtigte Änderungen absichern.

### 5. Neue Verteilungen priorisiert hinzufügen

Mögliche Erweiterungen:

| Priorität | Verteilungen | Hinweise |
|---|---|---|
| Früh | Logistic, Cauchy | Ergänzen symmetrische Modelle mit unterschiedlichen Tail-Eigenschaften; Cauchy benötigt besondere Behandlung, da gewöhnliche Momente nicht existieren. |
| Früh bis mittel | Log-logistisch, Pareto | Nützlich für positive Daten und schwere rechte Tails. |
| Mittel | Student-t | Symmetrisch mit einstellbarer Tail-Schwere; Freiheitsgrade müssen robust geschätzt werden. |
| Mittel | Chi-Quadrat | Spezialfälle beziehungsweise eng mit vorhandenen Familien verwandt; als eigener Verteilungstyp dennoch praktisch. |
| Später | Generalized Extreme Value (GEV) | Wertvoll für Extremwertanalysen, aber mit parameterabhängigem Support und anspruchsvoller numerischer Optimierung. |

Jede neue Verteilung benötigt neben PDF/CDF/LogPDF auch einen validierten Fitter, dokumentierte Parameter und Support-Bedingungen sowie eigene Tests. Die Implementierung über `ContinuousDistribution` allein reicht nicht aus, um die Qualität der Parameterschätzung sicherzustellen.

### 6. Performance und GC-Allokationen messen

Performanceoptimierungen sollen auf Benchmarks beruhen und erst nach der fachlichen Validierung erfolgen.

- Benchmarks für jeden Fitter und den vollständigen Modellvergleich erstellen.
- Datensätze unterschiedlicher Größen verwenden, zum Beispiel 100, 10.000 und 1.000.000 Beobachtungen, sofern für den jeweiligen Benchmark sinnvoll.
- Laufzeit und Allokationen mit `go test -bench=. -benchmem ./...` messen.
- Hotspots bei wiederholten Schleifen, temporären Slices, Interface-Aufrufen und wiederholter Berechnung gemeinsamer Ausdrücke profilieren.
- Ergebnisse vor und nach Optimierungen vergleichen und Korrektheitstests erneut ausführen.
- Keine schwerer lesbare oder fehleranfälligere Implementierung allein wegen vermuteter GC-Einsparungen einführen.

### 7. Dokumentation und Release-Vorbereitung

- README-Beispiele mit tatsächlich ausführbaren Befehlen synchron halten.
- Parameterkonventionen und Support jeder Verteilung dokumentieren.
- Grenzen von AIC, BIC, KS und RMSE erklären.
- Versionierung und kompatible Änderungen der Bibliotheks-API festlegen.
- CI für Tests, Formatierung und gegebenenfalls statische Analyse ergänzen.
- Erst nach bestandenen Validierungs- und Regressionstests einen produktiven Einsatz empfehlen.

## Entwicklungsbefehle

```bash
# Tests
go test ./...

# Benchmarks inklusive Allokationszahlen (derzeit sind noch keine Benchmark-Funktionen vorhanden)
go test -bench=. -benchmem ./...

# CLI bauen
go build -o distfit ./cmd/distfit
```

Benchmarks können je nach Maschine, Go-Version und Datensatz unterschiedlich ausfallen. Ergebnisse sollten deshalb auf derselben Umgebung und mit vergleichbaren Eingaben gegenübergestellt werden.

## Grundsatz

`godist` soll nicht einfach die größtmögliche Zahl von Verteilungen anbieten, sondern für jeden Kandidaten nachvollziehbare und numerisch belastbare Fits liefern. **Korrektheit, Diagnose und Reproduzierbarkeit haben Vorrang vor der Zahl der Verteilungen und vor Mikrooptimierungen.**
