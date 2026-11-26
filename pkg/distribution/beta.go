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
	// boundsEstimated markiert, dass Min/Max per ML aus den Daten geschätzt
	// wurden (BetaFitter ohne vorgegebene Grenzen). Dann zählen sie als
	// freie Parameter für AIC/BIC.
	boundsEstimated bool
}

func (b Beta) Quantile(p float64) float64 {
	if !b.Valid() || !validProbability(p) {
		return math.NaN()
	}
	if p == 0 {
		return b.Min
	}
	if p == 1 {
		return b.Max
	}
	y := bisectCDF(func(y float64) float64 {
		return mathext.RegIncBeta(b.Alpha, b.Beta, y)
	}, p, 0, 1)
	return b.Min + y*(b.Max-b.Min)
}

func (b Beta) Params() []float64 {
	return []float64{b.Alpha, b.Beta, b.Min, b.Max}
}

func (b Beta) ParamNames() []string {
	return []string{"alpha", "beta", "min", "max"}
}

// freeParamCount: Min und Max zählen nur, wenn sie per ML aus den Daten
// geschätzt wurden (boundsEstimated); vorgegebene Grenzen sind fix.
func (b Beta) freeParamCount() int {
	if b.boundsEstimated {
		return 4
	}
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

	// Grenzen automatisch bestimmen, falls nicht vorgegeben: Min/Max werden
	// dann als freie ML-Parameter mit α, β ≥ 1 geschätzt (siehe
	// fitBetaAutoBounds). Eine feste Boundary-Dichte verhindert die
	// unbegrenzte Likelihood klassischer Stützpunktschätzung.
	if minVal == 0 && maxVal == 0 {
		return fitBetaAutoBounds(cleaned)
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

// betaAutoPoint hält die aus z abgeleiteten Modellparameter samt
// Hilfsgrößen für die ML-Optimierung der Beta mit geschätzten Grenzen.
type betaAutoPoint struct {
	alpha, betaP float64
	min, max     float64
	rangeVal     float64
	s1, s2       float64 // Σ ln y, Σ ln(1−y)
	t1, t3       float64 // Σ 1/y, Σ y/(1−y)
	nll          float64
	ok           bool
}

// evalBetaAuto wertet Negative-Log-Likelihood und partielle Ableitungen
// nach (alpha, beta, min, max) an der Stelle z aus.
//
// Parametrisierung (alle Parameter > 0 per Konstruktion):
//
//	alpha = 1 + exp(z0),  beta = 1 + exp(z1)   (Dichte am Rand endlich)
//	min   = dataMin − exp(z2), max = dataMax + exp(z3)  (Träger enthält Daten strikt)
//
// Mit α, β ≥ 1 geht die Dichte an den Rändern gegen 0 (bzw. ist endlich
// für α = β = 1), sodass die Likelihood im Randabstand nach oben
// beschränkt ist — die Divergenz der Stützpunktschätzung ist ausgeschlossen.
func evalBetaAuto(z []float64, data []float64, dataMin, dataMax float64) betaAutoPoint {
	var p betaAutoPoint
	if len(z) != 4 {
		return p
	}

	p.alpha = 1 + math.Exp(z[0])
	p.betaP = 1 + math.Exp(z[1])
	p.min = dataMin - math.Exp(z[2])
	p.max = dataMax + math.Exp(z[3])
	p.rangeVal = p.max - p.min

	if !IsFinite(p.alpha) || !IsFinite(p.betaP) ||
		!IsFinite(p.min) || !IsFinite(p.max) ||
		p.rangeVal <= 0 || !IsFinite(p.rangeVal) {
		return p
	}

	n := float64(len(data))
	for _, x := range data {
		y := (x - p.min) / p.rangeVal
		if y <= 0 || y >= 1 {
			return betaAutoPoint{}
		}
		p.s1 += math.Log(y)
		p.s2 += math.Log1p(-y)
		p.t1 += 1 / y
		p.t3 += y / (1 - y)
	}
	if !IsFinite(p.s1) || !IsFinite(p.s2) ||
		!IsFinite(p.t1) || !IsFinite(p.t3) {
		return betaAutoPoint{}
	}

	// n·Lbeta(α,β) − (α−1)·Σln y − (β−1)·Σln(1−y) + n·ln(max−min)
	p.nll = n*mathext.Lbeta(p.alpha, p.betaP) -
		(p.alpha-1)*p.s1 -
		(p.betaP-1)*p.s2 +
		n*math.Log(p.rangeVal)

	p.ok = IsFinite(p.nll)
	return p
}

// fitBetaAutoBounds schätzt Alpha, Beta, Min und Max gemeinsam per
// Maximum-Likelihood. Aufgerufen, wenn BetaFitter keine Grenzen vorgibt.
func fitBetaAutoBounds(data []float64) (ContinuousDistribution, error) {
	dataMin := data[0]
	dataMax := data[0]
	for _, v := range data {
		if v < dataMin {
			dataMin = v
		}
		if v > dataMax {
			dataMax = v
		}
	}
	dataRange := dataMax - dataMin
	if !(dataRange > 0) || !IsFinite(dataRange) {
		return nil, errors.New("daten haben keine Variation für Beta-Fit")
	}

	// Startgrenzen: kleiner Rand um die Daten (wie bisherige Heuristik,
	// der ML optimiert sie anschließend als freie Parameter).
	margin0 := 0.01 * dataRange
	startMin := dataMin - margin0
	startMax := dataMax + margin0

	// Startwerte α, β aus der Momentenmethode auf [0, 1].
	// α, β müssen > 1 sein (Parametrisierung α = 1 + exp(z)); Werte
	// unter 1 werden auf 1.05 gekappt.
	mean := stat.Mean(data, nil)
	variance := stat.Variance(data, nil)
	scaledMean := (mean - startMin) / (startMax - startMin)
	scaledVariance := variance / ((startMax - startMin) * (startMax - startMin))

	alpha0, beta0 := 2.0, 2.0
	if scaledVariance > 0 && scaledVariance < scaledMean*(1.0-scaledMean) {
		commonFactor := (scaledMean*(1.0-scaledMean)/scaledVariance - 1.0)
		a := scaledMean * commonFactor
		b := (1.0 - scaledMean) * commonFactor
		if a > 1.05 && IsFinite(a) {
			alpha0 = a
		}
		if b > 1.05 && IsFinite(b) {
			beta0 = b
		}
	}

	// z = [ln(α−1), ln(β−1), ln(dataMin−min), ln(max−dataMax)]
	initial := []float64{
		math.Log(alpha0 - 1),
		math.Log(beta0 - 1),
		math.Log(margin0),
		math.Log(margin0),
	}

	bestZ := []float64{math.NaN(), math.NaN(), math.NaN(), math.NaN()}
	bestF := math.Inf(1)

	// Func und Grad werden pro Iteration meist an derselben Stelle
	// ausgewertet; die O(n)-Auswertung wird daher für gleiche z gekappt.
	var lastZ [4]float64
	var lastP betaAutoPoint
	var cached bool
	eval := func(z []float64) betaAutoPoint {
		if cached && len(z) == 4 &&
			z[0] == lastZ[0] && z[1] == lastZ[1] &&
			z[2] == lastZ[2] && z[3] == lastZ[3] {
			return lastP
		}
		p := evalBetaAuto(z, data, dataMin, dataMax)
		if len(z) == 4 {
			copy(lastZ[:], z)
			lastP = p
			cached = true
		}
		return p
	}

	problem := optimize.Problem{
		Func: func(z []float64) float64 {
			p := eval(z)
			if !p.ok {
				return math.Inf(1)
			}
			if p.nll < bestF {
				bestF = p.nll
				copy(bestZ, z)
			}
			return p.nll
		},
		Grad: func(grad, z []float64) {
			p := eval(z)
			if !p.ok {
				for i := range grad {
					grad[i] = math.Inf(1)
				}
				return
			}

			n := float64(len(data))
			sum := p.alpha + p.betaP

			// Partielle nach (alpha, beta, min, max):
			// dNLL/dα = n·(ψ(α)−ψ(α+β)) − S1
			// dNLL/dβ = n·(ψ(β)−ψ(α+β)) − S2
			// dNLL/dm = [−(α−1)(n−T1) − (β−1)n − n] / L
			// dNLL/dM = [(α−1)n − (β−1)T3 + n] / L
			dAlpha := n*(mathext.Digamma(p.alpha)-mathext.Digamma(sum)) - p.s1
			dBeta := n*(mathext.Digamma(p.betaP)-mathext.Digamma(sum)) - p.s2
			dMin := (-(p.alpha-1)*(n-p.t1) - (p.betaP-1)*n - n) / p.rangeVal
			dMax := ((p.alpha-1)*n - (p.betaP-1)*p.t3 + n) / p.rangeVal

			// Kettenregel auf z:
			// dα/dz0 = α−1, dβ/dz1 = β−1,
			// dm/dz2 = −exp(z2) = min−dataMin, dM/dz3 = exp(z3) = max−dataMax
			grad[0] = dAlpha * (p.alpha - 1)
			grad[1] = dBeta * (p.betaP - 1)
			grad[2] = dMin * (p.min - dataMin)
			grad[3] = dMax * (p.max - dataMax)

			for i := range grad {
				if !IsFinite(grad[i]) {
					grad[i] = math.Inf(1)
				}
			}
		},
	}

	settings := &optimize.Settings{
		GradientThreshold: 1e-8,
		MajorIterations:   1000,
		FuncEvaluations:   10000,
		GradEvaluations:   10000,
	}

	result, err := optimize.Minimize(problem, initial, settings, &optimize.LBFGS{})

	var x []float64
	if err != nil {
		if (!errors.Is(err, optimize.ErrNoProgress) &&
			!errors.Is(err, optimize.ErrLinesearcherFailure)) ||
			math.IsNaN(bestZ[0]) {
			return nil, fmt.Errorf("Beta-MLE-Optimierung fehlgeschlagen: %w", err)
		}

		// Wie bei fitBetaMLE: besten gesehehen Punkt nur akzeptieren,
		// wenn der Gradient pro Beobachtung klein ist. Gradient im
		// z-Raum wie in problem.Grad.
		p := evalBetaAuto(bestZ, data, dataMin, dataMax)
		if !p.ok {
			return nil, fmt.Errorf("Beta-MLE-Optimierung fehlgeschlagen: %w", err)
		}
		n := float64(len(data))
		sum := p.alpha + p.betaP
		g0 := (n*(mathext.Digamma(p.alpha)-mathext.Digamma(sum)) - p.s1) * (p.alpha - 1)
		g1 := (n*(mathext.Digamma(p.betaP)-mathext.Digamma(sum)) - p.s2) * (p.betaP - 1)
		maxGrad := math.Max(math.Abs(g0), math.Abs(g1))
		if !IsFinite(maxGrad) || maxGrad/n > 1e-6 {
			return nil, fmt.Errorf("Beta-MLE-Optimierung fehlgeschlagen: %w", err)
		}
		x = bestZ
	} else {
		if result.Status.Early() {
			return nil, fmt.Errorf("Beta-MLE nicht konvergiert: %s", result.Status)
		}
		x = result.X
	}

	p := evalBetaAuto(x, data, dataMin, dataMax)
	if !p.ok {
		return nil, errors.New("Beta-MLE lieferte ungültige Parameter")
	}

	// Degenerationsguard: Der Parameterraum ist nicht kompakt. Läuft die
	// Optimierung in den Grenzfall α, β → ∞ mit entsprechend unendlichen
	// Grenzen, konvergiert die Beta gegen eine andere Familie (z. B.
	// Gamma mit verschobenem Nullpunkt), die bereits eigener Kandidat
	// ist. Ein endlicher Fit wird daher abgelehnt.
	if p.alpha > 1e4 || p.betaP > 1e4 || p.rangeVal > 1e3*dataRange {
		return nil, errors.New(
			"beta-grenzen laufen in einen entarteten grenzfall (gamma-ähnlich)",
		)
	}

	return Beta{
		Alpha:           p.alpha,
		Beta:            p.betaP,
		Min:             p.min,
		Max:             p.max,
		boundsEstimated: true,
	}, nil
}
