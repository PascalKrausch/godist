package distribution

import (
	"errors"

	"math"
)

// EvaluateFit berechnet alle Gütemetriken für eine beliebige ContinuousDistribution.
func EvaluateFit(dist ContinuousDistribution, data []float64) (FitMetrics, error) {
	cleaned := CleanData(data)
	n := len(cleaned)
	if n == 0 {
		return FitMetrics{}, ErrEmptyData
	}

	k := float64(len(dist.Params()))
	nFloat := float64(n)

	// Sortierte Kopie für empirische CDF (KS-Test / RMSE) erstellen
	sortedData := SortedCopy(cleaned)

	// 1. Log-Likelihood berechnen
	ll := 0.0
	for _, x := range sortedData {
		logPdf := dist.LogPDF(x)
		if math.IsInf(logPdf, -1) || math.IsNaN(logPdf) {
			// Falls Datenpunkt außerhalb des Trägers liegt oder numerischer Fehler auftritt
			return FitMetrics{LogLikelihood: math.Inf(-1)}, ErrFitFailed
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
		LogLikelihood: ll,
		AIC:           aic,
		BIC:           bic,
		KSStatistic:   ksStat,
		RMSE:          rmse,
	}, nil
}

func SelectBestModel(data []float64) (ContinuousDistribution, FitMetrics, error) {
	results, _, err := SelectAllModels(data)
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
	fitters := []Fitter{
		WeibullFitter{},
		LogNormalFitter{},
		GammaFitter{},
		NormalFitter{},
		BetaFitter{},
		ExponentialFitter{},
		GumbelFitter{},
	}

	var results []FitResult
	var failures []FitFailure

	for _, fitter := range fitters {
		dist, err := fitter.Fit(data)
		if err != nil {
			failures = append(failures, FitFailure{
				Name:  fitter.Name(),
				Error: err.Error(),
			})
			continue
		}

		metrics, err := EvaluateFit(dist, data)
		if err != nil {
			failures = append(failures, FitFailure{
				Name:  fitter.Name(),
				Error: err.Error(),
			})
			continue
		}

		results = append(results, FitResult{
			Dist:    dist,
			Metrics: metrics,
		})
	}

	return results, failures, nil
}
