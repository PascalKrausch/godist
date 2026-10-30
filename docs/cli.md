# CLI

Die CLI `distfit` liest Dateien, fittet alle Kandidaten und wählt standardmäßig das beste Modell nach AIC. Die Auswahl- und Gütekriterien sind in [methodik.md](methodik.md) beschrieben, die JSON-Ausgabe in [json.md](json.md).

## Aktueller Prototyp

Der Prototyp liest Dateien und wählt standardmäßig das beste Modell nach AIC aus.

| Flag | Bedeutung |
|------|-----------|
| `-file` | Pflicht. Endung `.csv`, `.json` oder `.txt` |
| `-column` | Nur CSV: Spaltenname oder 0-basierter Index. Ohne Angabe muss genau eine numerische Spalte vorhanden sein |
| `-decimal` | `.` (Standard) oder `,` für Dezimalkommas |
| `-all` | Alle Kandidaten vergleichen, inklusive fehlgeschlagener Fits |
| `-format` | `text` (Standard), `json` oder `plot`; `json` und `plot` erfordern `-all` |
| `-zero-inflated` | Nullwerte als Zero-Inflated-Modell (Punktmasse bei 0) modellieren. Verglichen werden nur `ZI-`-Kandidaten (Weibull, Gamma, Exponential, LogNormal, Rayleigh, Pareto). Standard: Fast-Fail-Policy, Kandidaten mit Träger (0, ∞) scheitern bei Nullwerten. Die AIC-Werte beider Modi sind nicht miteinander vergleichbar. |

Unterstützte Eingabeformate sind `.txt` (ein Wert pro Zeile), `.json` (Array von Zahlen) und `.csv` (Semikolon als Trennzeichen; die erste Zeile gilt als Header, wenn sie keine Zahlen enthält).

`-format=plot` startet ein interaktives Dashboard auf `127.0.0.1` mit einem freien Port, öffnet es im Standardbrowser und läuft bis `Strg+C`. Es zeigt Histogramm, Dichte, empirische und theoretische CDF sowie Q-Q-Plots für alle angepassten Modelle. Die URL wird auf stderr ausgegeben, stdout bleibt leer. Anfragen mit einem anderen Host-Header als `127.0.0.1` oder `localhost` werden abgelehnt.

Im Dashboard lassen sich über Checkboxen einzelne Modelle ein- und ausblenden (in PDF, CDF und Q-Q-Plots). Das Feld „Klassen“ setzt die Anzahl der Histogrammklassen von 1 bis 500; „Automatisch“ stellt die Freedman-Diaconis-Wahl wieder her. Die Daten werden dafür über `GET /api/plot?bins=N` neu berechnet.

Verworfene Eingaben (leere Zellen, Text, `NaN`, `±Inf`, `null`) werden nicht stillschweigend übernommen: Die CLI meldet sie als Warnung auf stderr, und `Observations` bzw. `Datenpunkte` zeigt die Anzahl verworfener Werte. Im JSON-Bericht steht die Anzahl im Feld `discarded`.

Exit-Codes: `0` Erfolg, `1` Daten- oder Fitting-Fehler (inklusive mehrdeutiger CSV ohne `-column` und zu kleiner oder konstanter Stichproben), `2` Aufruffehler (ungültige Flags).

### Beispiele

```bash
./distfit -file=measurements.txt -all
./distfit -file=measurements.txt -all -format=json
./distfit -file=measurements.txt -all -format=plot

# Spalte nach Name auswählen
./distfit -file=winequality-red.csv -column="alcohol" -all

# Spalte nach Index auswählen (0-basiert)
./distfit -file=winequality-red.csv -column=10 -all

# Dezimalkomma
./distfit -file=messwerte.csv -column=2 -decimal=, -all -format=json

# Ohne -column schlägt winequality-red.csv fehl (mehrere numerische Spalten)
./distfit -file=winequality-red.csv
```

### Mindeststichprobe (Fast-Fail-Policy)

Die CLI prüft die Stichprobengröße `n` vor dem Fit:

| Bedingung | Verhalten |
|-----------|-----------|
| `n < 5` | Abbruch mit Fehlermeldung, Exit-Code `1` |
| konstante Daten (alle Werte identisch) | Abbruch mit Fehlermeldung, Exit-Code `1` |
| `5 ≤ n ≤ 20` | Fit läuft; Warnung auf stderr wegen hoher Parameterunsicherheit |
| `n > 20` | Fit läuft ohne Warnung |

Die Grenzen sind in `internal/cli/validate.go` als `MinSampleSize` (5) und `HighUncertaintySampleSize` (20) festgelegt. Die Schwellen sind eine vorläufige Entscheidung; die Warnung soll die Belastbarkeit der Parameter und des AIC-Rankings kennzeichnen.

## Geplantes Zielinterface

Die endgültige CLI-Oberfläche soll mit Unterbefehlen leichter auffindbar und erweiterbar werden. Die folgenden Befehle sind das Zielinterface; sie sind nicht zwingend bereits in jedem Build verfügbar.

### Automatische Modellwahl

```bash
godist fit measurements.csv --column wind_speed_10m
```

Der Befehl soll:

1. die gewählte Spalte einlesen und validieren,
2. die verfügbaren Kandidaten fitten,
3. fehlgeschlagene Fits gesondert ausweisen,
4. erfolgreiche Modelle nach AIC sortieren,
5. Gewinner, Parameter und Vergleichsmetriken ausgeben.

### Verfügbare Verteilungen auflisten

```bash
godist list distributions
```

Die Liste soll neben dem Verteilungsnamen auch Parameter, Support (zulässiger Wertebereich), eine kurze Beschreibung und gegebenenfalls besondere Fit-Anforderungen nennen.

### Eine bestimmte Verteilung erzwingen

```bash
godist fit measurements.csv --column wind_speed_10m --distribution weibull
```

Wenn eine Verteilung explizit angegeben wird, soll der Befehl genau dieses Modell fitten und dessen Kennzahlen ausgeben, statt es gegen alle anderen Kandidaten antreten zu lassen.

## Gewünschte Textausgabe

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
