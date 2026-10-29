package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadDatasetDecimalComma(t *testing.T) {
	path := writeTempFile(t, "d.txt", "1,5\n2,25\n")
	ds, err := ReadDataset(path, ReadOptions{Decimal: ","})
	if err != nil {
		t.Fatal(err)
	}
	if want := []float64{1.5, 2.25}; !reflect.DeepEqual(ds.Values, want) {
		t.Fatalf("Values = %v, want %v", ds.Values, want)
	}
}

func TestReadDatasetDiscardsInvalidTxtValues(t *testing.T) {
	path := writeTempFile(t, "d.txt", "1.5\nabc\nNaN\nInf\n\n2.5\n")
	ds, err := ReadDataset(path, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if want := []float64{1.5, 2.5}; !reflect.DeepEqual(ds.Values, want) {
		t.Fatalf("Values = %v, want %v", ds.Values, want)
	}
	if len(ds.Discarded) != 3 {
		t.Fatalf("Discarded = %d, want 3: %+v", len(ds.Discarded), ds.Discarded)
	}
	if ds.Discarded[0].Location != "Zeile 2" || ds.Discarded[0].Reason != "nicht als Zahl lesbar" {
		t.Errorf("first discard = %+v", ds.Discarded[0])
	}
}

func TestReadDatasetCSVSingleNumericColumnIsAutoSelected(t *testing.T) {
	path := writeTempFile(t, "d.csv", "name;value\na;1.5\nb;2.5\n")
	ds, err := ReadDataset(path, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if ds.Column != "value" {
		t.Errorf("Column = %q, want value", ds.Column)
	}
	if want := []float64{1.5, 2.5}; !reflect.DeepEqual(ds.Values, want) {
		t.Fatalf("Values = %v, want %v", ds.Values, want)
	}
}

func TestReadDatasetCSVMultipleNumericColumnsRequireColumn(t *testing.T) {
	path := writeTempFile(t, "d.csv", "a;b\n1;2\n3;4\n")
	_, err := ReadDataset(path, ReadOptions{})
	if err == nil || !strings.Contains(err.Error(), "-column") {
		t.Fatalf("err = %v, want hint to use -column", err)
	}
}

func TestReadDatasetCSVColumnByNameAndIndex(t *testing.T) {
	path := writeTempFile(t, "d.csv", "a;b\n1;10\n3;30\n")

	byName, err := ReadDataset(path, ReadOptions{Column: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []float64{10, 30}; !reflect.DeepEqual(byName.Values, want) {
		t.Errorf("by name Values = %v, want %v", byName.Values, want)
	}

	byIndex, err := ReadDataset(path, ReadOptions{Column: "0"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []float64{1, 3}; !reflect.DeepEqual(byIndex.Values, want) {
		t.Errorf("by index Values = %v, want %v", byIndex.Values, want)
	}
}

func TestReadDatasetCSVUnknownColumnListsAvailable(t *testing.T) {
	path := writeTempFile(t, "d.csv", "a;b\n1;10\n")
	_, err := ReadDataset(path, ReadOptions{Column: "zz"})
	if err == nil || !strings.Contains(err.Error(), "Verfügbare Spalten: a, b") {
		t.Fatalf("err = %v, want list of available columns", err)
	}
}

func TestReadDatasetHeaderlessCSVRejectsColumnName(t *testing.T) {
	path := writeTempFile(t, "d.csv", "1;10\n3;30\n")
	_, err := ReadDataset(path, ReadOptions{Column: "density"})
	if err == nil {
		t.Fatal("expected error for name on headerless CSV")
	}
}

func TestReadDatasetCSVRaggedRowDiscardsMissingCell(t *testing.T) {
	path := writeTempFile(t, "d.csv", "a;b\n1;10\n2\n3;30\n")
	ds, err := ReadDataset(path, ReadOptions{Column: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []float64{10, 30}; !reflect.DeepEqual(ds.Values, want) {
		t.Fatalf("Values = %v, want %v", ds.Values, want)
	}
	if len(ds.Discarded) != 1 {
		t.Fatalf("Discarded = %+v, want one entry", ds.Discarded)
	}
}

func TestReadDatasetStripsUTF8BOM(t *testing.T) {
	path := writeTempFile(t, "d.csv", "\ufeffvalue\n1.5\n2.5\n")
	ds, err := ReadDataset(path, ReadOptions{Column: "value"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []float64{1.5, 2.5}; !reflect.DeepEqual(ds.Values, want) {
		t.Fatalf("Values = %v, want %v", ds.Values, want)
	}
}

func TestReadDatasetJSONNullIsDiscarded(t *testing.T) {
	path := writeTempFile(t, "d.json", `[1.5, null, 2.5, "x"]`)
	ds, err := ReadDataset(path, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if want := []float64{1.5, 2.5}; !reflect.DeepEqual(ds.Values, want) {
		t.Fatalf("Values = %v, want %v", ds.Values, want)
	}
	if len(ds.Discarded) != 2 {
		t.Fatalf("Discarded = %+v, want 2 entries", ds.Discarded)
	}
}

func TestReadDatasetColumnOnlyForCSV(t *testing.T) {
	path := writeTempFile(t, "d.txt", "1\n2\n")
	if _, err := ReadDataset(path, ReadOptions{Column: "x"}); err == nil {
		t.Fatal("expected error for -column on txt file")
	}
}

func TestReadDatasetNoValidValuesIsError(t *testing.T) {
	path := writeTempFile(t, "d.txt", "abc\nNaN\n")
	if _, err := ReadDataset(path, ReadOptions{}); err == nil {
		t.Fatal("expected error when no valid values remain")
	}
}

func TestReadDatasetNestedJSON(t *testing.T) {
	// Test mit verschachtelter JSON-Struktur wie Open-Meteo
	path := writeTempFile(t, "d.json", `{"hourly":{"wind_speed_10m":[1.5,2.5,3.5]}}`)
	ds, err := ReadDataset(path, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if want := []float64{1.5, 2.5, 3.5}; !reflect.DeepEqual(ds.Values, want) {
		t.Fatalf("Values = %v, want %v", ds.Values, want)
	}
}

func TestReadDatasetNestedJSONMultipleArrays(t *testing.T) {
	// Test mit mehreren numerischen Arrays - sollte das erste finden
	path := writeTempFile(t, "d.json", `{"a":[1,2],"b":[3,4]}`)
	ds, err := ReadDataset(path, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Das Ergebnis sollte das erste Array sein ([1,2])
	if want := []float64{1, 2}; !reflect.DeepEqual(ds.Values, want) {
		t.Fatalf("Values = %v, want %v", ds.Values, want)
	}
}
