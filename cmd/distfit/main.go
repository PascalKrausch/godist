package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/PascalKrausch/godist/internal/cli"
	"github.com/PascalKrausch/godist/internal/plotserver"
	"github.com/PascalKrausch/godist/internal/stats"
	"github.com/PascalKrausch/godist/pkg/distribution"
)

const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run führt die CLI aus und liefert den Exit-Code.
// Ergebnisse gehen nach stdout, Warnungen und Fehler nach stderr.
// Ohne Unterbefehl gilt der Flag-Aufruf (-file=…) unverändert.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "fit":
			return runFitCommand("distfit fit", args[1:], true, stdout, stderr)
		case "list":
			return runListCommand(args[1:], stdout, stderr)
		}
	}
	return runFitCommand("distfit", args, false, stdout, stderr)
}

// runFitCommand parst die Fit-Optionen und führt den Fit aus. Mit allowPositional
// darf die Datei alternativ als einziges Positionsargument angegeben werden.
func runFitCommand(name string, args []string, allowPositional bool, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { writeUsage(stderr, fs) }

	fileFlag := fs.String("file", "", "Pfad zur Datei (.csv, .json, .txt)")
	column := fs.String("column", "", "CSV: Spaltenname oder 0-basierter Index (ohne Angabe: einzige numerische Spalte)")
	allModels := fs.Bool("all", false, "Alle Modelle vergleichen statt nur das beste anzuzeigen")
	format := fs.String("format", "text", "Ausgabeformat: text, json oder plot (json und plot erfordern -all)")
	decimal := fs.String("decimal", ".", "Dezimaltrennzeichen: . oder ,")
	zeroInflated := fs.Bool("zero-inflated", false, "Nullwerte als Zero-Inflated-Modell (Punktmasse bei 0) modellieren; nur ZI-Kandidaten werden verglichen (Standard: Nullwerte lassen Kandidaten mit Träger (0, ∞) scheitern)")
	distName := fs.String("distribution", "", "Genau eine Verteilung fitten, ohne Vergleich (z. B. weibull)")
	include := fs.String("include", "", "Nur diese Verteilungen verwenden, kommasepariert (z. B. weibull,gamma)")
	exclude := fs.String("exclude", "", "Diese Verteilungen auslassen, kommasepariert (z. B. cauchy,laplace)")
	bootstrap := fs.Int("bootstrap", 0, "Bootstrap-Wiederholungen für Konfidenzintervalle von Parametern und Metriken (0 = aus)")
	bootstrapMethod := fs.String("bootstrap-method", "nonparametric", "Bootstrap-Methode: nonparametric (Datenresampling) oder parametric (Sampling aus dem gefitteten Modell)")
	bootstrapCI := fs.Float64("bootstrap-ci", 0.95, "Konfidenzniveau der Bootstrap-Intervalle in (0, 1)")
	bootstrapSeed := fs.Uint64("bootstrap-seed", 0, "Seed für reproduzierbare Bootstrap-Läufe (0 = zufällig, wird auf stderr ausgegeben)")
	bootstrapModel := fs.String("bootstrap-model", "best", "Für welche Modelle Bootstrap läuft: best (niedrigster AIC) oder all")
	dip := fs.Bool("dip", false, "Hartigan-Dip-Test auf die Stichprobe (Unimodalität; p-Wert per Monte-Carlo)")
	dipSimulations := fs.Int("dip-simulations", 2000, "Monte-Carlo-Simulationen für -dip (2000 bis 10000 empfohlen)")
	dipSeed := fs.Uint64("dip-seed", 1, "Seed für reproduzierbare -dip-Läufe")

	positional, err := parseArgs(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}

	filePath, err := resolveFilePath(*fileFlag, positional, allowPositional)
	if err == nil {
		err = validateFlags(filePath, *column, *format, *decimal, *allModels)
	}
	bootMethod, bootErr := distribution.ParseBootstrapMethod(*bootstrapMethod)
	if err == nil {
		err = bootErr
	}
	if err == nil {
		err = validateBootstrapFlags(fs, *bootstrap, bootMethod, *bootstrapCI, *bootstrapModel)
	}
	if err == nil {
		err = validateDipFlags(fs, *dip, *dipSimulations, *format)
	}
	if err != nil {
		fmt.Fprintf(stderr, "Fehler: %v\n\n", err)
		fs.Usage()
		return exitUsage
	}

	ds, err := cli.ReadDataset(filePath, cli.ReadOptions{Column: *column, Decimal: *decimal})
	if err != nil {
		fmt.Fprintf(stderr, "Fehler beim Einlesen von %s: %v\n", filePath, err)
		return exitFailure
	}
	ds.WriteWarnings(stderr)

	warning, err := cli.ValidateSample(ds.Values)
	if err != nil {
		fmt.Fprintf(stderr, "Fehler: %v\n", err)
		return exitFailure
	}
	if warning != "" {
		fmt.Fprintf(stderr, "Warnung: %s\n", warning)
	}
	policy := distribution.PolicyFastFail
	if *zeroInflated {
		policy = distribution.PolicyZeroInflated
	}
	fitters, err := selectFitters(policy, *distName, *include, *exclude)
	if err != nil {
		fmt.Fprintf(stderr, "Fehler: %v\n\n", err)
		fs.Usage()
		return exitUsage
	}
	bootCfg := distribution.BootstrapConfig{
		B:      *bootstrap,
		Method: bootMethod,
		Level:  *bootstrapCI,
		Seed:   *bootstrapSeed,
	}
	if bootCfg.B > 0 {
		if bootCfg.Seed == 0 {
			bootCfg.Seed = distribution.RandomBootstrapSeed()
		}
		fmt.Fprintf(stderr, "Bootstrap: %d Wiederholungen (%s, %.0f%%-KI, Seed %d, Modell: %s)\n",
			bootCfg.B, bootCfg.Method, bootCfg.Level*100, bootCfg.Seed, *bootstrapModel)
	}
	var dipReport *cli.DipReport
	if *dip {
		res := distribution.DipTest(ds.Values, *dipSimulations, *dipSeed)
		dipReport = &cli.DipReport{
			Dip:         res.Dip,
			PValue:      res.PValue,
			Simulations: *dipSimulations,
			Seed:        *dipSeed,
		}
	}
	if *allModels {
		return runAll(stdout, stderr, ds, filePath, *format, fitters, bootCfg, *bootstrapModel, dipReport)
	}
	return runBest(stdout, stderr, ds, fitters, bootCfg, dipReport)
}

