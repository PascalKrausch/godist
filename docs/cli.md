# CLI

`distfit` liest eine Datei mit Messwerten, passt Verteilungen an die Daten an und wählt das beste Modell nach AIC (Akaike-Informationskriterium, kleiner ist besser).
## Aufruf

Es gibt drei Formen:

```text
distfit fit <datei> [optionen]          Unterbefehl "fit" (Datei als Argument)
distfit -file=<datei> [optionen]        Flag-Form (Datei per -file)
distfit list distributions [-format=text|json]
```

`distfit fit datei.txt` und `distfit -file=datei.txt` sind gleichwertig. Die Flag-Form bleibt vollständig gültig.

### Regeln für Flags

- Werte werden mit `=` angegeben: `-distribution=gamma`. Die Schreibweise `-distribution gamma` ist **nicht** erlaubt und führt zu Exit-Code `2`, weil `gamma` dann als Positionsargument gelesen wird.
- Flags dürfen vor oder nach der Datei stehen: `distfit fit werte.txt -all` und `distfit fit -all werte.txt` funktionieren gleich.
- `--` beendet die Flags. Alles danach gilt als Positionsargument: `distfit fit -all -- werte.txt`. Flags nach `--` werden nicht ausgewertet.
- `--flag` ist gleichwertig zu `-flag`: `--distribution=gamma`.
- Die Datei nicht doppelt angeben: `distfit fit werte.txt -file=werte.txt` ist ein Fehler (Exit `2`).

## Eingabeformate

| Endung | Format |
|--------|--------|
| `.txt` | Ein Zahlenwert pro Zeile |
| `.json` | Array von Zahlen. Ein JSON-Objekt wird akzeptiert, wenn es ein numerisches Array auf oberster Ebene enthält; gibt es mehrere, wird das Array mit dem alphabetisch ersten Schlüssel verwendet. |
| `.csv` | Trennzeichen Semikolon. Die erste Zeile gilt als Header, wenn sie keine Zahlen enthält. |

Bei CSV-Dateien mit mehreren numerischen Spalten muss mit `-column` eine Spalte gewählt werden, sonst bricht die CLI mit Exit `1` ab:

```text
Fehler beim Einlesen von mehrdeutig.csv: mehrere numerische Spalten gefunden (wind, density, citric); bitte mit -column eine Spalte wählen
```

Verworfene Eingaben (leere Zellen, Text, `NaN`, `±Inf`, `null`) werden nicht stillschweigend übernommen. Die CLI zeigt sie als Warnung auf stderr mit Zeilennummer, die Zusammenfassung nennt die Anzahl (`Datenpunkte` bzw. `Observations`, im JSON das Feld `discarded`).

## Flags

