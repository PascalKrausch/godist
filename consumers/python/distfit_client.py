"""Leser für die JSON-Ausgabe von distfit (Vertrag: docs/json.schema.json).

Unbekannte Felder werden ignoriert, damit additive Erweiterungen des Vertrags
bestehende Consumer nicht brechen.
"""

from __future__ import annotations

import json
import subprocess
from dataclasses import dataclass, fields
from typing import Dict, List, Optional

SUPPORTED_SCHEMA_VERSION = 1


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
class Failure:
    name: str
    error: str


@dataclass
class FitReport:
    schema_version: int
    input: str
    sample_size: int
    discarded: int
    candidates: int
    descriptive: Dict[str, Optional[float]]
    models: List[FitRow]
    failures: List[Failure]
    column: Optional[str] = None
    best_distribution: Optional[str] = None

    @classmethod
    def from_json(cls, text: str) -> "FitReport":
        data = json.loads(text)
        version = data["schema_version"]
        if version != SUPPORTED_SCHEMA_VERSION:
            raise ValueError(f"unsupported schema_version {version}, expected {SUPPORTED_SCHEMA_VERSION}")
        return cls(
            schema_version=version,
            input=data["input"],
            sample_size=data["sample_size"],
            discarded=data["discarded"],
            candidates=data["candidates"],
            descriptive=data["descriptive"],
            models=[_pick(FitRow, m) for m in data["models"]],
            failures=[_pick(Failure, f) for f in data["failures"]],
            column=data.get("column"),
            best_distribution=data.get("best_distribution"),
        )


def run_distfit(binary: str, args: List[str]) -> FitReport:
    """Startet distfit und parst stdout. Bei Fehlern wird eine RuntimeError mit stderr ausgelöst."""
    result = subprocess.run([binary, *args], capture_output=True, text=True)
    if result.returncode != 0:
        raise RuntimeError(f"distfit exit {result.returncode}: {result.stderr.strip()}")
    return FitReport.from_json(result.stdout)


def _pick(cls, data: dict):
    known = {f.name for f in fields(cls)}
    return cls(**{key: value for key, value in data.items() if key in known})
