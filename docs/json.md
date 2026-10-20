# JSON-Ausgabe

JSON wird im aktuellen Prototyp nur mit `-all -format=json` ausgegeben. Der nächste Schritt ist, das JSON-Format als stabile Schnittstelle zu definieren und zu testen. Es soll mindestens Eingabemetadaten, Stichprobengröße, Auswahlkriterium, erfolgreiche Modelle, benannte Parameter, Metriken und fehlgeschlagene Fits enthalten.

Beispiel für die geplante Verwendung:

```bash
godist fit measurements.csv --column wind_speed_10m --format json
```

Bei JSON-Ausgabe gehört ausschließlich das JSON-Dokument auf `stdout`; Statusmeldungen und Fehler gehören auf `stderr`. So lässt sich die Ausgabe zuverlässig an andere Werkzeuge weiterleiten.

## Beispiel

```json
{
  "input": "daten.csv",
  "column": "density",
  "sample_size": 1000,
  "discarded": 5,
  "candidates": 9,
  "best_distribution": "Weibull",
  "models": [
    {
      "name": "Weibull",
      "params": {"k": 2.5, "lambda": 1.2},
      "aic": 1234.5,
      "delta_aic": 0.0,
      "bic": 1240.2,
      "ks_statistic": 0.0234,
      "rmse": 0.0156
    }
  ],
  "failures": []
}
```

## Verwendung in anderen Projekten

### Go

```go
type FitReport struct {
    Input            string                    `json:"input"`
    Column           string                    `json:"column,omitempty"`
    SampleSize       int                       `json:"sample_size"`
    Discarded        int                       `json:"discarded"`
    Candidates       int                       `json:"candidates"`
    BestDistribution string                    `json:"best_distribution,omitempty"`
    Models           []FitRow                  `json:"models"`
    Failures         []distribution.FitFailure `json:"failures,omitempty"`
}

var report FitReport
json.Unmarshal(output, &report)
```

### Python

```python
from dataclasses import dataclass
from typing import Dict, List
import json

@dataclass
class FitRow:
    name: str
    params: Dict[str, float]
    aic: float
    delta_aic: float
    bic: float
    ks_statistic: float
    rmse: float

@dataclass
class FitReport:
    input: str
    column: str
    sample_size: int
    discarded: int
    candidates: int
    best_distribution: str
    models: List[FitRow]
    failures: List[dict]

report = FitReport(**json.loads(result.stdout))

result = subprocess.run(
    ["distfit", "-file=daten.csv", "-all", "-format=json"],
    capture_output=True,
    text=True
)
report = json.loads(result.stdout)

best_model = report["best_distribution"]
params = report["models"][0]["params"]
aic = report["models"][0]["aic"]
```

### TypeScript

```typescript
interface FitRow {
  name: string;
  params: Record<string, number>;
  aic: number;
  delta_aic: number;
  bic: number;
  ks_statistic: number;
  rmse: number;
}

interface FitReport {
  input: string;
  column?: string;
  sample_size: number;
  discarded: number;
  candidates: number;
  best_distribution?: string;
  models: FitRow[];
  failures?: any[];
}

const report: FitReport = JSON.parse(output);
```
