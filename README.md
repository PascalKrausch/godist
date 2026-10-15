# godist — Statistische Verteilungen und numerische Parameterschätzung

`godist` ist eine in Go entwickelte Bibliothek und ein Kommandozeilenwerkzeug (CLI) zur Analyse stetiger Wahrscheinlichkeitsverteilungen. Es passt mehrere Verteilungsmodelle an einen gegebenen Datensatz an, vergleicht deren Güte und wählt standardmäßig das Modell mit dem niedrigsten Akaike Information Criterion (AIC).

> **Projektstatus: noch nicht produktiv verwenden.**  
> Vor einem produktiven Einsatz müssen insbesondere Parameterschätzung, numerische Stabilität, Gütemaße, Eingabevalidierung und Testabdeckung systematisch überprüft werden.

## Ziele

- Stetige Verteilungen über eine gemeinsame Go-Schnittstelle anbieten.
- Verteilungsparameter aus Messdaten schätzen.
- Mehrere Modelle vergleichbar fitten und nach AIC sortieren.
- Zusätzliche Kennzahlen wie BIC, KS-Statistik und RMSE bereitstellen.
- Das gleiche Analyseergebnis als menschenlesbaren Terminalbericht und maschinenlesbare JSON-Ausgabe bereitstellen.
- Die mathematische Bibliothek unabhängig von CLI, Dateiformaten und Darstellung halten.
- Korrektheit und Robustheit zuerst absichern; anschließend Laufzeit und GC-Allokationen messen und gezielt optimieren.

## Aktuell unterstützte Verteilungen

- Beta
- Exponential
- Gauß / Normal
- Gumbel
- Gamma
- Lognormal
- Weibull

Die konkrete Unterstützung hängt davon ab, welche Fitter in der aktuellen Version registriert sind. Eine Verteilung sollte erst als unterstützt gelten, wenn nicht nur ihre Dichte, sondern auch Parameterschätzung, Parametergrenzen und Tests implementiert sind.

## Architektur

```text
.
├── cmd/
│   └── distfit/
│       └── main.go           # CLI-Einstiegspunkt
├── internal/
│   └── cli/
│       └── report/           # Terminalbericht und Ausgabeformatierung
├── pkg/
│   └── distribution/
│       ├── beta.go
│       ├── data_cleaning.go
│       ├── exponential.go
│       ├── fitter.go
│       ├── gamma.go
│       ├── gaussian.go
│       ├── gumbel.go
│       ├── log_normal.go
│       ├── types.go
│       ├── weibull.go
│       └── fitter_test/      # Tests für den Fitter
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
}

// Fitter schätzt die Parameter einer Verteilung aus Daten.
type Fitter interface {
    Fit(data []float64) (ContinuousDistribution, error)
    Name() string
}
```

`Params() []float64` ist als generische Schnittstelle praktisch, für nutzerfreundliche Berichte sind jedoch benannte Parameter nötig. Geplant ist daher, Parameter wie `shape`, `scale`, `location`, `rate`, `alpha` oder `beta` an der Verteilung beziehungsweise über Metadaten zugänglich zu machen, statt sie in CLI-Ausgaben nur als `p0`, `p1` usw. darzustellen.

## Aktueller CLI-Prototyp

Der bestehende Prototyp unterstützt bereits Dateieingabe, Modellvergleich und Textausgabe. In der getesteten Version sind außerdem die Auswahl aller Kandidaten über `-all` und die JSON-Ausgabe über `-format=json` vorhanden.

Beispiele für die aktuelle flag-basierte Oberfläche:

```bash
./distfit -file=measurements.txt -all
./distfit -file=measurements.txt -all -format=json
```

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

JSON wird im aktuellen Prototyp bereits ausgegeben. Der nächste Schritt ist, das JSON-Format als stabile Schnittstelle zu definieren und zu testen. Es soll mindestens Eingabemetadaten, Stichprobengröße, Auswahlkriterium, erfolgreiche Modelle, benannte Parameter, Metriken und fehlgeschlagene Fits enthalten.

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
- Werte außerhalb des Supports einer Verteilung,
- sehr kleine und sehr große Werte beziehungsweise Skalen,
- Parameter nahe erlaubten Grenzen,
- numerischer Überlauf oder Unterlauf bei Dichteberechnungen,
- nicht konvergierende Fits und fehlgeschlagene einzelne Kandidaten.

Ungültige Werte sollen nicht stillschweigend verschwinden. Die CLI soll anzeigen, wie viele Beobachtungen gelesen, verwendet, verworfen oder wegen ungültiger Daten abgelehnt wurden.

### 3. Automatisierte Tests ausbauen

- Unit-Tests für jede Verteilung und jeden Fitter ergänzen.
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
| Früh | Laplace, Logistic, Cauchy | Ergänzen symmetrische Modelle mit unterschiedlichen Tail-Eigenschaften; Cauchy benötigt besondere Behandlung, da gewöhnliche Momente nicht existieren. |
| Früh bis mittel | Log-logistisch, Pareto | Nützlich für positive Daten und schwere rechte Tails. |
| Mittel | Student-t | Symmetrisch mit einstellbarer Tail-Schwere; Freiheitsgrade müssen robust geschätzt werden. |
| Mittel | Rayleigh, Chi-Quadrat | Spezialfälle beziehungsweise eng mit vorhandenen Familien verwandt; als eigener Verteilungstyp dennoch praktisch. |
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

# Benchmarks inklusive Allokationszahlen
 go test -bench=. -benchmem ./...

# CLI bauen
 go build -o distfit ./cmd/distfit
```

Benchmarks können je nach Maschine, Go-Version und Datensatz unterschiedlich ausfallen. Ergebnisse sollten deshalb auf derselben Umgebung und mit vergleichbaren Eingaben gegenübergestellt werden.

## Grundsatz

`godist` soll nicht einfach die größtmögliche Zahl von Verteilungen anbieten, sondern für jeden Kandidaten nachvollziehbare und numerisch belastbare Fits liefern. **Korrektheit, Diagnose und Reproduzierbarkeit haben Vorrang vor der Zahl der Verteilungen und vor Mikrooptimierungen.**
