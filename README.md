# godist — Statistische Verteilungen und numerische Parameterschätzung

`godist` ist eine in Go entwickelte Bibliothek und ein Kommandozeilenwerkzeug (CLI) zur Analyse stetiger Wahrscheinlichkeitsverteilungen. Es passt mehrere Verteilungsmodelle an einen gegebenen Datensatz an, vergleicht deren Güte und wählt standardmäßig das Modell mit dem niedrigsten Akaike Information Criterion (AIC).

> **Projektstatus: noch nicht produktiv verwenden.**
> Vor einem produktiven Einsatz müssen insbesondere Parameterschätzung, numerische Stabilität, Gütemaße, Eingabevalidierung und Testabdeckung systematisch überprüft werden.
>
> **Aktueller Stand:** 20 Verteilungen sind implementiert. Die Tests in `pkg/distribution`, `internal/cli` und `cmd/distfit` laufen grün (`go test ./...`). Der CLI-Prototyp verarbeitet reale Datensätze (z. B. `winequality-red.csv`). Nullwerte werden strikt behandelt (siehe [Methodik])


## Unterstützte Verteilungen

| Verteilung | Parameter | Träger |
|---|---|---|
| Beta | `alpha`, `beta`, `min`, `max` | [min, max] |
| Exponential | `rate` | [0, ∞) |
| Gauß / Normal | `mu`, `sigma` | ℝ |
| Gumbel | `mu`, `beta` | ℝ |
| GEV | `mu`, `sigma`, `xi` | ξ-abhängig |
| Gamma | `alpha`, `scale` | [0, ∞) |
| Lognormal | `mu`, `sigma` | (0, ∞) |
| Weibull | `shape`, `scale` | [0, ∞) |
| Laplace | `mu`, `scale` | ℝ |
| Logistic | `mu`, `scale` | ℝ |
| LogLogistic | `scale`, `shape` | (0, ∞) |
| BurrXII | `scale`, `shape_c`, `shape_k` | (0, ∞) |
| Rayleigh | `scale` | [0, ∞) |
| Pareto | `shape`, `scale` | [scale, ∞) |
| ChiSquare | `df` | [0, ∞) |
| Cauchy | `mu`, `scale` | ℝ |
| StudentT | `mu`, `scale`, `nu` | ℝ |
| SkewNormal | `xi`, `omega`, `alpha` | ℝ |
| InverseGaussian | `mu`, `lambda` | (0, ∞) |
| GeneralizedGamma | `shape`, `scale`, `power` | (0, ∞) |

GeneralizedGamma (verallgemeinerte Gamma-Verteilung nach Stacy): `power` ist der Potenzparameter; `power = 1` ergibt die Gamma-Verteilung, `shape = 1` die Weibull-Verteilung. `power < 0` ist zulässig. Die drei Parameter werden per L-BFGS auf der in der Skala profilierten Likelihood geschätzt; beide Vorzeichen von `power` werden optimiert.

Student-t: ν wird per Profil-Likelihood auf [0.1, 10⁹] begrenzt. Nahezu normalverteilte Daten erhalten daher sehr große ν-Werte, die an SciPy angenähert sind (White-Wine: ν ≈ 3.5·10⁷ statt 7.7·10⁸, AIC-Abweichung 0.00).

SkewNormal (schiefe Normalverteilung nach Azzalini): `alpha` steuert die Schiefe; `alpha = 0` ergibt die Normalverteilung, `|alpha| → ∞` eine halbseitige Normalverteilung. Die Parameter werden per L-BFGS-MLE geschätzt.

GEV (verallgemeinerte Extremwertverteilung): `xi` ist der Formparameter; `xi = 0` ergibt die Gumbel-Verteilung. Für `xi > 0` (Fréchet-Typ) ist der Träger nach unten, für `xi < 0` (Weibull-Typ) nach oben beschränkt. Die drei Parameter werden per L-BFGS-MLE geschätzt; `xi` wird dabei auf [−0.99, 5] begrenzt. Die Verteilung ist für Extremwertdaten (z. B. Blockmaxima) gedacht — ein gutes AIC-Ranking ersetzt keine fachliche Prüfung der Datenherkunft und Modellannahmen.


## Methodik

### Nullwerte (Fast-Fail-Policy)

Die Verteilungen mit positivem Träger behandeln Nullwerte derzeit **streng**: Ein Fit scheitert, statt Nullen stillschweigend zu verwerfen oder den Träger zu verschieben.

