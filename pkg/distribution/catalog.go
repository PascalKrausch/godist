package distribution

import "strings"

// Spec beschreibt eine Verteilung des Katalogs für Auflistung, Auswahl und Dokumentation.
type Spec struct {
	Name        string   // eindeutiger Name, z. B. "Weibull"
	Description string   // kurze Beschreibung
	Params      []string // Parameternamen in derselben Reihenfolge wie ContinuousDistribution.ParamNames()
	Support     string   // zulässiger Wertebereich der Daten
	FitNotes    string   // besondere Anforderungen an den Fit; leer, wenn keine
	// ZeroInflatable markiert Verteilungen, die als ZI-Variante mit Nullwerten verglichen werden können.
	ZeroInflatable bool
	Fitter         Fitter
}

// Catalog liefert alle Standardverteilungen in der Reihenfolge der Kandidatenauswahl.
// Jeder Aufruf liefert eine neue Liste.
func Catalog() []Spec {
	return []Spec{
		{
			Name:           "Weibull",
			Description:    "Zwei-Parameter-Weibull, häufig für Lebensdauern und Windgeschwindigkeiten",
			Params:         []string{"shape", "scale"},
			Support:        "[0, ∞)",
			ZeroInflatable: true,
			Fitter:         WeibullFitter{},
		},
		{
			Name:           "LogNormal",
			Description:    "Logarithmische Normalverteilung für positive, rechtsschiefe Daten",
			Params:         []string{"mu", "sigma"},
			Support:        "(0, ∞)",
			ZeroInflatable: true,
			Fitter:         LogNormalFitter{},
		},
		{
			Name:           "Gamma",
			Description:    "Gammaverteilung für positive, rechtsschiefe Daten",
			Params:         []string{"alpha", "scale"},
			Support:        "[0, ∞)",
			ZeroInflatable: true,
			Fitter:         GammaFitter{},
		},
		{
			Name:        "Normal",
			Description: "Normalverteilung, symmetrisch um mu",
			Params:      []string{"mu", "sigma"},
			Support:     "ℝ",
			Fitter:      NormalFitter{},
		},
		{
			Name:        "SkewNormal",
			Description: "Schiefe Normalverteilung (Azzalini) mit Schiefeparameter alpha; alpha = 0 ergibt die Normalverteilung",
			Params:      []string{"xi", "omega", "alpha"},
			Support:     "ℝ",
			FitNotes:    "3-Parameter-MLE per L-BFGS; für |alpha| → ∞ nähert sich die Verteilung der halbseitigen Normalverteilung",
			Fitter:      SkewNormalFitter{},
		},
		{
			Name:        "Beta",
			Description: "Beta-Verteilung auf einem endlichen Intervall",
			Params:      []string{"alpha", "beta", "min", "max"},
			Support:     "[min, max] (aus den Daten)",
			FitNotes:    "Grenzen min und max werden aus den Daten bestimmt; Momentenmethode",
			Fitter:      BetaFitter{},
		},
		{
			Name:           "Exponential",
			Description:    "Exponentialverteilung, gedächtnislos, ein Parameter",
			Params:         []string{"rate"},
			Support:        "[0, ∞)",
			ZeroInflatable: true,
			Fitter:         ExponentialFitter{},
		},
		{
			Name:        "Gumbel",
			Description: "Gumbel-Verteilung für Maxima, rechtsschief",
			Params:      []string{"mu", "beta"},
			Support:     "ℝ",
			Fitter:      GumbelFitter{},
		},
		{
			Name:        "GEV",
			Description: "Verallgemeinerte Extremwertverteilung (GEV) mit Formparameter xi; xi = 0 ergibt die Gumbel-Verteilung",
			Params:      []string{"mu", "sigma", "xi"},
			Support:     "ξ-abhängig: ℝ für ξ = 0, für ξ > 0 nach unten und für ξ < 0 nach oben beschränkt",
			FitNotes:    "für Extremwertverteilungen (z. B. Blockmaxima) gedacht; ein gutes AIC-Ranking ersetzt keine fachliche Prüfung der Datenherkunft und Modellannahmen; 3-Parameter-MLE per L-BFGS, ξ wird auf [−0.99, 5] begrenzt",
			Fitter:      GEVFitter{},
		},
		{
			Name:           "Rayleigh",
			Description:    "Rayleigh-Verteilung, Betrag eines zweidimensionalen Vektors",
			Params:         []string{"scale"},
			Support:        "[0, ∞)",
			ZeroInflatable: true,
			Fitter:         RayleighFitter{},
		},
		{
			Name:        "Laplace",
			Description: "Laplace-Verteilung mit schwereren Tails als die Normalverteilung",
			Params:      []string{"mu", "scale"},
			Support:     "ℝ",
			Fitter:      LaplaceFitter{},
		},
		{
			Name:        "Logistic",
			Description: "Logistische Verteilung, symmetrisch mit etwas schwereren Tails als die Normalverteilung",
			Params:      []string{"mu", "scale"},
			Support:     "ℝ",
			Fitter:      LogisticFitter{},
		},
		{
			Name:           "LogLogistic",
			Description:    "Log-logistische Verteilung für positive Daten mit schwerem rechten Tail",
			Params:         []string{"scale", "shape"},
			Support:        "(0, ∞)",
			ZeroInflatable: true,
			Fitter:         LogLogisticFitter{},
		},
		{
			Name:           "BurrXII",
			Description:    "Burr-Typ-XII-Verteilung mit zwei Formparametern, verallgemeinert die Log-Logistic",
			Params:         []string{"scale", "shape_c", "shape_k"},
			Support:        "(0, ∞)",
			FitNotes:       "scheitert bei Nullwerten (ln(0) undefiniert); Startwerte aus der Log-Logistic-MLE; Fits mit c oder k über 1e6 (Grenzfall, z. B. exponentialähnliche Daten) werden abgelehnt",
			ZeroInflatable: true,
			Fitter:         BurrXIIFitter{},
		},
		{
			Name:           "Pareto",
			Description:    "Pareto-Verteilung mit schwerem rechten Tail",
			Params:         []string{"shape", "scale"},
			Support:        "[scale, ∞)",
			FitNotes:       "scale (xm) wird als Minimum der Daten geschätzt",
			ZeroInflatable: true,
			Fitter:         ParetoFitter{},
		},
		{
			Name:           "ChiSquare",
			Description:    "Chi-Quadrat-Verteilung mit k Freiheitsgraden",
			Params:         []string{"df"},
			Support:        "[0, ∞)",
			ZeroInflatable: true,
			Fitter:         ChiSquareFitter{},
		},
		{
			Name:        "Cauchy",
			Description: "Cauchy-Verteilung mit sehr schweren Tails (Mittelwert nicht definiert)",
			Params:      []string{"mu", "scale"},
			Support:     "ℝ",
			Fitter:      CauchyFitter{},
		},
		{
			Name:        "StudentT",
			Description: "Student-t-Verteilung mit einstellbarer Tail-Schwere über nu",
			Params:      []string{"mu", "scale", "nu"},
			Support:     "ℝ",
			FitNotes:    "nu wird per Profil-Likelihood auf [0.1, 10⁴] begrenzt",
			Fitter:      StudentTFitter{},
		},
		{
			Name:           "InverseGaussian",
			Description:    "Inverse-Gauß-Verteilung (Wald-Verteilung) für positive, rechtsschiefe Daten",
			Params:         []string{"mu", "lambda"},
			Support:        "(0, ∞)",
			FitNotes:       "scheitert bei Nullwerten; geschlossene MLE: mu = Mittelwert, lambda = 1 / (mean(1/x) − 1/mean(x))",
			ZeroInflatable: true,
			Fitter:         InverseGaussianFitter{},
		},
		{
			Name:           "GeneralizedGamma",
			Description:    "Verallgemeinerte Gamma-Verteilung (Stacy) mit Form-, Skalen- und Potenzparameter; enthält Gamma (power = 1) und Weibull (shape = 1)",
			Params:         []string{"shape", "scale", "power"},
			Support:        "(0, ∞)",
			FitNotes:       "3-Parameter-MLE per L-BFGS auf der in der Skala profilierten Likelihood; power > 0 und power < 0 werden beide optimiert; scheitert bei Nullwerten",
			ZeroInflatable: true,
			Fitter:         GeneralizedGammaFitter{},
		},
	}
}

// Lookup sucht eine Verteilung über ihren Namen ohne Beachtung der Groß- und Kleinschreibung.
func Lookup(name string) (Spec, bool) {
	for _, spec := range Catalog() {
		if strings.EqualFold(spec.Name, name) {
			return spec, true
		}
	}
	return Spec{}, false
}
