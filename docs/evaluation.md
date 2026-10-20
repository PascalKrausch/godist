# Evaluation: Vergleich und Benchmarks mit SciPy

Zur Evaluation der Implementierungen wurde die Python-Bibliothek SciPy als Referenz hinzugezogen.

## Vergleich mit SciPy

`evaluation/compare.py` vergleicht alle neun Verteilungen mit SciPy. Das Skript muss aus dem Repository-Wurzelverzeichnis gestartet werden, da es `go run ./cmd/distfit` aufruft:

```bash
evaluation/.venv/bin/python evaluation/compare.py --path=testdata/winequality-red.csv --column=density
```

Pfad und Spalte lassen sich anpassen. Im Virtualenv werden `scipy` und `pandas` benötigt. Das Skript baut die CLI vorab einmal (`go build`), sodass die Build-Zeit nicht in den Vergleich einfließt.

## Benchmarks: godist gegen SciPy

`evaluation/bench.py run` misst drei Dinge und speichert die Ergebnisse unter `evaluation/bench_results/<Zeitstempel>/` (Verzeichnis ist nicht versioniert):

```bash
evaluation/.venv/bin/python evaluation/bench.py run --repeat 10
```

- **Reine Algorithmuszeit:** `go test -bench` (`go-bench.txt`, Auswertung mit `benchstat`) und `timeit`-Messungen in Python (`python-algo.json`). Gemessen wird nur der Fit bzw. `SelectAllModels`/`AllModels`, ohne Einlesen und Prozessstart.
- **Speicher:** `B/op` und `allocs/op` aus `-benchmem` für Go, `tracemalloc`-Peak für Python (`python-mem.json`). Beide Werte sind nicht direkt vergleichbar: Go gibt die insgesamt allokierte Menge pro Operation an, Python den Spitzenwert. `tracemalloc` erfasst keinen Speicher, den SciPy-C-Code außerhalb des Python-Allokators belegt.
- **Gesamtlaufzeit und Prozess-RSS:** Wandzeit und Peak-RSS der kompletten Prozesse (Einlesen, alle Modelle, Metriken) für Go-CLI und Python-Gegenstück (`e2e.json`). Der Python-Prozess enthält die Importkosten von numpy, pandas und scipy.

Alle Messungen nutzen dieselben synthetischen Daten (100, 10.000 und 1.000.000 Werte, SplitMix64 mit Seed 42). `environment.json` hält Hardware, Versionen und Commit fest; `git_dirty` zeigt uncommittete Änderungen an. Vor einer Messung sollten Hintergrundlasten minimiert und das Gerät an der Steckdose betrieben werden.
