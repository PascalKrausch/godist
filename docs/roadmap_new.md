# Roadmap – Von Distribution Fitting zu Distribution Diagnostics

Dieses Dokument beschreibt die geplante Weiterentwicklung von `godist` und `distfit`.

Der bisherige Schwerpunkt lag auf der Implementierung und Optimierung parametrischer, stetiger Verteilungen sowie der automatisierten Auswahl des statistisch am besten passenden Modells.

Der Entwicklungsschwerpunkt verschiebt sich nun in Richtung **statistischer Diagnostik, Modellvalidierung und Unsicherheitsanalyse**.

Das langfristige Ziel ist nicht mehr ausschließlich, die bestpassende Verteilung zu identifizieren, sondern zusätzlich beurteilen zu können:

- Wie plausibel ist die Annahme einer einzelnen parametrischen Verteilung?
- Wie stabil sind die geschätzten Modellparameter?
- Wie empfindlich ist die Modellauswahl gegenüber Stichprobenvariation?
- Welche Eigenschaften der Daten werden durch das Modell möglicherweise nicht erfasst?
- Wie zuverlässig sind daraus abgeleitete Wahrscheinlichkeiten und Risikokennzahlen?

Die bisherige Roadmap (`roadmap_old.md`) bleibt zur Dokumentation der Projektentwicklung erhalten.

---

## 1. Gegenwärtiger Funktionsumfang

Der numerische Kern und die grundlegende Anwendungsarchitektur sind weitgehend implementiert.

- **Verteilungsbibliothek:** 20 parametrische Verteilungen mit gemeinsamer Schnittstelle. Modelle, Parameter und Träger sind in der `README.md` dokumentiert.
- **Zero-Inflated-Modellierung:** Erweiterung der kontinuierlichen Verteilungsmodelle um eine separate Punktmasse bei null.
- **Datenimport:** Unterstützung von CSV- und JSON-Eingabedaten.
- **CLI:** Auswahl von Eingabedatei und Datenkolumne sowie explizite Auswahl und Ausschluss einzelner Verteilungen über CLI-Parameter. Details sind in `cli.md` dokumentiert.
- **Ausgabeformate:** Strukturierte Terminalausgabe, JSON-Export und interaktives Web-Dashboard auf Basis von uPlot.
- **Statistische Auswertung:** Deskriptive Kennzahlen, Parameterschätzungen mittels Maximum Likelihood, AIC/BIC, KS-Statistik, CDF-RMSE und QQ-Plots.
- **Performanceanalyse:** Laufzeitmessungen sowohl für die Gesamtanalyse als auch für einzelne Modellanpassungen.
- **Bootstrap:** Parametrische und nichtparametrische Verfahren zur Untersuchung der Parameterunsicherheit und Berechnung von Konfidenzintervallen.

Bei typischen Datensatzgrößen von 1.000 bis 10.000 Beobachtungen liegt die Laufzeit der Modellanpassung ohne Bootstrap in der Regel im Millisekundenbereich. Die tatsächliche Laufzeit hängt von Datengröße, Modellauswahl und Optimierungsverfahren ab.

Bei 1.000 Bootstrap-Replikationen kann sich die zusätzliche Auswertungszeit je nach Modell und Stichprobenumfang auf etwa 1–3 Sekunden erhöhen.

---

## 2. Geplante Erweiterungen

### 2.1 Multimodalitätsdiagnostik

**Problemstellung**

Die bisherige Modellbibliothek ist darauf ausgelegt, einzelne parametrische Verteilungen an einen Datensatz anzupassen. Eine explizite Modellierung mehrerer Teilpopulationen durch Mischverteilungen ist derzeit nicht vorgesehen.

Bei heterogenen Daten, beispielsweise zusammengefassten Schnabelgrößen unterschiedlicher Pinguinarten, kann eine einzelne parametrische Verteilung die zugrunde liegende Datenstruktur nur unzureichend beschreiben.

Bevor eine Modellanpassung durchgeführt wird, sollte daher untersucht werden, ob die Daten Hinweise auf mehrere lokale Häufungsbereiche aufweisen.

**Geplanter Ansatz**

Als erste explorative Diagnostik soll eine Kernel Density Estimation (KDE) implementiert werden. Die geglättete empirische Dichte ermöglicht eine visuelle und algorithmische Identifikation lokaler Dichtemaxima.

Eine FFT-basierte Implementierung könnte die Berechnung insbesondere bei größeren Datensätzen beschleunigen. Die Anzahl der gefundenen Modi soll zusätzlich über verschiedene Bandbreiten untersucht werden, um die Abhängigkeit der Modenzählung von der Glättung sichtbar zu machen.