| Flag | Wert | Bedeutung |
|------|------|-----------|
| `-file` | Pfad | Eingabedatei (`.txt`, `.json`, `.csv`). Bei `fit` alternativ als Positionsargument. |
| `-column` | Name oder 0-basierter Index | Nur CSV. Ohne Angabe muss genau eine numerische Spalte vorhanden sein. Bei anderen Formaten Exit `2`. |
| `-decimal` | `.` oder `,` | Dezimaltrennzeichen in CSV-Dateien. Standard: `.` |
| `-all` | – | Alle ausgewählten Kandidaten vergleichen und als Tabelle ausgeben, inklusive fehlgeschlagener Fits. |
| `-format` | `text`, `json`, `plot` | Ausgabeformat. `json` und `plot` erfordern `-all`. Standard: `text` |
| `-distribution` | Verteilungsname | Genau eine Verteilung fitten. Groß-/Kleinschreibung egal. Schließt `-include` und `-exclude` aus. |
| `-include` | Liste, kommasepariert | Nur diese Verteilungen verwenden. Schließt `-exclude` aus. |
| `-exclude` | Liste, kommasepariert | Diese Verteilungen aus dem Katalog auslassen. |
| `-zero-inflated` | – | Nullwerte als Zero-Inflated-Modell (Punktmasse bei 0) modellieren. Siehe unten. |
| `-bootstrap` | Ganzzahl ≥ 0 | Anzahl Bootstrap-Wiederholungen für Konfidenzintervalle. `0` = aus (Standard). |
| `-bootstrap-method` | `nonparametric` oder `parametric` | Stichproben-Erzeugung. Standard: `nonparametric`. Erfordert `-bootstrap`. |
| `-bootstrap-ci` | Zahl in (0, 1) | Konfidenzniveau der Intervalle. Standard: `0.95`. Erfordert `-bootstrap`. |
| `-bootstrap-seed` | Ganzzahl ≥ 0 | Fester Seed für reproduzierbare Läufe. `0` = zufällig; der wirksame Seed steht dann auf stderr. Erfordert `-bootstrap`. |
| `-bootstrap-model` | `best` oder `all` | Für welche Modelle Bootstrap läuft: nur das AIC-beste (`best`, Standard) oder alle erfolgreichen Fits (`all`). Erfordert `-bootstrap`. |
| `-dip` | – | Hartigan-Dip-Test auf die bereinigte Stichprobe (Unimodalität). Siehe unten. |
| `-dip-simulations` | Ganzzahl ≥ 1 | Monte-Carlo-Simulationen für den p-Wert. Standard: `2000` (2000 bis 10000 empfohlen). Erfordert `-dip`. |
| `-dip-seed` | Ganzzahl ≥ 0 | Fester Seed für reproduzierbare Läufe. Standard: `1`. Erfordert `-dip`. |

Im Hilfetext (`distfit -h`) stehen dieselben Flags mit Kurzbeschreibung.

## Verteilungen

`distfit list distributions` zeigt alle 20 Kandidaten mit Parametern, Träger und Hinweisen:

```text
$ distfit list distributions
Name         Parameter              Träger                      Beschreibung
Weibull      shape, scale           [0, ∞)                      Zwei-Parameter-Weibull, häufig für Lebensdauern und Windgeschwindigkeiten
LogNormal    mu, sigma              (0, ∞)                      Logarithmische Normalverteilung für positive, rechtsschiefe Daten
Gamma        alpha, scale           [0, ∞)                      Gammaverteilung für positive, rechtsschiefe Daten
LogLogistic  scale, shape           (0, ∞)                      Log-Logistische Verteilung für positive Daten mit schwerem rechten Rand
Normal       mu, sigma              ℝ                           Normalverteilung, symmetrisch um mu
SkewNormal   xi, omega, alpha       ℝ                           Schiefe Normalverteilung (Azzalini); alpha = 0 ergibt die Normalverteilung
Beta         alpha, beta, min, max  [min, max] (aus den Daten)  Beta-Verteilung auf einem endlichen Intervall
Exponential  rate                   [0, ∞)                      Exponentialverteilung, gedächtnislos, ein Parameter
Gumbel       mu, beta               ℝ                           Gumbel-Verteilung für Maxima, rechtsschief
GEV          mu, sigma, xi          ξ-abhängig                   Verallgemeinerte Extremwertverteilung (GEV); xi = 0 ergibt die Gumbel-Verteilung
...
```

Die vollständige Liste der Namen (Standardreihenfolge):
`Weibull`, `LogNormal`, `Gamma`, `Normal`, `SkewNormal`, `Beta`, `Exponential`, `Gumbel`, `GEV`, `Rayleigh`, `Laplace`, `Logistic`, `LogLogistic`, `BurrXII`, `Pareto`, `ChiSquare`, `Cauchy`, `StudentT`, `InverseGaussian`, `GeneralizedGamma`.

Mit `-format=json` liefert der Befehl ein Array; die Felder sind `name`, `description`, `params`, `support`, `fit_notes` und `zero_inflatable`:

```bash
distfit list distributions -format=json
```