// selectFitters übersetzt die Auswahl-Flags in Kandidaten. -distribution ist ein Sonderfall
// von -include mit genau einem Namen und schließt include und exclude aus.
func selectFitters(policy distribution.ZeroPolicy, distName, include, exclude string) ([]distribution.Fitter, error) {
	if distName != "" && (include != "" || exclude != "") {
		return nil, errors.New("-distribution schließt -include und -exclude aus")
	}
	if distName != "" {
		include = distName
	}
	return distribution.SelectFitters(policy, splitNames(include), splitNames(exclude))
}

func splitNames(list string) []string {
	if list == "" {
		return nil
	}
	return strings.Split(list, ",")
}

// runListCommand führt `distfit list distributions` aus.
func runListCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "distributions" {
		fmt.Fprintln(stderr, "Fehler: unbekannter oder fehlender Befehl, erwartet: distfit list distributions")
		return exitUsage
	}

	fs := flag.NewFlagSet("distfit list distributions", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Verwendung: distfit list distributions [-format=text|json]")
		fs.PrintDefaults()
	}
	format := fs.String("format", "text", "Ausgabeformat: text oder json")

	positional, err := parseArgs(fs, args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if len(positional) > 0 {
		fmt.Fprintf(stderr, "Fehler: unerwartete Argumente: %s\n\n", strings.Join(positional, " "))
		fs.Usage()
		return exitUsage
	}

	specs := distribution.Catalog()
	switch *format {
	case "text":
		err = cli.WriteDistributionsText(stdout, specs)
	case "json":
		err = cli.WriteDistributionsJSON(stdout, specs)
	default:
		fmt.Fprintf(stderr, "Fehler: ungültiges -format %q (erlaubt: text, json)\n\n", *format)
		fs.Usage()
		return exitUsage
	}
	if err != nil {
		fmt.Fprintf(stderr, "Fehler: Ausgabe fehlgeschlagen: %v\n", err)
		return exitFailure
	}
	return exitOK
}

