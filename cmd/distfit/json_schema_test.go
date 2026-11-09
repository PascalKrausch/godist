package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/PascalKrausch/godist/internal/cli"
	"github.com/PascalKrausch/godist/internal/stats"
	"github.com/PascalKrausch/godist/pkg/distribution"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const schemaPath = "../../docs/json.schema.json"

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	const id = "https://github.com/PascalKrausch/godist/docs/json.schema.json"
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(id, doc); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(id)
	if err != nil {
		t.Fatalf("schema does not compile: %v", err)
	}
	return schema
}

func validateJSON(t *testing.T, schema *jsonschema.Schema, data []byte) error {
	t.Helper()
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, data)
	}
	return schema.Validate(instance)
}

func TestCLIJSONOutputMatchesSchema(t *testing.T) {
	schema := compileSchema(t)
	txt := writeFile(t, "d.txt", "1.5\nabc\n2.5\n3.1\n4.2\n2.2\n")

	tests := []struct {
		name string
		args []string
	}{
		{"csv with column", []string{"-file=" + wineCSV, "-column=density", "-all", "-format=json"}},
		{"txt with discarded value and no column", []string{"-file=" + txt, "-all", "-format=json"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(t, tt.args...)
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr)
			}
			if err := validateJSON(t, schema, []byte(stdout)); err != nil {
				t.Errorf("stdout violates schema: %v", err)
			}
		})
	}
}

func TestJSONReportWithFailuresAndNullStatsMatchesSchema(t *testing.T) {
	schema := compileSchema(t)
	data := []float64{1.5, 2.5, 3.1, 4.2, 2.2, 3.3, 2.8, 1.9, 3.7, 2.6}
	results, _, err := distribution.SelectAllModels(data)
	if err != nil || len(results) == 0 {
		t.Fatalf("fitting failed: %v", err)
	}
	report := cli.BuildFitReport(
		results[:1],
		[]distribution.FitFailure{{Name: "Pareto", Error: "no convergence"}},
		"synthetic.csv",
		"",
		stats.Describe(data),
		0,
	)
	report.Descriptive.Kurtosis = math.NaN()

	var buf bytes.Buffer
	if err := cli.WriteJSONReport(&buf, report); err != nil {
		t.Fatal(err)
	}
	if err := validateJSON(t, schema, buf.Bytes()); err != nil {
		t.Errorf("report violates schema: %v\n%s", err, buf.String())
	}
	if !strings.Contains(buf.String(), `"failures": [`) {
		t.Errorf("failures must be present as array:\n%s", buf.String())
	}
}

func TestSchemaRejectsInvalidDocuments(t *testing.T) {
	schema := compileSchema(t)
	code, stdout, _ := runCLI(t, "-file="+wineCSV, "-column=density", "-all", "-format=json")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}

	mutations := map[string]func(map[string]any){
		"missing schema_version": func(m map[string]any) { delete(m, "schema_version") },
		"unknown schema_version": func(m map[string]any) { m["schema_version"] = 2 },
		"missing failures":       func(m map[string]any) { delete(m, "failures") },
		"string sample_size": func(m map[string]any) {
			m["sample_size"] = "1599"
		},
		"model without aic": func(m map[string]any) {
			models := m["models"].([]any)
			delete(models[0].(map[string]any), "aic")
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
				t.Fatal(err)
			}
			mutate(doc)
			data, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateJSON(t, schema, data); err == nil {
				t.Error("schema accepted an invalid document")
			}
		})
	}
}

func TestJSONWriterWritesNothingOnNonFiniteParam(t *testing.T) {
	report := cli.FitReport{
		Models: []cli.FitRow{{Name: "Broken", Params: map[string]float64{"mu": math.NaN()}}},
	}
	var buf bytes.Buffer
	if err := cli.WriteJSONReport(&buf, report); err == nil {
		t.Fatal("expected error for NaN parameter")
	}
	if buf.Len() != 0 {
		t.Errorf("partial output on stdout would break consumers: %q", buf.String())
	}
}