```json
[
  {
    "name": "Weibull",
    "description": "Zwei-Parameter-Weibull, häufig für Lebensdauern und Windgeschwindigkeiten",
    "params": ["shape", "scale"],
    "support": "[0, ∞)",
    "fit_notes": "",
    "zero_inflatable": true
  },
  ...
]
```

Die Namen sind in allen Flags case-insensitive: `weibull`, `Weibull` und `WEIBULL` sind dasselbe.

## Modellauswahl

Ohne Auswahlflag werden alle 20 Kandidaten verwendet. Mit den folgenden Flags lässt sich die Menge einschränken:

| Kombination | Verhalten |
|-------------|-----------|
| `-distribution=weibull` | Nur Weibull wird gefittet. Ohne `-all` gibt es eine Zusammenfassung, mit `-all` eine Tabelle mit dieser einen Zeile. |
| `-include=normal,lognormal` | Nur Normal und Lognormal werden verglichen. Duplikate werden entfernt; die Reihenfolge von `-include` bleibt erhalten. |
| `-exclude=studentt,cauchy` | Alle Kandidaten außer Student t und Cauchy werden verglichen. |
| `-include` + `-exclude` | Fehler (Exit `2`): `include und exclude schließen sich aus` |
| `-distribution` + `-include`/`-exclude` | Fehler (Exit `2`): `-distribution schließt -include und -exclude aus` |
| unbekannter Name | Fehler (Exit `2`), mit der Liste der gültigen Namen |
| alle Kandidaten ausgeschlossen | Fehler (Exit `2`): `keine Verteilung übrig; include oder exclude prüfen` |

### Zero-Inflation mit `-zero-inflated`

`-zero-inflated` modelliert Nullwerte als Punktmasse bei 0 plus eine kontinuierliche Verteilung für die positiven Werte. Verglichen werden dann nur die ZI-fähigen Verteilungen, angezeigt als `ZI-<Name>`:

- ZI-fähig: Weibull, LogNormal, LogLogistic, BurrXII, Gamma, Exponential, Rayleigh, Pareto, ChiSquare
- Ohne Flag scheitern Kandidaten mit Träger (0, ∞) bei Nullwerten (Fast-Fail-Policy).
- Mit `-zero-inflated` und einer nicht ZI-fähigen Verteilung (`-distribution=normal`, `-include=weibull,normal`) gilt Exit `2`: `Normal unterstützt keine Zero-Inflation`.

Die AIC-Werte mit und ohne `-zero-inflated` sind nicht miteinander vergleichbar, weil die Modelle unterschiedlich viele Parameter und Datenpunkte verwenden.

## Bootstrap-Konfidenzintervalle

Mit `-bootstrap=B` werden pro Modell `B` Bootstrap-Wiederholungen ausgeführt: Der Fit wird auf resampelten Daten wiederholt und die Verteilung der geschätzten Parameter und Gütemetriken (AIC, BIC, KS, RMSE) wird als Perzentil-Konfidenzintervall zum Niveau `-bootstrap-ci` zusammengefasst. `B = 0` (Standard) schaltet das Feature ab; die bootstrap-Flags ohne `-bootstrap` sind ein Aufruffehler (Exit `2`).

```bash
# KI nur für das beste Modell (Standard)
distfit fit werte.txt -bootstrap=1000 -bootstrap-seed=42

# KI für alle Kandidaten, parametrisch, 90%-Niveau
distfit fit werte.txt -all -bootstrap=500 -bootstrap-model=all \
    -bootstrap-method=parametric -bootstrap-ci=0.9
```

Die Intervalle erscheinen im Textbericht als Abschnitt „Bootstrap confidence intervals“ (mit `-all`) bzw. als Block in der Einzelmodellausgabe, im JSON als optionales Feld `bootstrap` je Modell (siehe [JSON.md](JSON.md)). Auf stderr meldet die CLI die Parameter: Anzahl Wiederholungen, Methode, Niveau, Seed und Modellumfang; der Seed macht den Lauf mit `-bootstrap-seed` reproduzierbar.