Als ergänzende statistische Diagnostik ist Hartigans Dip Test (Hartigan & Hartigan, 1985) vorgesehen. Dieser prüft die Nullhypothese einer unimodalen Verteilung. Er liefert eine statistische Aussage über die Vereinbarkeit der Daten mit Unimodalität, bestimmt jedoch nicht unmittelbar die Anzahl der Modi.

**Methodische Herausforderungen**

- Die Modenzählung mittels KDE hängt wesentlich von der gewählten Bandbreite ab.
- Zu geringe Glättung kann durch Datenrauschen zusätzliche Maxima erzeugen; zu starke Glättung kann reale Strukturen verdecken.
- Der Dip Test muss hinsichtlich Implementierung und p-Wert-Kalibrierung gründlich validiert werden.
- Eine festgestellte Abweichung von Unimodalität darf nicht automatisch zum Ausschluss einzelner Modelle führen.

Langfristig könnte aus dieser Diagnostik eine separate Erweiterung für explizite Mischverteilungen entstehen.

### 2.2 Explorative Verteilungsdiagnostik – Cullen-Frey-Diagramm

**Problemstellung**

Die direkte Betrachtung und der Vergleich aller implementierten Verteilungen kann insbesondere bei einer größeren Anzahl von Modellkandidaten unübersichtlich werden.

Eine vorgelagerte explorative Analyse der empirischen Verteilungscharakteristik könnte bereits vor der numerischen Anpassung Hinweise auf plausible Verteilungsfamilien liefern.

**Geplanter Ansatz**

Implementierung eines Cullen-Frey-Diagramms auf Basis von:

- quadrierter empirischer Schiefe (Skewness²)
- empirischer Pearson-Kurtosis

Die empirischen Kennzahlen werden den theoretischen Momentverhältnissen verschiedener Verteilungsfamilien gegenübergestellt.

Für Verteilungen mit freien Formparametern können theoretische Kurven oder Wertebereiche über zulässige Parameterkombinationen dargestellt werden.

Ergänzend könnte eine Bootstrap-Punktwolke die Stichprobenunsicherheit der empirischen Momentverhältnisse visualisieren.

**Methodische Herausforderungen**

- Nicht alle Verteilungen besitzen endliche dritte und vierte Momente.
- Die theoretischen Momente von Zero-Inflated-Modellen müssen die Punktmasse bei null berücksichtigen.
- Die Position im Diagramm ist kein eindeutiges Identifikationsmerkmal einer Verteilungsfamilie.

Das Cullen-Frey-Diagramm soll daher als exploratives Hilfsmittel zur Eingrenzung plausibler Modellkandidaten dienen, nicht als automatisches Auswahl- oder Ausschlusskriterium.

### 2.3 Modellunsicherheit – Akaike Weights

**Problemstellung**

Derzeit wird das beste Modell anhand des niedrigsten AIC ausgewählt. Diese Vorgehensweise liefert einen eindeutigen Ranglistenführer, bildet aber die relative Unterstützung der übrigen Kandidaten nicht vollständig ab.

Insbesondere bei vergleichbaren AIC-Werten kann die alleinige Betrachtung des Siegers eine scheinbare Eindeutigkeit suggerieren.

**Geplanter Ansatz**

Berechnung und Ausgabe der Akaike Weights auf Grundlage der bereits ermittelten AIC-Differenzen:

\[
w_i =
\frac{\exp(-\Delta_i/2)}
{\sum_j \exp(-\Delta_j/2)}
\]

Die Gewichte beschreiben die relative Unterstützung der Modelle innerhalb der betrachteten Kandidatenmenge.

Als weiterführende Möglichkeit ist eine modellgemittelte Prognose beziehungsweise Verteilungsfunktion denkbar:

\[
f_{\mathrm{avg}}(x)=\sum_i w_i f_i(x)
\]

**Ziel**

Erweiterung der bisherigen eindimensionalen Modellrangliste um eine quantitative Einschätzung der Modellunsicherheit.

Akaike Weights sind dabei keine absoluten Wahrscheinlichkeiten dafür, dass ein Modell wahr ist. Sie hängen von der betrachteten Modellmenge und der Vergleichbarkeit der verwendeten Likelihoods ab.

### 2.4 Zensierte und trunkierte Daten

**Problemstellung**

In vielen realen Anwendungen liegen Beobachtungen nicht vollständig vor.

Beispiele sind Lebensdauerdaten, bei denen Maschinen bis zum Ende eines Beobachtungszeitraums nicht ausgefallen sind, sowie Konzentrationsmessungen, bei denen Werte unterhalb einer Nachweisgrenze nicht in den Datensatz aufgenommen werden.

