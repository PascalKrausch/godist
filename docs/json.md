# JSON-Ausgabe

JSON wird im aktuellen Prototyp nur mit `-all -format=json` ausgegeben. Der nächste Schritt ist, das JSON-Format als stabile Schnittstelle zu definieren und zu testen. Es soll mindestens Eingabemetadaten, Stichprobengröße, Auswahlkriterium, erfolgreiche Modelle, benannte Parameter, Metriken und fehlgeschlagene Fits enthalten.

Aufruf:

```bash
distfit -file=measurements.csv -column=wind_speed_10m -all -format=json
```

Bei JSON-Ausgabe gehört ausschließlich das JSON-Dokument auf `stdout`; Statusmeldungen und Fehler gehören auf `stderr`. So lässt sich die Ausgabe zuverlässig an andere Werkzeuge weiterleiten.

## Vertrag

Der verbindliche Vertrag ist [`json.schema.json`](json.schema.json) (JSON Schema, Draft 2020-12). Die Tests in `cmd/distfit/json_schema_test.go` prüfen die CLI-Ausgabe dagegen.

- `schema_version` ist immer vorhanden (aktuell `1`). Neue optionale Felder sind nicht-brechend; Umbenennungen oder Entfernungen erhöhen die Version.
- `failures` ist immer vorhanden (ggf. leeres Array).
- `models[].bootstrap` ist optional und nur vorhanden, wenn `-bootstrap > 0` für dieses Modell lief (siehe [CLI.md, Bootstrap](CLI.md#bootstrap-konfidenzintervalle)). Es enthält `method` (`nonparametric` oder `parametric`), `requested`/`replicates`/`failed` (Anzahl Wiederholungen und Ausfälle), `level` (Konfidenzniveau), `seed` (für `-bootstrap-seed`), `duration_ms`, `params` (Array aus `name`, `mean`, `lo`, `hi`) und `metrics` (`aic`, `bic`, `ks`, `rmse` als `mean`/`lo`/`hi`).
- Nicht berechenbare deskriptive Kennzahlen sind `null`.
- Ist ein Wert nicht darstellbar (NaN/Inf), gibt es keine Teilausgabe: stdout bleibt leer, Exit-Code 1.

## Beispiel

```json
{
  "schema_version": 1,
  "input": "daten.csv",
  "column": "density",
  "sample_size": 1000,
  "discarded": 5,
  "candidates": 9,
  "best_distribution": "Weibull",
  "total_duration_ms": 42.7,
  "descriptive": {
    "n": 1000,
    "mean": 1.02,
    "median": 0.98,
    "variance": 0.31,
    "std_dev": 0.557,
    "min": 0.01,
    "q1": 0.55,
    "q3": 1.41,
    "max": 3.9,
    "skewness": 0.84,
    "kurtosis": 0.37
  },
  "models": [
    {
      "name": "Weibull",
      "params": {"k": 2.5, "lambda": 1.2},
      "aic": 1234.5,
      "delta_aic": 0.0,
      "bic": 1240.2,
      "ks_statistic": 0.0234,
      "rmse": 0.0156,
      "duration_ms": 12.3,
      "bootstrap": {
        "method": "nonparametric",
        "requested": 1000,
        "replicates": 998,
        "failed": 2,
        "level": 0.95,
        "seed": 42,
        "duration_ms": 512.3,
        "params": [
          {"name": "shape", "mean": 2.51, "lo": 2.3, "hi": 2.75},
          {"name": "scale", "mean": 1.19, "lo": 1.1, "hi": 1.3}
        ],
        "metrics": {
          "aic": {"mean": 1236.1, "lo": 1228.0, "hi": 1245.2},
          "bic": {"mean": 1241.8, "lo": 1233.7, "hi": 1250.9},
          "ks": {"mean": 0.0241, "lo": 0.0188, "hi": 0.0312},
          "rmse": {"mean": 0.0159, "lo": 0.0121, "hi": 0.0204}
        }
      }
    }
  ],
  "failures": []
}
```

## Verwendung in anderen Projekten

Die Beispiele sind geprüft: Das Go-Beispiel wird kompiliert, das TypeScript-Beispiel typgeprüft und der Python-Client läuft in der CI (`consumers/python/test_json_contract.py`).

### Go

Die Pakete `internal/...` lassen sich von anderen Modulen nicht importieren. Typen deshalb im eigenen Projekt definieren:

```go
type FitRow struct {
    Name        string             `json:"name"`
    Params      map[string]float64 `json:"params"`
    AIC         float64            `json:"aic"`
    DeltaAIC    float64            `json:"delta_aic"`
    BIC         float64            `json:"bic"`
    KSStatistic float64            `json:"ks_statistic"`
    RMSE        float64            `json:"rmse"`
    DurationMs  float64            `json:"duration_ms,omitempty"`
    // Optional; nil ohne -bootstrap. Typen siehe unten.
    Bootstrap   *BootstrapResult   `json:"bootstrap,omitempty"`
}

type ParameterCI struct {
    Name string  `json:"name"`
    Mean float64 `json:"mean"`
    Lo   float64 `json:"lo"`
    Hi   float64 `json:"hi"`
}

type MetricCI struct {
    Mean float64 `json:"mean"`
    Lo   float64 `json:"lo"`
    Hi   float64 `json:"hi"`
}

type BootstrapResult struct {
    Method     string                `json:"method"`
    Requested  int                   `json:"requested"`
    Replicates int                   `json:"replicates"`
    Failed     int                   `json:"failed"`
    Level      float64               `json:"level"`
    Seed       uint64                `json:"seed"`
    DurationMs float64               `json:"duration_ms,omitempty"`
    Params     []ParameterCI         `json:"params"`
    Metrics    map[string]MetricCI   `json:"metrics"`
}

type Failure struct {
    Name  string `json:"name"`
    Error string `json:"error"`
}

// Nicht berechenbare Kennzahlen sind null und werden deshalb als Zeiger gelesen.
type Descriptive struct {
    N        int      `json:"n"`
    Mean     *float64 `json:"mean"`
    Median   *float64 `json:"median"`
    Variance *float64 `json:"variance"`
    StdDev   *float64 `json:"std_dev"`
    Min      *float64 `json:"min"`
    Q1       *float64 `json:"q1"`
    Q3       *float64 `json:"q3"`
    Max      *float64 `json:"max"`
    Skewness *float64 `json:"skewness"`
    Kurtosis *float64 `json:"kurtosis"`
}

type FitReport struct {
    SchemaVersion    int         `json:"schema_version"`
    Input            string      `json:"input"`
    Column           string      `json:"column,omitempty"`
    SampleSize       int         `json:"sample_size"`
    Discarded        int         `json:"discarded"`
    Candidates       int         `json:"candidates"`
    BestDistribution string      `json:"best_distribution,omitempty"`
    TotalDurationMs  float64     `json:"total_duration_ms,omitempty"`
    Descriptive      Descriptive `json:"descriptive"`
    Models           []FitRow    `json:"models"`
    Failures         []Failure   `json:"failures"`
}

func main() {
    out, err := exec.Command("distfit", "-file=daten.csv", "-column=density", "-all", "-format=json").Output()
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
    var report FitReport
    if err := json.Unmarshal(out, &report); err != nil {
        panic(err)
    }
    if report.SchemaVersion != 1 {
        panic("unsupported schema_version")
    }
    fmt.Println(report.BestDistribution, report.Models[0].Params)
}
```

### Python

Der Client liegt in [`consumers/python/distfit_client.py`](../consumers/python/distfit_client.py) und wird in der CI getestet. Er ignoriert unbekannte Felder und lehnt unbekannte `schema_version`-Werte ab.

```python
from distfit_client import run_distfit

report = run_distfit("distfit", ["-file=daten.csv", "-column=density", "-all", "-format=json"])

best = report.models[0]
print(report.best_distribution, best.params, best.aic)
```

Ohne den Client, nur mit der Standardbibliothek:

```python
import json
import subprocess

result = subprocess.run(
    ["distfit", "-file=daten.csv", "-column=density", "-all", "-format=json"],
    capture_output=True,
    text=True,
)
if result.returncode != 0:
    raise RuntimeError(result.stderr)

report = json.loads(result.stdout)
assert report["schema_version"] == 1
best_model = report["best_distribution"]
params = report["models"][0]["params"]
```

### TypeScript

```typescript
interface MetricCI {
  mean: number;
  lo: number;
  hi: number;
}

interface BootstrapResult {
  method: "nonparametric" | "parametric";
  requested: number;
  replicates: number;
  failed: number;
  level: number;
  seed: number;
  duration_ms?: number;
  params: { name: string; mean: number; lo: number; hi: number }[];
  metrics: Record<string, MetricCI>; // Schluessel: aic, bic, ks, rmse
}

interface FitRow {
  name: string;
  params: Record<string, number>;
  aic: number;
  delta_aic: number;
  bic: number;
  ks_statistic: number;
  rmse: number;
  duration_ms?: number;
  bootstrap?: BootstrapResult;
}

interface Failure {
  name: string;
  error: string;
}

// Nicht berechenbare Kennzahlen sind null.
interface Descriptive {
  n: number;
  mean: number | null;
  median: number | null;
  variance: number | null;
  std_dev: number | null;
  min: number | null;
  q1: number | null;
  q3: number | null;
  max: number | null;
  skewness: number | null;
  kurtosis: number | null;
}

interface FitReport {
  schema_version: 1;
  input: string;
  column?: string;
  sample_size: number;
  discarded: number;
  candidates: number;
  best_distribution?: string;
  total_duration_ms?: number;
  descriptive: Descriptive;
  models: FitRow[];
  failures: Failure[];
}

const report = JSON.parse(output) as FitReport;
if (report.schema_version !== 1) {
  throw new Error("unsupported schema_version");
}
```
