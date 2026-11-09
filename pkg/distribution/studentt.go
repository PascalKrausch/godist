package distribution

import (
	"math"
	"sort"

	"gonum.org/v1/gonum/mathext"
)

// StudentT repräsentiert eine verschobene und skalierte t-Verteilung mit Lageparameter Mu,
// Skalierungsparameter Scale (s > 0) und Freiheitsgraden Nu (ν > 0). Für ν = 1 ergibt sich
// die Cauchy-Verteilung, für ν → ∞ die Normalverteilung.
type StudentT struct {
	Mu    float64 // Lageparameter
	Scale float64 // Skalierungsparameter (s > 0)
	Nu    float64 // Freiheitsgrade (ν > 0)
}

func (s StudentT) Quantile(p float64) float64 {
	if !s.Valid() || !validProbability(p) {
		return math.NaN()
	}
	switch p {
	case 0:
		return math.Inf(-1)
	case 1:
		return math.Inf(1)
	}
	// Bisektion auf der Standard-CDF; die Klammer wird bei Bedarf verdoppelt.
	lo, hi := -1.0, 1.0
	for stdStudentTCDF(lo, s.Nu) > p {
		lo *= 2
	}
	for stdStudentTCDF(hi, s.Nu) < p {
		hi *= 2
	}
	for i := 0; i < 200 && hi-lo > 1e-14*math.Max(1, math.Abs(lo)); i++ {
		mid := 0.5 * (lo + hi)
		if stdStudentTCDF(mid, s.Nu) < p {
			lo = mid
		} else {
			hi = mid
		}
	}
	return s.Mu + s.Scale*0.5*(lo+hi)
}

func (s StudentT) Params() []float64 {
	return []float64{s.Mu, s.Scale, s.Nu}
}

func (s StudentT) ParamNames() []string {
	return []string{"mu", "scale", "nu"}
}

func (s StudentT) Valid() bool {
	return s.Scale > 0 && s.Nu > 0 && IsFinite(s.Mu) && IsFinite(s.Scale) && IsFinite(s.Nu)
}

func (s StudentT) PDF(x float64) float64 {
	if !s.Valid() {
		return 0
	}
	return math.Exp(s.LogPDF(x))
}

func (s StudentT) LogPDF(x float64) float64 {
	if !s.Valid() {
		return math.Inf(-1)
	}
	lgNum, _ := math.Lgamma((s.Nu + 1) / 2)
	lgDen, _ := math.Lgamma(s.Nu / 2)
	z := (x - s.Mu) / s.Scale
	return lgNum - lgDen - 0.5*math.Log(s.Nu*math.Pi) - math.Log(s.Scale) -
		(s.Nu+1)/2*math.Log1p(z*z/s.Nu)
}

func (s StudentT) CDF(x float64) float64 {
	if !s.Valid() {
		return 0
	}
	return stdStudentTCDF((x-s.Mu)/s.Scale, s.Nu)
}

// stdStudentTCDF ist die CDF der Standard-t-Verteilung mit ν Freiheitsgraden. Mit x = t²/(ν+t²) gilt
// P(|T| > |t|) = I_{1−x}(ν/2, 1/2) = 1 − I_x(1/2, ν/2). Die Form wird so gewählt, dass das Argument
// der regularisierten Betafunktion klein bleibt und die Genauigkeit nahe t = 0 erhalten bleibt.
func stdStudentTCDF(t, nu float64) float64 {
	x := t * t / (nu + t*t)
	var pAbs float64
	if x < 0.5 {
		pAbs = 1 - mathext.RegIncBeta(0.5, nu/2, x)
	} else {
		pAbs = mathext.RegIncBeta(nu/2, 0.5, nu/(nu+t*t))
	}
	tail := 0.5 * pAbs
	if t >= 0 {
		return 1 - tail
	}
	return tail
}

// StudentTFitter schätzt Mu, Scale und Nu per Maximum-Likelihood. Für festes ν wird Mu und Scale
// per EM-Algorithmus bestimmt. ν wird durch eine Golden-Section-Suche über das Profil-Log-Likelihood
// auf log(ν) gefunden.
type StudentTFitter struct{}

func (f StudentTFitter) Name() string {
	return "StudentT"
}

const (
	studentTNuMin = 0.1
	studentTNuMax = 1e9
	studentTEMTol = 1e-7
	studentTEMMax = 5000
	// studentTNuTol ist die Toleranz auf log(ν) der Golden-Section-Suche (relative Genauigkeit von ν ca. 1e-4).
	studentTNuTol = 1e-4
)

