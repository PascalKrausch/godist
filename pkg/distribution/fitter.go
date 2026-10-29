package distribution

import (
	"errors"
	"math"
	"sync"
)

// EvaluateFit berechnet alle Gütemetriken für eine beliebige ContinuousDistribution.
func EvaluateFit(dist ContinuousDistribution, data []float64) (FitMetrics, error) {
	cleaned := CleanDataWithZero(data)
	n := len(cleaned)
	if n == 0 {
		return FitMetrics{Converged: false}, ErrEmptyData
	}

	k := float64(freeParamCount(dist))
	nFloat := float64(n)

	// Sortierte Kopie für empirische CDF (KS-Test / RMSE) erstellen
	sortedData := SortedCopy(cleaned)

	// 1. Log-Likelihood berechnen
	ll := 0.0
	for _, x := range sortedData {
		logPdf := dist.LogPDF(x)
		if !IsFinite(logPdf) {
			// Außerhalb des Trägers (-Inf), singuläre Dichte (+Inf) oder numerischer Fehler (NaN)
			return FitMetrics{LogLikelihood: math.Inf(-1), Converged: false}, ErrFitFailed
		}
		ll += logPdf
	}

	// 2. Informationskriterien berechnen (AIC & BIC)
	aic := 2.0*k - 2.0*ll
	bic := k*math.Log(nFloat) - 2.0*ll

	// 3. KS-Statistik (D) & RMSE berechnen
	ksStat := 0.0
	sumSquaredErrors := 0.0

	for i, x := range sortedData {
		modelCDF := dist.CDF(x)

		// Empirische CDF (ECDF) Stufen: lower = (i)/N, upper = (i+1)/N
		ecdfLower := float64(i) / nFloat
		ecdfUpper := float64(i+1) / nFloat

		// Maximum Distance für Kolmogorov-Smirnov Test
		dLower := math.Abs(modelCDF - ecdfLower)
		dUpper := math.Abs(modelCDF - ecdfUpper)

		if dLower > ksStat {
			ksStat = dLower
		}
		if dUpper > ksStat {
			ksStat = dUpper
		}

		// RMSE-Berechnung gegen die kontinuierliche Plotting-Position (Hazens Formel: (i - 0.5) / N)
		// Dies ist symmetrischer und robuster als der reine obere Sprungpunkt
		ecdfMid := (float64(i) + 0.5) / nFloat
		err := modelCDF - ecdfMid
		sumSquaredErrors += err * err
	}

	rmse := math.Sqrt(sumSquaredErrors / nFloat)

	return FitMetrics{
		SampleSize:    n,
		Discarded:     len(data) - n,
		LogLikelihood: ll,
		AIC:           aic,
		BIC:           bic,
		KSStatistic:   ksStat,
		RMSE:          rmse,
		Converged:     true,
	}, nil
}

// freeParamCounter liefert die Anzahl der tatsächlich geschätzten Parameter (k für AIC/BIC),
// falls diese von Params() abweicht.
type freeParamCounter interface {
	freeParamCount() int
}

// freeParamCount nutzt freeParamCounter, falls implementiert, sonst len(Params()).
func freeParamCount(dist ContinuousDistribution) int {
	if c, ok := dist.(freeParamCounter); ok {
		return c.freeParamCount()
	}
	return len(dist.Params())
}

// ZeroPolicy legt fest, welche Kandidaten bei Nullwerten gewertet werden.
type ZeroPolicy int

const (
	// ZeroFastFail: Verteilungen mit Träger (0, ∞) scheitern bei Nullwerten.
	PolicyFastFail ZeroPolicy = iota
	// PolicyZeroInflated: Nur Zero-Inflated-Varianten; Nullen werden als Punktmasse modelliert.
	// Stetige Kandidaten werden nicht gewertet, da sie auf einem anderen Maß liegen.
	PolicyZeroInflated
)

func SelectBestModel(data []float64) (ContinuousDistribution, FitMetrics, error) {
	return SelectBestModelWithPolicy(data, PolicyFastFail)
}

func SelectBestModelWithPolicy(data []float64, policy ZeroPolicy) (ContinuousDistribution, FitMetrics, error) {
	results, _, err := SelectAllModelsWithPolicy(data, policy)
	if err != nil {
		return nil, FitMetrics{}, err
	}

	if len(results) == 0 {
		return nil, FitMetrics{}, errors.New("kein geeignetes Modell gefunden")
	}

	bestIdx := 0
	bestAIC := results[0].Metrics.AIC
	for i, result := range results {
		if result.Metrics.AIC < bestAIC {
			bestAIC = result.Metrics.AIC
			bestIdx = i
		}
	}

	return results[bestIdx].Dist, results[bestIdx].Metrics, nil
}

func SelectAllModels(data []float64) ([]FitResult, []FitFailure, error) {
	return SelectAllModelsWithPolicy(data, PolicyFastFail)
}

// candidateFitters liefert die Kandidaten für die gewählte Nullwert-Policy.
func candidateFitters(policy ZeroPolicy) []Fitter {
	if policy == PolicyZeroInflated {
		positive := []Fitter{ExponentialFitter{}, GammaFitter{}, WeibullFitter{}, LogNormalFitter{}, RayleighFitter{}}
		fitters := make([]Fitter, len(positive))
		for i, base := range positive {
			fitters[i] = ZeroInflatedFitter{Base: base}
		}
		return fitters
	}
	return []Fitter{
		WeibullFitter{},
		LogNormalFitter{},
		GammaFitter{},
		NormalFitter{},
		BetaFitter{},
		ExponentialFitter{},
		GumbelFitter{},
		RayleighFitter{},
		LaplaceFitter{},
	}
}

func SelectAllModelsWithPolicy(data []float64, policy ZeroPolicy) ([]FitResult, []FitFailure, error) {
	fitters := candidateFitters(policy)

	// Jede Verteilung läuft in einer eigenen Goroutine; Slots pro Index halten die Reihenfolge von fitters bei.
	outcomes := make([]fitOutcome, len(fitters))
	var wg sync.WaitGroup
	for i, fitter := range fitters {
		wg.Add(1)
		go func(i int, fitter Fitter) {
			defer wg.Done()
			outcomes[i] = fitOne(fitter, data)
		}(i, fitter)
	}
	wg.Wait()

	var results []FitResult
	var failures []FitFailure
	for _, o := range outcomes {
		if o.failure != nil {
			failures = append(failures, *o.failure)
		} else {
			results = append(results, o.result)
		}
	}

	return results, failures, nil
}

// fitOutcome ist das Ergebnis eines einzelnen Fitters: entweder Result oder Failure.
type fitOutcome struct {
	result  FitResult
	failure *FitFailure
}

// fitOne fittet eine Verteilung und bewertet sie mit EvaluateFit.
func fitOne(fitter Fitter, data []float64) fitOutcome {
	dist, err := fitter.Fit(data)
	if err != nil {
		return fitOutcome{failure: &FitFailure{Name: fitter.Name(), Error: err.Error()}}
	}

	metrics, err := EvaluateFit(dist, data)
	if err != nil {
		return fitOutcome{failure: &FitFailure{Name: fitter.Name(), Error: err.Error()}}
	}

	return fitOutcome{result: FitResult{Name: fitter.Name(), Dist: dist, Metrics: metrics}}
}