// parseArgs parst Flags und sammelt Positionsargumente, auch wenn sie vor den Flags stehen
// (`distfit fit daten.csv -all`). Nach einem `--` gilt alles Weitere als Positionsargument.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return positional, nil
		}
		if idx := len(args) - len(rest); idx > 0 && args[idx-1] == "--" {
			return append(positional, rest...), nil
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
}

// resolveFilePath bestimmt die Datei aus -file oder dem Positionsargument.
func resolveFilePath(flagValue string, positional []string, allowPositional bool) (string, error) {
	if !allowPositional {
		if len(positional) > 0 {
			return "", fmt.Errorf("unerwartete Argumente: %s", strings.Join(positional, " "))
		}
		return flagValue, nil
	}
	switch {
	case len(positional) == 0:
		return flagValue, nil
	case len(positional) > 1:
		return "", fmt.Errorf("unerwartete Argumente: %s", strings.Join(positional[1:], " "))
	case flagValue != "":
		return "", errors.New("Datei entweder als Argument oder mit -file angeben, nicht beides")
	}
	return positional[0], nil
}

// validateFlags prüft Flag-Kombinationen, bevor Daten gelesen werden.
func validateFlags(filePath, column, format, decimal string, allModels bool) error {
	if filePath == "" {
		return errors.New("bitte eine Datei angeben: -file=daten.csv oder distfit fit daten.csv")
	}
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".csv", ".json", ".txt":
	default:
		return fmt.Errorf("nicht unterstütztes Dateiformat für %s (erlaubt: .csv, .json, .txt)", filePath)
	}
	if column != "" && strings.ToLower(filepath.Ext(filePath)) != ".csv" {
		return errors.New("-column wird nur für CSV-Dateien unterstützt")
	}
	switch format {
	case "text":
	case "json", "plot":
		if !allModels {
			return fmt.Errorf("-format=%s erfordert -all", format)
		}
	default:
		return fmt.Errorf("ungültiges -format %q (erlaubt: text, json, plot)", format)
	}
	if decimal != "." && decimal != "," {
		return fmt.Errorf("ungültiges -decimal %q (erlaubt: . oder ,)", decimal)
	}
	return nil
}

// validateBootstrapFlags prüft die Bootstrap-Optionen. Die bootstrap-Optionen ohne
// -bootstrap > 0 sind ein Fehler, damit ein versehentlich gesetzter Flag nicht ignoriert wird.
func validateBootstrapFlags(fs *flag.FlagSet, b int, method distribution.BootstrapMethod, level float64, scope string) error {
	if b < 0 {
		return errors.New("-bootstrap muss >= 0 sein")
	}
	if level <= 0 || level >= 1 {
		return fmt.Errorf("ungültiges -bootstrap-ci %v (erlaubt: Wert in (0, 1))", level)
	}
	if scope != "best" && scope != "all" {
		return fmt.Errorf("ungültiges -bootstrap-model %q (erlaubt: best, all)", scope)
	}
	if b == 0 {
		var stray []string
		fs.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "bootstrap-method", "bootstrap-ci", "bootstrap-seed", "bootstrap-model":
				stray = append(stray, "-"+f.Name)
			}
		})
		if len(stray) > 0 {
			return fmt.Errorf("%s erfordert -bootstrap > 0", strings.Join(stray, ", "))
		}
	}
	return nil
}

