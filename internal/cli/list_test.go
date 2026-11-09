package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/PascalKrausch/godist/pkg/distribution"
)

func TestWriteDistributionsJSONShape(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteDistributionsJSON(&buf, distribution.Catalog()); err != nil {
		t.Fatalf("WriteDistributionsJSON: %v", err)
	}

	var infos []DistributionInfo
	if err := json.Unmarshal(buf.Bytes(), &infos); err != nil {
		t.Fatalf("Ausgabe ist kein gültiges JSON-Array: %v", err)
	}
	if len(infos) != len(distribution.Catalog()) {
		t.Fatalf("got %d entries, want %d", len(infos), len(distribution.Catalog()))
	}

	// Verteilungen ohne Parameter müssen ein leeres Array liefern, kein null.
	for _, info := range infos {
		if info.Params == nil {
			t.Errorf("%s: params is nil, want empty slice", info.Name)
		}
	}
	if !strings.Contains(buf.String(), `"zero_inflatable": true`) {
		t.Error("JSON enthält keinen Eintrag mit zero_inflatable: true")
	}
}

func TestWriteDistributionsTextListsAllAndFitNotes(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteDistributionsText(&buf, distribution.Catalog()); err != nil {
		t.Fatalf("WriteDistributionsText: %v", err)
	}
	out := buf.String()

	for _, spec := range distribution.Catalog() {
		if !strings.Contains(out, spec.Name) {
			t.Errorf("text output lacks distribution %q", spec.Name)
		}
	}
	if !strings.Contains(out, "Fit-Hinweise") {
		t.Error("text output lacks the Fit-Hinweise section")
	}
}
