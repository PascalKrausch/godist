package distribution

import (
	"errors"
	"fmt"
	"math"

	"gonum.org/v1/gonum/mathext"
	"gonum.org/v1/gonum/optimize"
	"gonum.org/v1/gonum/stat"
)

// Beta repräsentiert eine Beta-Verteilung im Intervall [Min, Max].
// Für die Standard-Beta-Verteilung gilt Min = 0 und Max = 1.
type Beta struct {
	Alpha float64 // Formparameter (α > 0)
	Beta  float64 // Formparameter (β > 0)
	Min   float64 // Untere Intervallgrenze
	Max   float64 // Obere Intervallgrenze
}

func (b Beta) Params() []float64 {
	return []float64{b.Alpha, b.Beta, b.Min, b.Max}
}

func (b Beta) ParamNames() []string {
	return []string{"alpha", "beta", "min", "max"}
}

// freeParamCount: Min und Max sind fest (vorgegeben oder aus den Daten bestimmt) und werden nicht geschätzt.
func (b Beta) freeParamCount() int {
	return 2
}

func (b Beta) Valid() bool {
	return b.Alpha > 0 && b.Beta > 0 && b.Max > b.Min && IsFinite(b.Alpha) && IsFinite(b.Beta)
}

// Transformiert x aus [Min, Max] nach [0, 1]
func (b Beta) normalize(x float64) float64 {
	return (x - b.Min) / (b.Max - b.Min)
}

func (b Beta) PDF(x float64) float64 {
	if !b.Valid() || x < b.Min || x > b.Max {
		return 0
	}
	return math.Exp(b.LogPDF(x))
}

func (b Beta) LogPDF(x float64) float64 {
	if !b.Valid() || x < b.Min || x > b.Max {
		return math.Inf(-1)
	}

	y := b.normalize(x)
	if y == 0 || y == 1 {
		return math.Inf(-1)
	}

	lgammaA, _ := math.Lgamma(b.Alpha)
	lgammaB, _ := math.Lgamma(b.Beta)
	lgammaAB, _ := math.Lgamma(b.Alpha + b.Beta)

	// LogPDF mit Skalierungsfaktor 1 / (Max - Min)
	logBetaFunc := lgammaA + lgammaB - lgammaAB
	return (b.Alpha-1)*math.Log(y) + (b.Beta-1)*math.Log(1-y) - logBetaFunc - math.Log(b.Max-b.Min)
}

func (b Beta) CDF(x float64) float64 {
	if !b.Valid() || x <= b.Min {
		return 0
	}
	if x >= b.Max {
		return 1
	}
	y := b.normalize(x)
	// Regulierte unvollständige Beta-Funktion I_y(α, β) aus Gonum
	return mathext.RegIncBeta(b.Alpha, b.Beta, y)
}

// BetaFitter schätzt Alpha und Beta via Momentenmethode.
type BetaFitter struct {
	// Wenn Min == Max == 0, werden Min/Max automatisch aus den Daten ermittelt.
	Min float64
	Max float64
}

func (f BetaFitter) Name() string {
	return "Beta"
}

func (f BetaFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	if len(cleaned) < 3 {
		return nil, errors.New("zu wenige Datenpunkte für Beta-Fit")
	}

	minVal := f.Min
	maxVal := f.Max

	// Grenzen automatisch bestimmen, falls nicht vorgegeben
	if minVal == 0 && maxVal == 0 {
		// Berechne Min/Max direkt ohne Summarize (spart 1 Allokation)
		dataMin := cleaned[0]
		dataMax := cleaned[0]
		for _, v := range cleaned {
			if v < dataMin {
				dataMin = v
			}
			if v > dataMax {
				dataMax = v
			}
		}
		margin := 0.01 * (dataMax - dataMin)
		if margin == 0 {
			margin = 0.001
		}
		minVal = dataMin - margin
		maxVal = dataMax + margin
	}

	// Daten auf [0, 1] skalieren und Log-Summen für MLE berechnen
	rangeVal := maxVal - minVal
	var sumLogY, sumLog1mY float64
	for _, x := range cleaned {
		if x <= minVal || x >= maxVal {
			return nil, errors.New("datenpunkte liegen außerhalb der angegebenen Min/Max-Grenzen")
		}
		y := (x - minVal) / rangeVal
		sumLogY += math.Log(y)
		sumLog1mY += math.Log1p(-y)
	}

	// Momentenmethode im skalierten Raum [0, 1]
	mean := stat.Mean(cleaned, nil)
	variance := stat.Variance(cleaned, nil)

	// Transformiere mean und variance von [minVal, maxVal] nach [0, 1]
	scaledMean := (mean - minVal) / rangeVal
	scaledVariance := variance / (rangeVal * rangeVal)

	if scaledVariance >= scaledMean*(1.0-scaledMean) || scaledVariance <= 0 {
		return nil, errors.New("datenvarianz ist zu hoch/ungültig für Beta-Verteilung")
	}

	// Schätzung für α und β
	commonFactor := (scaledMean * (1.0 - scaledMean) / scaledVariance) - 1.0
	alpha := scaledMean * commonFactor
	betaParam := (1.0 - scaledMean) * commonFactor

	if alpha <= 0 || betaParam <= 0 || !IsFinite(alpha) || !IsFinite(betaParam) {
		return nil, errors.New("beta-parameter konnte nicht geschätzt werden")
	}

	alphaMLE, betaMLE, err := fitBetaMLE(
		sumLogY,
		sumLog1mY,
		alpha,
		betaParam,
		len(cleaned),
	)
	if err != nil {
		return nil, err
	}

	return Beta{
		Alpha: alphaMLE,
		Beta:  betaMLE,
		Min:   minVal,
		Max:   maxVal,
	}, nil
}