// validateDipFlags prüft die Dip-Optionen. Wie bei -bootstrap sind die
// Unteroptionen ohne -dip ein Aufruffehler; -dip ist mit -format=plot nicht
// kombinierbar, weil das Dashboard das Ergebnis nicht anzeigt.
func validateDipFlags(fs *flag.FlagSet, enabled bool, simulations int, format string) error {
	if enabled {
		if simulations < 1 {
			return fmt.Errorf("-dip-simulations muss >= 1 sein, erhalten: %d", simulations)
		}
		if format == "plot" {
			return errors.New("-dip ist mit -format=plot nicht kombinierbar (Text- oder JSON-Ausgabe verwenden)")
		}
		return nil
	}
	var stray []string
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "dip-simulations", "dip-seed":
			stray = append(stray, "-"+f.Name)
		}
	})
	if len(stray) > 0 {
		return fmt.Errorf("%s erfordert -dip", strings.Join(stray, ", "))
	}
	return nil
}

// applyBootstrap führt die Bootstrap-Wiederholungen für die gewählten Modelle aus
// und hängt das Ergebnis an FitResult.Bootstrap. scope "best" beschränkt auf das
// Modell mit dem kleinsten AIC, "all" auf alle erfolgreichen Fits.
func applyBootstrap(results []distribution.FitResult, fitters []distribution.Fitter, data []float64, cfg distribution.BootstrapConfig, scope string, stderr io.Writer) {
	if cfg.B <= 0 {
		return
	}
	byName := make(map[string]distribution.Fitter, len(fitters))
	for _, f := range fitters {
		byName[f.Name()] = f
	}

	idxs := make([]int, 0, len(results))
	if scope == "all" {
		for i := range results {
			idxs = append(idxs, i)
		}
	} else {
		best := 0
		for i := range results {
			if results[i].Metrics.AIC < results[best].Metrics.AIC {
				best = i
			}
		}
		idxs = []int{best}
	}

	for _, i := range idxs {
		fitter, ok := byName[results[i].Name]
		if !ok {
			fmt.Fprintf(stderr, "Warnung: kein Fitter für %s, Bootstrap übersprungen\n", results[i].Name)
			continue
		}
		res, err := distribution.BootstrapFit(fitter, results[i].Dist, data, cfg)
		if err != nil {
			fmt.Fprintf(stderr, "Warnung: Bootstrap für %s fehlgeschlagen: %v\n", results[i].Name, err)
			continue
		}
		results[i].Bootstrap = res
	}
}

// runAll vergleicht alle Verteilungen und gibt den Bericht aus.
func runAll(stdout, stderr io.Writer, ds cli.Dataset, filePath, format string, fitters []distribution.Fitter, bootCfg distribution.BootstrapConfig, bootScope string, dip *cli.DipReport) int {
	start := time.Now()
	results, failures, err := distribution.SelectAllModelsFromFitters(ds.Values, fitters)
	if err != nil {
		fmt.Fprintf(stderr, "Fehler: Fitting fehlgeschlagen: %v\n", err)
		return exitFailure
	}
	applyBootstrap(results, fitters, ds.Values, bootCfg, bootScope, stderr)
	totalDuration := time.Since(start)

	if format == "plot" {
		return runPlot(stderr, ds.Values, results, totalDuration)
	}

	report := cli.BuildFitReport(results, failures, filePath, ds.Column, stats.Describe(ds.Values), len(ds.Discarded))
	report.Dip = dip
	report.TotalDurationMs = float64(totalDuration) / float64(time.Millisecond)

	if format == "json" {
		if err := cli.WriteJSONReport(stdout, report); err != nil {
			fmt.Fprintf(stderr, "Fehler: JSON-Ausgabe fehlgeschlagen: %v\n", err)
			return exitFailure
		}
	} else if err := cli.WriteTextReport(stdout, report); err != nil {
		fmt.Fprintf(stderr, "Fehler: Report-Erstellung fehlgeschlagen: %v\n", err)
		return exitFailure
	}

	if len(results) == 0 {
		fmt.Fprintln(stderr, "Fehler: Kein Modell konnte angepasst werden.")
		return exitFailure
	}
	return exitOK
}