// studentTLogLik summiert die Log-Likelihood. Die normierende Konstante wird einmal berechnet,
// damit die Schleife keine Gamma-Funktionen pro Beobachtung auswertet.
func studentTLogLik(data []float64, s StudentT) float64 {
	lgNum, _ := math.Lgamma((s.Nu + 1) / 2)
	lgDen, _ := math.Lgamma(s.Nu / 2)
	c := lgNum - lgDen - 0.5*math.Log(s.Nu*math.Pi) - math.Log(s.Scale)
	sum := 0.0
	for _, x := range data {
		z := (x - s.Mu) / s.Scale
		sum += math.Log1p(z * z / s.Nu)
	}
	return float64(len(data))*c - (s.Nu+1)/2*sum
}

func (f StudentTFitter) Fit(data []float64) (ContinuousDistribution, error) {
	cleaned := CleanDataWithZero(data)
	if len(cleaned) == 0 {
		return nil, ErrEmptyData
	}
	if len(cleaned) < 3 {
		return nil, ErrFitFailed
	}

	sorted := append([]float64(nil), cleaned...)
	sort.Float64s(sorted)
	n := float64(len(sorted))
	mu0 := sorted[int(0.5*(n-1))]
	s0 := (sorted[int(0.75*(n-1))] - sorted[int(0.25*(n-1))]) / 2
	if s0 <= 0 || !IsFinite(s0) {
		return nil, ErrFitFailed
	}

	// Jeder Aufruf startet EM beim Ergebnis des zuletzt erfolgreichen Aufrufs (Warm Start).
	warmMu, warmS := mu0, s0
	profile := func(logNu float64) (StudentT, float64, bool) {
		dist, ok := studentTEM(cleaned, warmMu, warmS, math.Exp(logNu))
		if !ok {
			return StudentT{}, math.Inf(-1), false
		}
		warmMu, warmS = dist.Mu, dist.Scale
		return dist, studentTLogLik(cleaned, dist), true
	}

	// Golden-Section-Suche auf log(ν) zur Maximierung des Profil-Log-Likelihoods.
	const phi = 0.6180339887498949
	a, b := math.Log(studentTNuMin), math.Log(studentTNuMax)
	c := b - phi*(b-a)
	d := a + phi*(b-a)
	_, fc, okC := profile(c)
	_, fd, okD := profile(d)
	if !okC {
		fc = math.Inf(-1)
	}
	if !okD {
		fd = math.Inf(-1)
	}
	for i := 0; i < 100 && b-a > studentTNuTol; i++ {
		if fc > fd {
			b, d, fd = d, c, fc
			c = b - phi*(b-a)
			_, fc, okC = profile(c)
			if !okC {
				fc = math.Inf(-1)
			}
		} else {
			a, c, fc = c, d, fd
			d = a + phi*(b-a)
			_, fd, okD = profile(d)
			if !okD {
				fd = math.Inf(-1)
			}
		}
	}

	dist, _, ok := profile(0.5 * (a + b))
	if !ok {
		return nil, ErrFitFailed
	}
	return dist, nil
}

// studentTEM liefert Mu und Scale für festes ν per EM. Die Gewichte sind
// w_i = (ν+1)/(ν + d_i²/s²) mit d_i = x_i − μ; μ ist das gewichtete Mittel, s² das gewichtete Mittel
// der quadrierten Abweichungen.
func studentTEM(data []float64, mu, s, nu float64) (StudentT, bool) {
	n := float64(len(data))
	for iter := 0; iter < studentTEMMax; iter++ {
		s2 := s * s
		sumW, sumWX := 0.0, 0.0
		for _, x := range data {
			d := x - mu
			w := (nu + 1) / (nu + d*d/s2)
			sumW += w
			sumWX += w * x
		}
		muNew := sumWX / sumW

		sumWD := 0.0
		for _, x := range data {
			d := x - muNew
			w := (nu + 1) / (nu + (x-mu)*(x-mu)/s2)
			sumWD += w * d * d
		}
		sNew := math.Sqrt(sumWD / n)

		if !IsFinite(muNew) || !IsFinite(sNew) || sNew <= 0 {
			return StudentT{}, false
		}
		converged := math.Abs(muNew-mu) <= studentTEMTol*(1+math.Abs(mu)) &&
			math.Abs(sNew-s) <= studentTEMTol*s
		mu, s = muNew, sNew
		if converged {
			break
		}
	}
	return StudentT{Mu: mu, Scale: s, Nu: nu}, true
}