func fitBetaMLE(
	sumLogY, sumLog1mY float64,
	alpha0, beta0 float64,
	dataLen int,
) (float64, float64, error) {
	if dataLen < 2 {
		return 0, 0, errors.New(
			"zu wenige Datenpunkte für Beta-MLE",
		)
	}

	if alpha0 <= 0 || beta0 <= 0 ||
		math.IsNaN(alpha0) || math.IsNaN(beta0) ||
		math.IsInf(alpha0, 0) || math.IsInf(beta0, 0) {
		return 0, 0, errors.New(
			"ungültige Startwerte für Beta-MLE",
		)
	}

	n := float64(dataLen)

	isFinite := func(v float64) bool {
		return !math.IsNaN(v) && !math.IsInf(v, 0)
	}

	// z[0] = log(alpha), z[1] = log(beta).
	// Optimiert wird die negative Log-Likelihood.
	//
	// Die Gradienten sind bereits nach der Kettenregel
	// auf die logarithmierten Parameter umgerechnet.
	bestZ := []float64{math.NaN(), math.NaN()}
	bestF := math.Inf(1)

	problem := optimize.Problem{
		Func: func(z []float64) float64 {
			invalid := math.Inf(1)

			if len(z) != 2 {
				return invalid
			}

			alpha := math.Exp(z[0])
			betaParam := math.Exp(z[1])

			if !isFinite(alpha) || !isFinite(betaParam) ||
				alpha <= 0 || betaParam <= 0 {
				return invalid
			}

			sum := alpha + betaParam
			if !isFinite(sum) {
				return invalid
			}

			// Negative Log-Likelihood:
			//
			// n*log(B(alpha,beta))
			// - (alpha-1)*sum(log(y))
			// - (beta-1)*sum(log(1-y))
			f := n*mathext.Lbeta(alpha, betaParam) -
				(alpha-1)*sumLogY -
				(betaParam-1)*sumLog1mY

			if !isFinite(f) {
				return invalid
			}

			if f < bestF {
				bestF = f
				bestZ[0] = z[0]
				bestZ[1] = z[1]
			}

			return f
		},
		Grad: func(grad, z []float64) {
			if len(z) != 2 {
				grad[0] = math.Inf(1)
				grad[1] = math.Inf(1)
				return
			}

			alpha := math.Exp(z[0])
			betaParam := math.Exp(z[1])

			if !isFinite(alpha) || !isFinite(betaParam) ||
				alpha <= 0 || betaParam <= 0 {
				grad[0] = math.Inf(1)
				grad[1] = math.Inf(1)
				return
			}

			sum := alpha + betaParam
			if !isFinite(sum) {
				grad[0] = math.Inf(1)
				grad[1] = math.Inf(1)
				return
			}

			// Ableitungen bezüglich alpha und beta,
			// anschließend Kettenregel für log(alpha), log(beta).
			gradAlpha := alpha * (n*(mathext.Digamma(alpha)-mathext.Digamma(sum)) - sumLogY)
			gradBeta := betaParam * (n*(mathext.Digamma(betaParam)-mathext.Digamma(sum)) - sumLog1mY)

			if !isFinite(gradAlpha) || !isFinite(gradBeta) {
				grad[0] = math.Inf(1)
				grad[1] = math.Inf(1)
				return
			}

			grad[0] = gradAlpha
			grad[1] = gradBeta
		},
	}

	settings := &optimize.Settings{
		GradientThreshold: 1e-8,
		MajorIterations:   1000,
		FuncEvaluations:   10000,
		GradEvaluations:   10000,
	}

	initial := [2]float64{
		math.Log(alpha0),
		math.Log(beta0),
	}

	result, err := optimize.Minimize(
		problem,
		initial[:],
		settings,
		&optimize.LBFGS{},
	)

	var x []float64
	if err != nil {
		// ErrNoProgress tritt nahe am Optimum auf, wenn die Schrittweite
		// unter die Rundungsgenauigkeit fällt. Der beste gesehene Punkt
		// wird nur akzeptiert, wenn der Gradient pro Beobachtung klein ist.
		if (!errors.Is(err, optimize.ErrNoProgress) &&
			!errors.Is(err, optimize.ErrLinesearcherFailure)) ||
			math.IsNaN(bestZ[0]) {
			return 0, 0, fmt.Errorf(
				"Beta-MLE-Optimierung fehlgeschlagen: %w",
				err,
			)
		}

		var grad [2]float64
		problem.Grad(grad[:], bestZ)
		maxGrad := math.Max(math.Abs(grad[0]), math.Abs(grad[1]))
		if !isFinite(maxGrad) || maxGrad/n > 1e-6 {
			return 0, 0, fmt.Errorf(
				"Beta-MLE-Optimierung fehlgeschlagen: %w",
				err,
			)
		}

		x = bestZ
	} else {
		if result.Status.Early() {
			return 0, 0, fmt.Errorf(
				"Beta-MLE nicht konvergiert: %s",
				result.Status,
			)
		}
		x = result.X
	}

	alpha := math.Exp(x[0])
	betaParam := math.Exp(x[1])

	if !isFinite(alpha) || !isFinite(betaParam) ||
		alpha <= 0 || betaParam <= 0 {
		return 0, 0, errors.New(
			"Beta-MLE lieferte ungültige Parameter",
		)
	}

	return alpha, betaParam, nil
}
