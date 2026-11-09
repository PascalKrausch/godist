package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/PascalKrausch/godist/pkg/distribution"
)

// DistributionInfo ist die maschinenlesbare Form eines Katalogeintrags.
type DistributionInfo struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Params         []string `json:"params"`
	Support        string   `json:"support"`
	FitNotes       string   `json:"fit_notes"`
	ZeroInflatable bool     `json:"zero_inflatable"`
}

// WriteDistributionsText schreibt den Katalog als Tabelle; Fit-Hinweise folgen darunter.
func WriteDistributionsText(w io.Writer, specs []distribution.Spec) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Name\tParameter\tTräger\tBeschreibung")
	for _, s := range specs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.Name, strings.Join(s.Params, ", "), s.Support, s.Description)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	var notes []string
	for _, s := range specs {
		if s.FitNotes != "" {
			notes = append(notes, fmt.Sprintf("  %s: %s", s.Name, s.FitNotes))
		}
	}
	if len(notes) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Fit-Hinweise:")
		for _, n := range notes {
			fmt.Fprintln(w, n)
		}
	}
	return nil
}

// WriteDistributionsJSON schreibt den Katalog als JSON-Array.
func WriteDistributionsJSON(w io.Writer, specs []distribution.Spec) error {
	infos := make([]DistributionInfo, 0, len(specs))
	for _, s := range specs {
		params := s.Params
		if params == nil {
			params = []string{}
		}
		infos = append(infos, DistributionInfo{
			Name:           s.Name,
			Description:    s.Description,
			Params:         params,
			Support:        s.Support,
			FitNotes:       s.FitNotes,
			ZeroInflatable: s.ZeroInflatable,
		})
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(infos)
}