// runPlot startet das interaktive Dashboard, öffnet den Browser und blockiert bis Strg+C.
func runPlot(stderr io.Writer, values []float64, results []distribution.FitResult, fitDuration time.Duration) int {
	if len(results) == 0 {
		fmt.Fprintln(stderr, "Fehler: Kein Modell konnte angepasst werden.")
		return exitFailure
	}

	source := cli.BuildPlotSource(values, results)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := plotserver.Serve(ctx, source, func(url string) {
		fmt.Fprintf(stderr, "Dashboard: %s\nFit-Zeit gesamt: %s\nBeenden mit Strg+C.\n", url, fitDuration.Round(time.Millisecond))
		if err := cli.OpenBrowser(url); err != nil {
			fmt.Fprintf(stderr, "Warnung: Browser konnte nicht geöffnet werden: %v\n", err)
		}
	})
	if err != nil {
		fmt.Fprintf(stderr, "Fehler: Dashboard fehlgeschlagen: %v\n", err)
		return exitFailure
	}
	return exitOK
}

// runBest gibt nur das beste Modell aus.
func runBest(stdout, stderr io.Writer, ds cli.Dataset, fitters []distribution.Fitter, bootCfg distribution.BootstrapConfig, dip *cli.DipReport) int {
	results, _, err := distribution.SelectAllModelsFromFitters(ds.Values, fitters)
	if err != nil {
		fmt.Fprintf(stderr, "Fehler: Fitting fehlgeschlagen: %v\n", err)
		return exitFailure
	}
	if len(results) == 0 {
		fmt.Fprintln(stderr, "Fehler: kein geeignetes Modell gefunden")
		return exitFailure
	}
	best := 0
	for i := range results {
		if results[i].Metrics.AIC < results[best].Metrics.AIC {
			best = i
		}
	}
	applyBootstrap(results, fitters, ds.Values, bootCfg, "best", stderr)
	result := results[best]
	cli.WriteSummary(stdout, result.Dist, result.Metrics, ds.Column, stats.Describe(ds.Values), len(ds.Discarded), result.Bootstrap, dip)
	return exitOK
}

func writeUsage(w io.Writer, fs *flag.FlagSet) {
	fmt.Fprintln(w, "Verwendung:")
	fmt.Fprintln(w, "  distfit -file=<datei> [optionen]")
	fmt.Fprintln(w, "  distfit fit <datei> [optionen]")
	fmt.Fprintln(w, "  distfit list distributions [-format=text|json]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Passt Verteilungen an Daten an und vergleicht sie nach AIC.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Optionen:")
	fs.PrintDefaults()
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Beispiele:")
	fmt.Fprintln(w, "  distfit -file=werte.txt")
	fmt.Fprintln(w, "  distfit fit werte.txt -all")
	fmt.Fprintln(w, "  distfit fit werte.txt -distribution=weibull")
	fmt.Fprintln(w, "  distfit list distributions")
	fmt.Fprintln(w, "  distfit -file=daten.csv -column=density -all")
	fmt.Fprintln(w, "  distfit -file=daten.csv -column=2 -decimal=, -all -format=json")
	fmt.Fprintln(w, "  distfit -file=werte.txt -all -format=plot")
	fmt.Fprintln(w, "  distfit -file=daten.csv -column=citric -zero-inflated -all")
	fmt.Fprintln(w, "  distfit fit werte.txt -bootstrap=1000 -bootstrap-seed=42")
	fmt.Fprintln(w, "  distfit fit werte.txt -dip -dip-seed=42")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Exit-Codes: 0 Erfolg, 1 Daten- oder Fitting-Fehler, 2 Aufruf-Fehler")
}
