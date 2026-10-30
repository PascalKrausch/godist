package distribution

import (
	"errors"
	"math"
)

// Logistic repräsentiert eine logistische Verteilung mit Location (μ) und Scale (s > 0).
type Logistic struct {
	Mu    float64 // Lageparameter (Median)
	Scale float64 // Skalierungsparameter (s > 0)
}

func (l Logistic) Quantile(p float64) float64 {
	if !l.Valid() || !validProbability(p) {
		return math.NaN()
	}
	return l.Mu + l.Scale*math.Log(p/(1-p))
}

func (l Logistic) Params() []float64 {
	return []float64{l.Mu, l.Scale}
}

func (l Logistic) ParamNames() []string {
	return []string{"mu", "scale"}
}

func (l Logistic) Valid() bool {
	return l.Scale > 0 && IsFinite(l.Mu) && IsFinite(l.Scale)
}

func (l Logistic) PDF(x float64) float64 {
	if !l.Valid() {
		return 0
	}
	return math.Exp(l.LogPDF(x))
}

func (l Logistic) LogPDF(x float64) float64 {
	if !l.Valid() {
		return math.Inf(-1)
	}
	// f(x) = e^{-z} / (s (1 + e^{-z})^2) mit z = (x - μ) / s
	// ln f(x) = -z - 2·ln(1 + e^{-z}) - ln(s)
	z := (x - l.Mu) / l.Scale
	return -z - 2*logisticSoftplus(-z) - math.Log(l.Scale)
}

func (l Logistic) CDF(x float64) float64 {
	if !l.Valid() {
		return 0
	}
	return 1 / (1 + math.Exp(-(x-l.Mu)/l.Scale))
}

// logisticSoftplus berechnet ln(1 + e^t) ohne Überlauf für große |t|.
func logisticSoftplus(t float64) float64 {
	if t > 0 {
		return t + math.Log1p(math.Exp(-t))
	}
	return math.Log1p(math.Exp(t))
}

// LogisticFitter schätzt Mu und Scale per Maximum-Likelihood.
type LogisticFitter struct{}

func (f LogisticFitter) Name() string {
	return "Logistic"
}

// Fit löst die Score-Gleichungen mit z_i = (x_i - μ) / s:
//
//	Σ tanh(z_i / 2) = 0                 (nach μ)
//	Σ (z_i · tanh(z_i / 2) - 1) = 0     (nach s, mit μ = μ(s) profiliert)
//
// logisticNewton löst beide Gleichungen gemeinsam per Newton. Konvergiert es nicht, bestimmt
// logisticBisect die Lösung verschachtelt: innen μ per Bisektion, außen log(s) per Illinois-Verfahren.
func (f LogisticFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	n := len(cleaned)
	if n < 2 {
		return nil, errors.New("zu wenige Datenpunkte für Logistic-Fit")
	}

	lo, hi := cleaned[0], cleaned[0]
	for _, x := range cleaned {
		lo = math.Min(lo, x)
		hi = math.Max(hi, x)
	}
	spread := hi - lo
	if spread <= 0 {
		return nil, errors.New("logistic-parameter konnten nicht geschätzt werden")
	}

	mean := 0.0
	for _, x := range cleaned {
		mean += x
	}
	mean /= float64(n)
	var variance float64
	for _, x := range cleaned {
		variance += (x - mean) * (x - mean)
	}
	sd := math.Sqrt(variance / float64(n))

	mu, scale, ok := logisticNewton(cleaned, mean, math.Log(sd*math.Sqrt(3)/math.Pi))
	if !ok {
		mu, scale = logisticBisect(cleaned, lo, hi, spread, mean, sd)
	}

	if !IsFinite(mu) || !IsFinite(scale) || scale <= 0 {
		return nil, errors.New("logistic-parameter konnten nicht geschätzt werden")
	}

	return Logistic{Mu: mu, Scale: scale}, nil
}

// logisticBisect ist der Rückfall-Solver mit verschachtelter Bisektion (siehe Fit).
func logisticBisect(cleaned []float64, lo, hi, spread, mean, sd float64) (float64, float64) {
	n := len(cleaned)
	mu := mean
	profile := func(logS float64) float64 {
		s := math.Exp(logS)
		mu = logisticMuForScale(cleaned, lo, hi, s, mu)
		return logisticScaleScore(cleaned, mu, s)
	}

	// Enges Intervall um die Standardabweichung; fällt zurück auf die Spannweite, falls kein Vorzeichenwechsel vorliegt.
	logLo, logHi := math.Log(sd*1e-4), math.Log(sd*1e4)
	fLo, fHi := profile(logLo), profile(logHi)
	if (fLo > 0) == (fHi > 0) {
		logLo, logHi = math.Log(spread*1e-12), math.Log(spread*1e6)
		fLo, fHi = profile(logLo), profile(logHi)
	}

	// Illinois-Verfahren (modifizierte Regula falsi) auf der monoton fallenden Profil-Gleichung in log(s).
	x := 0.5 * (logLo + logHi)
	side := 0
	for i := 0; i < 200 && logHi-logLo > 1e-10; i++ {
		prev := x
		x = logHi - fHi*(logHi-logLo)/(fHi-fLo)
		if !(x > logLo && x < logHi) {
			x = 0.5 * (logLo + logHi)
		}
		fX := profile(x)
		if fX == 0 {
			break
		}
		if math.Abs(fX) <= 1e-12*float64(n) || math.Abs(x-prev) <= 1e-8 {
			break
		}
		if fX > 0 {
			logLo, fLo = x, fX
			if side == 1 {
				fHi /= 2
			}
			side = 1
		} else {
			logHi, fHi = x, fX
			if side == -1 {
				fLo /= 2
			}
			side = -1
		}
	}

	scale := math.Exp(x)
	mu = logisticMuForScale(cleaned, lo, hi, scale, mu)
	return mu, scale
}