- **LogNormal:** scheitert bei jedem Nullwert, da `log(0)` nicht definiert ist.
- **LogLogistic:** scheitert bei jedem Nullwert, da `log(0)` nicht definiert ist (wie LogNormal).
- **BurrXII:** scheitert bei jedem Nullwert, da `log(0)` nicht definiert ist. Fits, bei denen c oder k über 10⁶ laufen (Grenzfall, z. B. exponentialähnliche Daten), werden ebenfalls abgelehnt, da dann kein inneres Maximum der Likelihood existiert.
- **GeneralizedGamma:** scheitert bei jedem Nullwert, da `log(0)` nicht definiert ist.
- **Gamma und Weibull:** scheitern bei jedem Nullwert. Die Dichte an der Stelle 0 ist für Formparameter ungleich 1 entweder 0 oder unendlich. Für Formparameter genau 1 entspricht die Verteilung der Exponentialverteilung, die Nullwerte behält.
- **Rayleigh:** Nullen werden bei der Schätzung ignoriert, die Dichte an der Stelle 0 ist aber 0. Der Kandidat scheitert daher bei jedem Nullwert.
- **ChiSquare:** Der Träger ist [0, ∞). Nullwerte liegen für k > 0 am Rand des Trägers und werden wie bei Gamma abgelehnt (Fast-Fail).
- **Exponential:** behält Nullwerte; die Dichte an der Stelle 0 ist endlich, der Kandidat ist also nicht betroffen.
- **Normal, SkewNormal, Gumbel, GEV, Laplace, Logistic:** behalten Nullwerte (`CleanDataWithZero`) und sind von dieser Regel nicht betroffen.
- **Beta:** Der Träger wird aus den Daten abgeleitet ( Ob Nullwerte scheitern, entscheidet die Dichteprüfung. `min`/`max` werden per ML geschätzt und zählen als freie Parameter (α, β ≥ 1); bei entarteten Grenzfällen (β, Grenzen → ∞, entsprechend Gamma-Grenzverteilungen) scheitert der Kandidat.

**Negative Werte** werden ebenfalls über die Trägerprüfung behandelt: Exponential, Gamma, Weibull, LogNormal, LogLogistic, BurrXII, Rayleigh, Pareto, ChiSquare und GeneralizedGamma lehnen jeden negativen Wert mit `ErrNegativeValues` ab. Normal, SkewNormal, Gumbel, GEV, Laplace, Cauchy und StudentT sowie Beta (Träger aus den Daten) verarbeiten negative Werte. Alle Kandidaten werten so dieselbe Stichprobe aus.

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

### Bootstrap-Konfidenzintervalle

Optional (`-bootstrap=B`) werden pro Modell `B` Bootstrap-Wiederholungen ausgeführt und Perzentil-Konfidenzintervalle für die Parameter und die Gütemetriken (AIC, BIC, KS, RMSE) berechnet. Zwei Stichproben-Methoden stehen zur Wahl: `nonparametric` (Resampling der Daten mit Zurücklegen) und `parametric` (Sampling aus der gefitteten Verteilung). Anzahl, Niveau (`-bootstrap-ci`), Methode, Seed (`-bootstrap-seed`) und Modellumfang (`-bootstrap-model=best|all`) sind konfigurierbar; Details, Aussagekraft und Performance stehen in [docs/CLI.md](docs/CLI.md#bootstrap-konfidenzintervalle).

Optional (`-dip`) wird zusätzlich Hartigans Dip-Test auf die Eingabestichprobe ausgeführt: Er prüft die Nullhypothese Unimodalität der Daten (nicht des Modells) und liefert Dip-Statistik und Monte-Carlo-p-Wert. Simulationszahl und Seed sind über `-dip-simulations` und `-dip-seed` konfigurierbar; in der JSON-Ausgabe erscheint das optionale Feld `dip_test`. Details stehen in [docs/CLI.md](docs/CLI.md#dip-test-unimodalität).

## Tests

Die Testsuite besteht derzeit aus ca. 25 Testdateien und mehr als 100 Einzeltests.
Es folgt ein kurzer Überblick über den Testumfang:

### Datenvalidierung

- Leere Daten
- Konstante Daten
- NaN, Inf und 0.0
- Werte außerhalb des Supports
- Sehr kleine und große Werte
- Numerischer Über- und Unterlauf

### Fitter 

- Unit-Tests für alle unterstützten Verteilungen 
- Eigenschaftstests (CDF monoton, PDF ≥ 0, LogPDF = log(PDF))
- Numerische Integrationstests
- Regressionstests
- Modellranking-Tests

### CLI-Tests

- Verhalten im Fehlerfall (Exit-Codes)
- Umgang mit fehlgeschlagenen Fits (Discarded im JSON-Format) 
- Wenige oder identische Daten
- Flagtests
- Dashboard-Tests
- API-Tests

### Performancetests

- Benchmarks für alle Verteilungen für 100, 10000 und 1000000 synthetische Daten
- Benchmarks für den Gesamtlauf (skaliert ungefähr linear ca. 25ms für n=10000 und 2-3 s für n=1000000)
- Benchmarks für parallelen und sequenziellen Lauf (Parallelität reduziert die Laufzeit um Faktor 4 auf Apple M3, 8 CPU-Kerne)

## Weitere Schritte

Der numerische Kern ist weitgehend implementiert und getestet. 
Die zukünftige Entwicklung wird sich daher auf Modellvalidierung und Unsicherheitsdiagostik konzentrieren.
Der detaillierte Plan ist dem Dokument docs/roadmap_new.md zu entnehmen.

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
| CLI: Flags, Eingabeformate, Mindeststichprobe, Zielinterface | [docs/cli.md](docs/cli.md) |
| JSON-Ausgabe und Einbindung in Go, Python, TypeScript | [docs/json.md](docs/json.md) |
| Evaluation: Vergleich mit SciPy, Benchmarks | [docs/evaluation.md](docs/evaluation.md) |
| Roadmap und nächste Schritte | [docs/roadmap_new.md](docs/roadmap_new.md) |
| Alte Roadmap und Projektanfänge | [docs/roadmap_old.md](docs/roadmap_old.md) |

## Grundsatz

`godist` soll nicht einfach die größtmögliche Zahl von Verteilungen anbieten, sondern für jeden Kandidaten nachvollziehbare und numerisch belastbare Fits liefern. **Korrektheit, Diagnose und Reproduzierbarkeit haben Vorrang vor der Zahl der Verteilungen und vor Mikrooptimierungen.**