### Methoden

- **`nonparametric`** (Standard): Jede Replicate zieht `n` Werte mit Zurücklegen aus den (bereinigten) Daten und fitet erneut. Keine Verteilungsannahme für die Stichprobe; robust, aber bei kleinen `n` und Schiefheit können die Intervalle asymmetrisch und breit sein.
- **`parametric`**: Jede Replicate zieht `n` Werte aus der *gefiteten* Verteilung (Inversion über `Quantile`) und fitet erneut. Engere Intervalle, aber nur sinnvoll, wenn das Modell die Daten wirklich beschreibt — die Aussagekraft hängt an der Modellgüte, nicht an den Daten.

Replicates, deren Fit oder Metrikberechnung scheitert (z. B. Zero-Inflated-Modelle ohne Nullwerte in der Resample, Grenzfälle bei Beta/Pareto), werden gezählt (`failed`) und nicht stillschweigend aus dem Ergebnis entfernt; die Kennzahlen `replicates`/`requested` zeigen die Ausfallquote. Unter 2 erfolgreichen Replicates gibt es keine Intervalle.

### Aussagekraft und Grenzen

- Die Intervalle sind **Perzentil-Intervalle** (kein Bias-Corrected-and-Accelerated): einfach und robust, aber bei stark schiefen oder grenznahen Schätzungen (z. B. GEV-Formparameter) können sie die tatsächliche Überdeckungswahrscheinlichkeit verfehlen.
- Ein Überlappen zweier Modell-Intervalle ist **kein** formeller Modellvergleich — es ist ein Hinweis auf nicht unterscheidbare Parameter, nicht auf gleiche Modellgüte. Die Modellauswahl bleibt Sache des AIC.
- Parametrisches Bootstrap setzt ein korrekt spezifiziertes Modell voraus; es beantwortet „wie streuen die Schätzer, wenn das Modell stimmt“, nicht „stimmt das Modell“.
- Für sehr kleine Stichproben (`n` unter ~20) sind die Intervalle typischerweise so breit, dass sie wenig Information tragen; die CLI warnt in diesem Fall bereits separat.
- `-format=plot` zeigt die Bootstrap-Ergebnisse nicht an; die Daten stehen nur in Text- und JSON-Ausgabe.

### Performance

Jede Replicate ist ein vollständiger Fit plus Metrikberechnung; die Kosten skalieren etwa linear mit `B` und mit der Anzahl der Modelle (`-bootstrap-model=all` multipliziert den Aufwand mit der Kandidatenzahl). Die Replicates laufen parallel in einem Worker-Pool der Größe `GOMAXPROCS`.

Gemessen auf Apple M3 (`go test -bench=Bootstrap -benchtime=1x ./pkg/distribution`), `n = 1000`, `B = 100`:

| Methode | Zeit für B=100 |
|---------|----------------|
| `nonparametric` | ca. 5 ms |
| `parametric` | ca. 6 ms |

Das sind ~50 µs pro Replicate; `B = 1000` kostet also etwa 50 ms pro Modell. Der Flaschenhals ist bei teuren Fittern (L-BFGS-MLE wie GEV, StudentT, GeneralizedGamma, BurrXII) deutlich höher — dort sollte `B` kleiner gewählt oder `-bootstrap-model=best` verwendet werden. `duration_ms` im Bootstrap-Objekt bzw. die Runtime in der Textausgabe zeigt die tatsächlich verbrauchte Zeit.

Praxisbeispiel (Apple M3, `winequality-red.csv`, `density`, n = 1599, `B = 200`, `-bootstrap-model=all`): alle 19 erfolgreichen Modelle in **1,3 s** Wandzeit (~8 s CPU über 8 Kerne). Pro Modell für B = 200: die meisten unter 25 ms, die L-BFGS-Fitter deutlich teurer — GEV ca. 420 ms, SkewNormal ca. 360 ms, GeneralizedGamma ca. 120 ms, StudentT ca. 60 ms. Ein Lauf mit `B = 1000` über alle Modelle liegt damit grob im einstelligen Sekundenbereich; bei `B = 10000` sollte `-bootstrap-model=best` oder eine `-include`-Einschränkung gewählt werden.