// logisticNewton löst das Gleichungssystem (Σtanh(z/2), Σ(z·tanh(z/2) − 1)) in (μ, log s) per gedämpftem Newton.
// ok ist false, wenn keine Konvergenz erreicht wird.
func logisticNewton(data []float64, mu, theta float64) (float64, float64, bool) {
	s := math.Exp(theta)
	g, jac := logisticScoreJacobian(data, mu, s)
	for iter := 0; iter < 100; iter++ {
		det := jac[0][0]*jac[1][1] - jac[0][1]*jac[1][0]
		if det == 0 || !IsFinite(det) {
			return 0, 0, false
		}
		dMu := -(g[0]*jac[1][1] - jac[0][1]*g[1]) / det
		dTheta := -(jac[0][0]*g[1] - jac[1][0]*g[0]) / det

		// Schritt in log(s) begrenzen, damit s nicht um Größenordnungen springt.
		step := 1.0
		if math.Abs(dTheta) > 1 {
			step = 1 / math.Abs(dTheta)
		}
		merit := math.Abs(g[0]) + math.Abs(g[1])
		accepted := false
		var nMu, nTheta float64
		var nG [2]float64
		var nJac [2][2]float64
		for k := 0; k < 30; k++ {
			nMu = mu + step*dMu
			nTheta = theta + step*dTheta
			nG, nJac = logisticScoreJacobian(data, nMu, math.Exp(nTheta))
			if m := math.Abs(nG[0]) + math.Abs(nG[1]); IsFinite(m) && m < merit {
				accepted = true
				break
			}
			step /= 2
		}
		if !accepted {
			return mu, s, merit <= 1e-10*float64(len(data))
		}

		converged := math.Abs(nMu-mu) <= 1e-10*s && math.Abs(nTheta-theta) <= 1e-10
		mu, theta, g, jac = nMu, nTheta, nG, nJac
		s = math.Exp(theta)
		if converged {
			return mu, s, true
		}
	}
	return 0, 0, false
}

// logisticScoreJacobian liefert Score (Σtanh(z/2), Σ(z·tanh(z/2) − 1)) und dessen Jacobi-Matrix
// bezüglich (μ, log s) in einem einzigen Durchlauf, mit z = (x − μ)/s.
func logisticScoreJacobian(data []float64, mu, s float64) ([2]float64, [2][2]float64) {
	var g [2]float64
	var jac [2][2]float64
	for _, x := range data {
		z := (x - mu) / s
		t := math.Tanh(z / 2)
		w := 1 - t*t
		g[0] += t
		g[1] += z*t - 1
		jac[0][0] += -w / (2 * s)
		jac[0][1] += -z * w / 2
		jac[1][0] += (-t - z*w/2) / s
		jac[1][1] += -z*t - z*z*w/2
	}
	return g, jac
}

// logisticEps ist die relative Maschinengenauigkeit für float64, damit die Newton-Abbruchbedingung erreichbar bleibt.
const logisticEps = 2.220446049250313e-16

// logisticMuForScale bestimmt μ für festes s als Nullstelle von g(μ) = Σ tanh((x - μ) / (2s)) im Intervall [lo, hi].
// start ist der Startwert für Newton (z. B. μ aus dem vorherigen Schritt).
// Newton-Schritte werden durch die Bisektion-Klammer abgesichert.
func logisticMuForScale(data []float64, lo, hi, s, start float64) float64 {
	a, b := lo, hi
	mu := start
	for i := 0; i < 200; i++ {
		var g, dg float64
		for _, x := range data {
			t := math.Tanh((x - mu) / (2 * s))
			g += t
			dg += 1 - t*t
		}
		if g > 0 {
			a = mu
		} else {
			b = mu
		}
		next := mu - g/(-dg/(2*s))
		if !(next > a && next < b) {
			next = 0.5 * (a + b)
		}
		tol := 1e-10*s + 4*logisticEps*math.Abs(mu)
		if math.Abs(next-mu) <= tol || b-a <= tol {
			return next
		}
		mu = next
	}
	return mu
}

// logisticScaleScore ist die Score-Gleichung nach s bei gegebenem μ.
func logisticScaleScore(data []float64, mu, s float64) float64 {
	var sum float64
	for _, x := range data {
		z := (x - mu) / s
		sum += z * math.Tanh(z/2)
	}
	return sum - float64(len(data))
}
