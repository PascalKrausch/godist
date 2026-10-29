#!/usr/bin/env python3
"""Vergleicht die Diagrammdaten des Dashboards (Go) mit SciPy.

Die Modellparameter stammen aus der Go-Payload. Verglichen wird daher nur die Plot-Logik
(Histogramm, Dichte, ECDF, PDF und CDF auf dem gemeinsamen Raster), nicht die Anpassung.
Die Anpassung selbst prüft compare.py.

Aufruf aus dem Repository-Wurzelverzeichnis:
    python evaluation/compare_plot.py --path testdata/winequality-red.csv --column alcohol
"""
import argparse
import json
import subprocess
import sys
from pathlib import Path

import numpy as np
from scipy.stats import (
    beta as beta_dist,
    expon,
    gamma,
    gumbel_r,
    laplace,
    lognorm,
    norm,
    rayleigh,
    weibull_min,
)

from compare import load_data

REPO = Path(__file__).resolve().parents[1]


def frozen_from_params(name, params):
    """Baut die SciPy-Verteilung mit den Parametern aus der Go-Payload (gleiche Namen wie in internal/plot)."""
    p = {item["name"]: item["value"] for item in params}
    if name == "Weibull":
        return weibull_min(p["shape"], loc=0, scale=p["scale"])
    if name == "Gamma":
        return gamma(p["alpha"], loc=0, scale=p["scale"])
    if name == "LogNormal":
        return lognorm(p["sigma"], loc=0, scale=np.exp(p["mu"]))
    if name == "Normal":
        return norm(loc=p["mu"], scale=p["sigma"])
    if name == "Gumbel":
        return gumbel_r(loc=p["mu"], scale=p["beta"])
    if name == "Exponential":
        return expon(loc=0, scale=1.0 / p["rate"])
    if name == "Laplace":
        return laplace(loc=p["mu"], scale=p["scale"])
    if name == "Rayleigh":
        return rayleigh(loc=0, scale=p["scale"])
    if name == "Beta":
        lo, hi = p["min"], p["max"]
        return beta_dist(p["alpha"], p["beta"], loc=lo, scale=hi - lo)
    raise ValueError(f"Unbekanntes Modell in Payload: {name}")


def run_go_payload(path, column, bins):
    cmd = ["go", "run", "./evaluation/payloaddump", "-file", path, "-column", column, "-bins", str(bins)]
    proc = subprocess.run(cmd, cwd=REPO, capture_output=True, text=True, encoding="utf-8")
    if proc.returncode != 0:
        raise RuntimeError(f"payloaddump fehlgeschlagen:\n{proc.stderr or proc.stdout}")
    return json.loads(proc.stdout)


def compare_series(go_values, sci_values):
    """Maximale absolute Abweichung über Punkte, die in Go gültig sind. Null in Go zählt als ungültig."""
    go = np.array([np.nan if v is None else v for v in go_values], dtype=float)
    valid = np.isfinite(go)
    sci = np.asarray(sci_values, dtype=float)
    finite_mismatch = int(np.sum(valid != np.isfinite(sci)))
    if not np.any(valid):
        return 0.0, int(np.sum(~valid)), finite_mismatch
    diff = np.max(np.abs(go[valid] - sci[valid]))
    return float(diff), int(np.sum(~valid)), finite_mismatch


def main():
    parser = argparse.ArgumentParser(description="Vergleicht Dashboard-Diagrammdaten mit SciPy")
    parser.add_argument("--path", required=True)
    parser.add_argument("--column", required=True)
    parser.add_argument("--bins", type=int, default=0, help="Klassenanzahl (0 = automatisch wie im Dashboard)")
    parser.add_argument("--tol", type=float, default=1e-9, help="maximale absolute Abweichung für Dichte, CDF und ECDF")
    args = parser.parse_args()

    data, col_name = load_data(args.path, args.column)
    payload = run_go_payload(args.path, args.column, args.bins)

    mismatches = []
    n = len(data)
    if payload["sample_size"] != n:
        mismatches.append(f"sample_size: Go={payload['sample_size']} SciPy={n}")

    hist = payload["histogram"]
    edges = np.asarray(hist["edges"], dtype=float)
    sci_counts, _ = np.histogram(data, bins=edges)
    go_counts = np.asarray(hist["counts"])
    count_mismatch = int(np.sum(go_counts != sci_counts))
    if count_mismatch:
        mismatches.append(f"Histogramm: {count_mismatch} Klassen mit abweichender Anzahl")
    widths = np.diff(edges)
    sci_density = sci_counts / (n * widths)
    density_diff = float(np.max(np.abs(np.asarray(hist["density"]) - sci_density)))
    if density_diff > args.tol:
        mismatches.append(f"Histogramm-Dichte: Abweichung {density_diff:.3g}")

    grid = np.asarray(payload["grid"], dtype=float)
    if np.any(np.diff(grid) < 0):
        mismatches.append("Raster ist nicht sortiert")
    sci_ecdf = np.searchsorted(np.sort(data), grid, side="right") / n
    ecdf_diff = float(np.max(np.abs(np.asarray(payload["ecdf"]) - sci_ecdf)))
    if ecdf_diff > args.tol:
        mismatches.append(f"ECDF: Abweichung {ecdf_diff:.3g}")

    print(f"\nPlot-Vergleich: {args.path} / {col_name}")
    print(f"Beobachtungen: {n}, Klassen: {len(go_counts)}, Rasterpunkte: {len(grid)}")
    print(f"Histogramm: Klassenanzahl abweichend {count_mismatch}, max. Dichte-Abweichung {density_diff:.3g}")
    print(f"ECDF:       max. Abweichung {ecdf_diff:.3g}")
    print()
    print(f"{'Verteilung':<13} {'max |Δ PDF|':>13} {'max |Δ CDF|':>13} {'null (Go)':>10} {'Finit-Diff.':>12}")
    print("-" * 66)

    for model in payload["models"]:
        name = model["name"]
        dist = frozen_from_params(name, model["params"])
        sci_pdf = dist.pdf(grid)
        sci_cdf = dist.cdf(grid)
        pdf_diff, pdf_null, pdf_finite = compare_series(model["pdf"], sci_pdf)
        cdf_diff, cdf_null, cdf_finite = compare_series(model["cdf"], sci_cdf)
        print(f"{name:<13} {pdf_diff:>13.3g} {cdf_diff:>13.3g} {pdf_null:>10} {pdf_finite + cdf_finite:>12}")

        if pdf_diff > args.tol:
            mismatches.append(f"{name}: PDF-Abweichung {pdf_diff:.3g}")
        if cdf_diff > args.tol:
            mismatches.append(f"{name}: CDF-Abweichung {cdf_diff:.3g}")
        if pdf_finite or cdf_finite:
            mismatches.append(f"{name}: unterschiedliche Endlichkeit bei PDF/CDF")

    if mismatches:
        print(f"\nAbweichungen über Toleranz {args.tol}:")
        for m in mismatches:
            print(f"  - {m}")
        sys.exit(1)
    print(f"\nAlle Plot-Daten innerhalb Toleranz {args.tol}.")


if __name__ == "__main__":
    main()