## Dip-Test (Unimodalität)

Mit `-dip` wird zusätzlich zu den Modellfits Hartigans Dip-Test auf die (bereinigte) Eingabestichprobe ausgeführt. Der Test prüft die Nullhypothese „die Daten stammen aus einer unimodalen Verteilung“: Die Dip-Statistik misst den maximalen Abstand zwischen empirischer Verteilungsfunktion und der bestmöglichen unimodalen Anpassung (0 bis 0.25); der p-Wert wird per Monte-Carlo-Simulation gegen die Gleichverteilung geschätzt — kleine p-Werte sprechen gegen Unimodalität (z. B. bimodale Daten). Der Test ist eine Eigenschaft der *Daten*, nicht des gewählten Modells: Er ersetzt keine Modellgütemetriken, kann aber vor der Modellwahl vor einer nicht-unimodalen Stichprobe warnen.

```bash
distfit fit werte.txt -dip

# Reproduzierbarer Lauf mit mehr Simulationen
distfit fit werte.txt -dip -dip-simulations=10000 -dip-seed=42
```

Im Textbericht erscheint ein Abschnitt „Dip test (Hartigan)“ (Dip und p-Wert samt Simulationszahl und Seed), in der JSON-Ausgabe das optionale Feld `dip_test` (siehe [JSON.md](JSON.md)). Die `-dip`-Unterflags ohne `-dip` sind ein Aufruffehler (Exit `2`), ebenso `-dip` mit `-format=plot`, weil das Dashboard das Ergebnis nicht anzeigt.

Der p-Wert ist Monte-Carlo: mit `-dip-seed` ist er pro Datenstand reproduzierbar, seine Auflösung liegt bei `1/(simulationen+1)` (Standard `2000` → kleinster unterscheidbarer Wert ≈ 0,0005). Für sehr große Stichproben (`n` im sechsstelligen Bereich) kann `-dip-simulations` reduziert werden; die Statistik selbst ist auch für kleine `n` (≥ 5) definiert, verliert aber an Trennschärfe.

**Nullwerte:** `-dip` läuft immer auf den rohen Werten einschließlich Nullen, unabhängig von `-zero-inflated`. Eine Punktmasse bei 0 wirkt wie eine eigene Mode und kann die Unimodalität-Hypothese allein schon verwerfen — bei Zero-inflated-Daten ist ein kleiner p-Wert daher erwartbar und kein Widerspruch zum ZI-Modell. Die Aussage „die positiven Werte sind unimodal" liefert der Test so nicht; dafür ist die Stichprobe vorher auf `> 0` zu filtern.

## Ausgabeformate

### `text` (Standard)

Ohne `-all` gibt es eine Zusammenfassung des besten Modells. Mit `-distribution` ist das das gewählte Modell:

```text
$ distfit fit werte.txt -distribution=weibull
Warnung: hohe Parameterunsicherheit bei n=10 (ab n>20 sind die Schätzungen stabiler); Parameter und AIC-Ranking sind nur eingeschränkt belastbar
=== GODIST FITTER ERGEBNIS ===
Datenpunkte:    10
Beste Kurve:    Weibull
Parameter:      scale=2.6, shape=3.35
AIC / BIC:      26.69 / 27.29
KS-Statistik:   0.1153 (RMSE: 0.0332)

Descriptive statistics
----------------------
...
```

Mit `-all` gibt es Kopf, deskriptive Statistik, das beste Modell und eine Vergleichstabelle:

