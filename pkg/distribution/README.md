## Migration

Der Distribution-Ordner wird als eigenständiges Modul ausgelagert.
Die Migration der bestehenden Dateien in ein eigenes Repository kann umstandslos erfolgen, da distribution keine anderen packages importiert.
Externe Abhängigkeiten basieren ausschließlich auf Gonum.
Entsprechend muss dann auch die go.mod konfiguriert werden.

Was zunächst gemacht werden muss, Unit-Tests hinzuzufügen, die fehlen bisher komplett, weitere Verteilungen hinzufügen, in Frage kommen Exponential, Hyper- und Hypoexponential, Student-t, Pareto, Cauchy, Chi-Quadrat, Log-Logistic, Gumbel

Anschließend werde ich mich mit der Möglichkeit befassen, das Projekt als CLI-Tool aufzusetzen für verschiedene Eingabeformate wie JSON oder CSV.

Ausgabeformat könnte Text oder JSON-Output sein.

Abschließend muss ich für alle implementierten Verteilungen geeignete Beispiele finden, um die Funktionsfähigkeit an realen Daten zu testen.