Werden solche Daten wie gewöhnliche vollständige Beobachtungen behandelt, können die Parameterschätzungen erheblich verzerrt werden.

**Geplanter Ansatz**

Erweiterung der Datenstruktur und Likelihood-Berechnung um explizite Informationen zu:

- rechtszensierten Beobachtungen
- linkszensierten Beobachtungen
- intervallzensierten Beobachtungen
- links- und rechtstrunkierten Stichproben

Die bestehenden Verteilungsmodelle sollen hierfür geeignete Likelihood-Beiträge bereitstellen.

**Ziel**

Erweiterung des Anwendungsbereichs insbesondere auf Zuverlässigkeitsanalyse, Lebensdauerstatistik und Messdaten mit Nachweisgrenzen.

Zensierung und Trunkierung müssen dabei getrennt behandelt werden, da sie unterschiedliche statistische Beobachtungsmechanismen beschreiben.

### 2.5 Quantil- und Risikomaße

**Problemstellung**

Die bisherige Bewertung konzentriert sich primär auf Verteilungsparameter und globale Gütemaße. In vielen Anwendungsfällen sind jedoch konkrete Aussagen über seltene Ereignisse und Verteilungsschwänze relevanter als Parameter wie Shape oder Scale.

**Geplanter Ansatz**

Ergänzung anwendungsorientierter Kennzahlen:

- Quantile und Extremquantile
- Überschreitungswahrscheinlichkeiten
- Value at Risk (VaR)
- Expected Shortfall (ES)

Die Berechnung erfolgt auf Basis der angepassten Verteilungsfunktionen und Quantilfunktionen.

**Ziel**

Erweiterung der Bibliothek von der reinen Verteilungsanpassung hin zu einer quantitativen Bewertung seltener Ereignisse und extremer Beobachtungen.

**Methodische Herausforderungen**

Die Existenz und Endlichkeit von Risikomaßen hängt von den Eigenschaften der jeweiligen Verteilung ab. Insbesondere bei heavy-tailed distributions können Erwartungswerte und Expected Shortfall unendlich oder nicht definiert sein.

Die verwendete Tail-Konvention und die Interpretation der Risikomaße müssen deshalb explizit dokumentiert werden.

### 2.6 UX-Verbesserungen – Reduktion visueller Komplexität

**Problemstellung**

Derzeit werden im Dichte- und CDF-Dashboard zahlreiche erfolgreich angepasste Verteilungen gleichzeitig dargestellt. Bei 20 Modellkandidaten entsteht dadurch schnell visuelle Überladung.

Die wichtigsten Informationen, insbesondere die empirische Datenstruktur und die Anpassung des bestplatzierten Modells, gehen dabei teilweise unter.

**Geplanter Ansatz**

Reduktion der standardmäßig sichtbaren Kurven im Dashboard:

- Empirisches Histogramm
- Nichtparametrische KDE
- PDF des AIC-Sieger-Modells

Alle weiteren angepassten Verteilungen bleiben verfügbar, werden jedoch zunächst ausgeblendet und können über individuelle Toggle-Elemente zugeschaltet werden.

Die Nullwertmasse von Zero-Inflated-Modellen soll separat visualisiert werden, da sie keine gewöhnliche kontinuierliche Dichte darstellt.

QQ- und CDF-Plots bleiben als eigenständige Diagnostikansichten erhalten.

**Ziel**

Verbesserung der Interpretierbarkeit und Reduktion visueller Komplexität, ohne den Informationsumfang der Anwendung einzuschränken.

---

## 3. Entwicklungsschwerpunkt

Die nächsten Entwicklungsschritte konzentrieren sich bewusst weniger auf die Erweiterung der Anzahl implementierter Verteilungen und stärker auf die Bewertung der statistischen Qualität der Anpassung.

Die Priorität liegt auf:

1. **Explorativer Diagnostik:** KDE, Modenzählung und Cullen-Frey-Diagramm.
2. **Modellunsicherheit:** Akaike Weights und weiterführende Bootstrap-Auswertungen.
3. **Modellvalidität:** Diagnostik von Multimodalität sowie Behandlung zensierter und trunkierter Daten.
4. **Anwendungsorientierte Auswertung:** Quantil-, Tail- und Risikomaße.
5. **Benutzerführung:** Reduktion visueller Komplexität und verständlichere Ergebnisdarstellung.

Die Erweiterungen sollen dabei nicht isoliert als zusätzliche Features verstanden werden. Sie verfolgen ein gemeinsames Ziel: `godist` von einem Werkzeug zur automatisierten Verteilungsanpassung zu einem Diagnostiktool weiterzuentwickeln, das die Plausibilität, Stabilität und Aussagekraft parametrischer Modelle systematisch bewertet.