```text
$ distfit fit werte.txt -include=weibull,gamma -all
GODIST — Distribution Analysis

Input:        werte.txt
Observations: 10
Candidates:   2 (2 successful, 0 failed)
Criterion:    AIC (lower is better)
...
Best fit
--------
Distribution: Gamma
...
Model comparison
+------+--------------+-------------------------+-------+-----------+-------+--------+--------+
| Rank | Distribution | Parameters              | AIC   | Delta AIC | BIC   | KS     | RMSE   |
+------+--------------+-------------------------+-------+-----------+-------+--------+--------+
| 1    | Gamma        | alpha=9.21, scale=0.253 | 26.35 | 0.00      | 26.95 | 0.1147 | 0.0316 |
| 2    | Weibull      | scale=2.6, shape=3.35   | 26.69 | 0.34      | 27.29 | 0.1153 | 0.0332 |
+------+--------------+-------------------------+-------+-----------+-------+--------+--------+
```

Fehlgeschlagene Fits stehen nach der Tabelle unter `Models that could not be fitted:`. Die Anzahl steht in `Candidates` (z. B. `14 (8 successful, 6 failed)`).

### `json`

Nur mit `-all`. Die Struktur ist in [JSON.md](JSON.md) beschrieben, das Schema in `docs/json.schema.json`. Das Feld `schema_version` ist aktuell `1`.

```bash
distfit -file=werte.txt -all -format=json
```

### `plot`

Nur mit `-all`. Startet ein lokales Dashboard auf `127.0.0.1` mit freiem Port, öffnet es im Standardbrowser und läuft bis `Strg+C`. Die URL steht auf stderr, stdout bleibt leer. Anfragen mit einem anderen Host-Header als `127.0.0.1` oder `localhost` werden abgelehnt.

Das Dashboard zeigt Histogramm, Dichte, empirische und theoretische CDF sowie Q-Q-Plots. Über Checkboxen lassen sich Modelle ein- und ausblenden. Das Feld „Klassen“ setzt die Histogrammklassen (1 bis 500); „Automatisch“ stellt die Freedman-Diaconis-Wahl wieder her. Die Daten werden dafür über `GET /api/plot?bins=N` neu berechnet.

## Beispiele

Die folgenden Beispiele verwenden zwei Testdateien. Sie lassen sich mit diesen Befehlen erzeugen:

```bash
printf "1.2\n2.3\n1.9\n3.1\n2.7\n1.5\n2.2\n3.8\n2.9\n1.7\n" > werte.txt

printf "wind;density;citric\n5.1;1.2;0\n6.3;1.4;0.5\n4.8;1.1;0\n7.2;1.6;1.2\n5.9;1.3;0.3\n6.1;1.5;0\n4.4;1.0;2.1\n5.5;1.2;0.8\n" > messwerte.csv
```

### Schnellstart

```bash
# Bestes Modell nach AIC (Standard)
distfit fit werte.txt

# Vergleich aller Kandidaten
distfit fit werte.txt -all

# Gleichwertig in der Flag-Form
distfit -file=werte.txt -all
```

### Eine Verteilung fitten

```bash
distfit fit werte.txt -distribution=weibull
distfit fit werte.txt --distribution=Weibull     # Doppelstrich und Großschreibung sind erlaubt
distfit fit werte.txt -distribution=weibull -all # Tabelle mit genau dieser Verteilung
```

### Kandidaten einschränken

```bash
# Nur zwei Verteilungen vergleichen
distfit fit werte.txt -include=weibull,gamma -all

# Duplikate werden entfernt: nur Gamma und Weibull
distfit fit werte.txt -include=gamma,gamma,WEIBULL -all

# Alles außer Cauchy und Laplace vergleichen
distfit fit werte.txt -exclude=cauchy,laplace -all
```

### CSV-Dateien

```bash
# Mehrere numerische Spalten: Spalte per Name wählen
distfit -file=messwerte.csv -column=density -all

# Spalte per 0-basiertem Index
distfit -file=messwerte.csv -column=1 -all

# Dezimalkomma statt Punkt
distfit -file=messwerte_de.csv -column=density -decimal=, -all
```

### Nullwerte mit Zero-Inflation

