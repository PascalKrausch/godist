# Roadmap

Die Arbeit sollte zunächst die Zuverlässigkeit der vorhandenen Verteilungen absichern und anschließend die CLI und den Verteilungskatalog ausbauen. Das Zielinterface der CLI ist in [cli.md](cli.md#geplantes-zielinterface) beschrieben.

## 1. Statistischen Kern validieren — höchste Priorität

- Für jede Verteilung PDF, CDF und LogPDF prüfen: Wertebereiche, endliche Ergebnisse und konsistentes Verhalten an den Support-Grenzen.
- Synthetische Datensätze mit bekannten Parametern erzeugen und die Schätzungen gegen erwartete Werte oder eine unabhängige Referenzimplementierung prüfen.
- Log-Likelihood und AIC/BIC anhand unabhängiger Berechnungen validieren.
- Sicherstellen, dass `k` alle tatsächlich frei geschätzten Parameter zählt.
- Konvergenzstatus und Optimierungsfehler explizit erfassen.
- Verhindern, dass ein Fallback auf Momentenschätzer fälschlich als erfolgreicher MLE ausgegeben wird.
- Startwerte, Parametergrenzen und Support-Bedingungen für jeden Fitter dokumentieren.

## 2. Randfälle und Datenvalidierung

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

## 3. Automatisierte Tests ausbauen

- Unit-Tests für alle neun Verteilungen und Fitter sind in `pkg/distribution` vorhanden; ergänzt werden Grenzfälle gemäß Punkt 2.
- Eigenschaften testen: CDF ist monoton und liegt in `[0, 1]`; PDF ist auf dem Support nicht negativ; LogPDF stimmt numerisch mit `log(PDF)` überein, soweit die Dichte nicht unterläuft.
- Geeignete numerische Integrations- und Grenzwerttests für die Dichte implementieren.
- Regressionstests mit festen Beispieldatensätzen und erwarteten Ergebnissen ergänzen.
- Tests für Modellranking, AIC-Parameterzählung und fehlgeschlagene Fits hinzufügen.
- CLI-Tests für Textausgabe, JSON-Schema, Exit-Codes, CSV-Spaltenauswahl und ungültige Eingaben ergänzen.
- Testläufe mit `go test ./...` in die Entwicklungsroutine integrieren.

## 4. CLI und Einleselogik vervollständigen

- Unterbefehle `fit` und `list distributions` implementieren.
- `--column` nach Spaltenname und – wenn sinnvoll – nach Index unterstützen.
- CSV-Dateien mit mehreren numerischen Spalten niemals ungefragt zu einem einzigen Datensatz vermischen.
- Für mehrdeutige Eingaben eine klare Fehlermeldung mit verfügbaren Spalten anzeigen.
- Unterstützung für Standard-Input (`-`) und klare Regeln für leere Zeilen, Header, Trennzeichen und Dezimalformate ergänzen.
- `--distribution` zum Fit einer einzelnen Verteilung anbieten.
- Fehler und Warnungen verständlich ausgeben und geeignete Exit-Codes definieren.
- Ausgabeparameter mit stabilen, verteilungsspezifischen Namen bereitstellen.
- JSON-Schema dokumentieren und mit Tests gegen unbeabsichtigte Änderungen absichern (siehe [json.md](json.md)).

## 5. Neue Verteilungen priorisiert hinzufügen

Mögliche Erweiterungen:

| Priorität | Verteilungen | Hinweise |
|---|---|---|
| Früh | Logistic, Cauchy | Ergänzen symmetrische Modelle mit unterschiedlichen Tail-Eigenschaften; Cauchy benötigt besondere Behandlung, da gewöhnliche Momente nicht existieren. |
| Früh bis mittel | Log-logistisch, Pareto | Nützlich für positive Daten und schwere rechte Tails. |
| Mittel | Student-t | Symmetrisch mit einstellbarer Tail-Schwere; Freiheitsgrade müssen robust geschätzt werden. |
| Mittel | Chi-Quadrat | Spezialfälle beziehungsweise eng mit vorhandenen Familien verwandt; als eigener Verteilungstyp dennoch praktisch. |
| Später | Generalized Extreme Value (GEV) | Wertvoll für Extremwertanalysen, aber mit parameterabhängigem Support und anspruchsvoller numerischer Optimierung. |

Jede neue Verteilung benötigt neben PDF/CDF/LogPDF auch einen validierten Fitter, dokumentierte Parameter und Support-Bedingungen sowie eigene Tests. Die Implementierung über `ContinuousDistribution` allein reicht nicht aus, um die Qualität der Parameterschätzung sicherzustellen.

## 6. Performance und GC-Allokationen messen

Performanceoptimierungen sollen auf Benchmarks beruhen und erst nach der fachlichen Validierung erfolgen.

- Benchmarks für jeden Fitter und den vollständigen Modellvergleich erstellen.
- Datensätze unterschiedlicher Größen verwenden, zum Beispiel 100, 10.000 und 1.000.000 Beobachtungen, sofern für den jeweiligen Benchmark sinnvoll.
- Laufzeit und Allokationen mit `go test -bench=. -benchmem ./...` messen (siehe [evaluation.md](evaluation.md)).
- Hotspots bei wiederholten Schleifen, temporären Slices, Interface-Aufrufen und wiederholter Berechnung gemeinsamer Ausdrücke profilieren.
- Ergebnisse vor und nach Optimierungen vergleichen und Korrektheitstests erneut ausführen.
- Keine schwerer lesbare oder fehleranfälligere Implementierung allein wegen vermuteter GC-Einsparungen einführen.

## 7. Dokumentation und Release-Vorbereitung

- README-Beispiele mit tatsächlich ausführbaren Befehlen synchron halten.
- Parameterkonventionen und Support jeder Verteilung dokumentieren.
- Grenzen von AIC, BIC, KS und RMSE erklären (siehe [methodik.md](methodik.md)).
- Versionierung und kompatible Änderungen der Bibliotheks-API festlegen.
- CI für Tests, Formatierung und gegebenenfalls statische Analyse ergänzen.
- Erst nach bestandenen Validierungs- und Regressionstests einen produktiven Einsatz empfehlen.

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

