package distribution

import (
	"errors"
	"fmt"
	"strings"
)

// SelectFitters liefert die Kandidaten für eine Auswahl. Mit include werden genau diese
// Verteilungen verwendet, mit exclude wird der Katalog um diese reduziert. Beides zusammen ist
// ein Fehler. Unbekannte Namen und, bei PolicyZeroInflated, Verteilungen ohne ZI-Variante
// sind ebenfalls Fehler, damit Tippfehler nicht stillschweigend ignoriert werden.
func SelectFitters(policy ZeroPolicy, include, exclude []string) ([]Fitter, error) {
	if len(include) > 0 && len(exclude) > 0 {
		return nil, errors.New("include und exclude schließen sich aus")
	}

	excludedSpecs, err := resolveSpecs(exclude, false)
	if err != nil {
		return nil, err
	}
	excluded := make(map[string]bool, len(excludedSpecs))
	for _, spec := range excludedSpecs {
		excluded[spec.Name] = true
	}

	var specs []Spec
	if len(include) > 0 {
		specs, err = resolveSpecs(include, policy == PolicyZeroInflated)
		if err != nil {
			return nil, err
		}
	} else {
		for _, spec := range Catalog() {
			if policy == PolicyZeroInflated && !spec.ZeroInflatable {
				continue
			}
			if excluded[spec.Name] {
				continue
			}
			specs = append(specs, spec)
		}
	}

	if len(specs) == 0 {
		return nil, errors.New("keine Verteilung übrig; include oder exclude prüfen")
	}

	fitters := make([]Fitter, 0, len(specs))
	for _, spec := range specs {
		if policy == PolicyZeroInflated {
			fitters = append(fitters, ZeroInflatedFitter{Base: spec.Fitter})
		} else {
			fitters = append(fitters, spec.Fitter)
		}
	}
	return fitters, nil
}

// resolveSpecs löst Namen über Lookup auf, entfernt Duplikate und behält die Reihenfolge.
// Mit requireZI müssen alle Verteilungen eine Zero-Inflated-Variante haben.
func resolveSpecs(names []string, requireZI bool) ([]Spec, error) {
	var specs []Spec
	seen := make(map[string]bool, len(names))
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		spec, ok := Lookup(name)
		if !ok {
			return nil, fmt.Errorf("unbekannte Verteilung %q (gültig: %s)", name, catalogNames())
		}
		if requireZI && !spec.ZeroInflatable {
			return nil, fmt.Errorf("%s unterstützt keine Zero-Inflation", spec.Name)
		}
		if !seen[spec.Name] {
			seen[spec.Name] = true
			specs = append(specs, spec)
		}
	}
	return specs, nil
}

func catalogNames() string {
	var names []string
	for _, spec := range Catalog() {
		names = append(names, spec.Name)
	}
	return strings.Join(names, ", ")
}