```bash
# Alle ZI-fähigen Kandidaten vergleichen
distfit -file=messwerte.csv -column=citric -zero-inflated -all

# Ein einzelnes ZI-Modell fitten
distfit fit messwerte.csv -column=citric -zero-inflated -distribution=gamma
```

Die Ausgabe enthält `Beste Kurve: ZI-Gamma` und den Parameter `p_zero`.

### JSON für andere Systeme

```bash
distfit -file=messwerte.csv -column=density -all -format=json > ergebnis.json
distfit fit werte.txt -distribution=gamma -all -format=json
```

Ohne `-all` ist JSON nicht verfügbar: `distfit fit werte.txt -distribution=gamma -format=json` gibt `-format=json erfordert -all` aus (Exit `2`).

### Modellliste

```bash
distfit list distributions
distfit list distributions -format=json
```

### Plot-Dashboard

```bash
distfit -file=werte.txt -all -format=plot
```

## Fehler und Exit-Codes

| Exit | Bedeutung |
|------|-----------|
| `0` | Erfolg |
| `1` | Daten- oder Fitting-Fehler, z. B. Datei nicht lesbar, mehrdeutige CSV ohne `-column`, zu wenige Werte |
| `2` | Aufruffehler, z. B. ungültige oder fehlende Flags, unbekannte Verteilung |

Typische Fehlermeldungen mit Exit-Code:

| Aufruf | Meldung | Exit |
|--------|---------|------|
| `fit werte.txt -distribution=wibull` | `unbekannte Verteilung "wibull" (gültig: Weibull, LogNormal, …)` | `2` |
| `fit werte.txt -distribution=weibull -include=gamma` | `-distribution schließt -include und -exclude aus` | `2` |
| `fit werte.txt -include=weibull -exclude=gamma` | `include und exclude schließen sich aus` | `2` |
| `fit werte.txt -distribution=normal -zero-inflated` | `Normal unterstützt keine Zero-Inflation` | `2` |
| `fit werte.txt -file=werte.txt` | `Datei entweder als Argument oder mit -file angeben, nicht beides` | `2` |
| `fit` | `bitte eine Datei angeben: -file=daten.csv oder distfit fit daten.csv` | `2` |
| `-file=werte.txt -format=json` | `-format=json erfordert -all` | `2` |
| `list distributions -format=xml` | `ungültiges -format "xml" (erlaubt: text, json)` | `2` |
| `-file=werte.txt -column=0` | `-column wird nur für CSV-Dateien unterstützt` | `2` |
| `-file=mehrdeutig.csv` | `mehrere numerische Spalten gefunden (…); bitte mit -column eine Spalte wählen` | `1` |
| `-file=kurz.txt` mit 3 Werten | `zu wenige Werte: n=3, mindestens 5 erforderlich` | `1` |

Bei Aufruffehlern wird zusätzlich die Verwendung ausgegeben.

## Mindeststichprobe und Warnungen

| Bedingung | Verhalten |
|-----------|-----------|
| `n < 5` | Abbruch mit Fehlermeldung, Exit `1` |
| konstante Daten (alle Werte identisch) | Abbruch mit Fehlermeldung, Exit `1` |
| `5 ≤ n ≤ 20` | Fit läuft; Warnung auf stderr wegen hoher Parameterunsicherheit |
| `n > 20` | Fit läuft ohne Warnung |

Die Grenzen sind in `internal/cli/validate.go` als `MinSampleSize` (5) und `HighUncertaintySampleSize` (20) festgelegt. Sie sind eine vorläufige Entscheidung; die Warnung soll die Belastbarkeit der Parameter und des AIC-Rankings kennzeichnen.

## Geplant

Noch nicht umgesetzt:

- JSON-Schema für `list distributions -format=json` in `docs/json.md` und `docs/json.schema.json`.
- Python-Consumer-Test für `--distribution` und `-include`/`-exclude` in `consumers/python/`.
- Golden-Test für die Textausgabe von `list distributions`.
