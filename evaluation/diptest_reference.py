"""Erzeugt Referenzwerte für Hartigans Dip-Test.

Verwendet das Paket `diptest` (Python-Port der R-Referenzimplementierung
auf Basis von AS 217, Hartigan & Hartigan 1985). Der p-Wert wird per
Bootstrap gegen die Uniform(0,1)-Nullhypothese berechnet (`boot_pval=True`),
analog zum Monte-Carlo-Vorgehen in Go.

Ausgabe: evaluation/diptest_fixtures.json, das vom Go-Test
pkg/distribution/hartigans_dip_reference_test.go ausgewertet wird.

Aufruf aus dem Repository-Wurzelverzeichnis:

    evaluation/.venv/bin/python evaluation/diptest_reference.py
"""

import json
import os

import numpy as np
import pandas as pd
import diptest

OUT_PATH = os.path.join(os.path.dirname(__file__), "diptest_fixtures.json")
N_BOOT = 10000
SEED = 12345


def reference(x):
    """Liefert (dip, pvalue) aus der Referenzimplementierung."""
    x = np.asarray(x, dtype=float)
    x = x[np.isfinite(x)]
    dip = float(diptest.dipstat(x))
    _, pval = diptest.diptest(
        x, boot_pval=True, n_boot=N_BOOT, seed=SEED, sort_x=False
    )
    return dip, float(pval)


def main():
    rng = np.random.default_rng(42)
    cases = {}

    def add(name, x, dip_tol=None):
        x = np.sort(np.asarray(x, dtype=float))
        dip, pval = reference(x)
        case = {
            "data": x.tolist(),
            "dip": dip,
            "pvalue": pval,
        }
        if dip_tol is not None:
            case["dip_tol"] = dip_tol
        cases[name] = case
        print(f"{name}: n={len(x)} dip={dip:.6f} p={pval:.4f}")

    add("normal_n100", rng.normal(0, 1, 100))
    add("normal_n500", rng.normal(5, 2, 500))
    add("exponential_n300", rng.exponential(2.0, 300))
    add("uniform_n150", rng.uniform(-1, 1, 150))
    add("lognormal_n400", rng.lognormal(0, 0.8, 400))

    # Bimodale Mischungen mit unterschiedlicher Separation
    n = 200
    modes = np.where(np.arange(n) % 2 == 0, -2.0, 2.0) + 0.5 * rng.standard_normal(n)
    add("bimodal_separated_n200", modes)
    modes_close = np.where(np.arange(n) % 2 == 0, -0.4, 0.4) + 0.8 * rng.standard_normal(n)
    add("bimodal_close_n200", modes_close)

    # Trimodale Mischung
    n = 300
    modes3 = np.where(np.arange(n) % 3 == 0, -3.0, np.where(np.arange(n) % 3 == 1, 0.0, 3.0))
    modes3 = modes3 + 0.7 * rng.standard_normal(n)
    add("trimodal_n300", modes3)

    # Schwere Tails (Cauchy)
    add("cauchy_n300", rng.standard_cauchy(300))

    # Konstante mit einem einzelnen Ausreißer
    add("constant_plus_outlier_n60", np.concatenate([np.full(59, 1.0), [5.0]]))

    # Große Stichproben
    add("large_normal_n5000", rng.normal(0, 1, 5000))
    n = 5000
    modes_big = np.where(np.arange(n) % 2 == 0, -1.5, 1.5) + 0.6 * rng.standard_normal(n)
    add("large_bimodal_n5000", modes_big)

    # Ties durch Rundung (wie reale Messdaten)
    add("ties_rounded_n100", np.round(rng.normal(0, 1, 100), 1))
    add("heavy_ties_n80", np.round(rng.exponential(1, 80), 0))

    # Kleine Stichproben
    add("small_n10", rng.normal(0, 1, 10))
    add("small_n30", rng.normal(0, 1, 30))

    # Sehr kleine Stichproben (Grenzfälle der Indexarithmetik, n >= 4)
    add("tiny_n4", rng.normal(0, 1, 4))
    add("tiny_n5", rng.normal(0, 1, 5))
    add("tiny_n7", rng.normal(0, 1, 7))
    add("tiny_n9", rng.normal(0, 1, 9))

    # Zero-inflated: Punktmasse bei 0 plus Gamma für positive Werte.
    # Der Dip-Test läuft auf den rohen Werten inklusive Nullen.
    n = 300
    zi = np.where(rng.random(n) < 0.4, 0.0, rng.gamma(2.0, 1.0, n))
    add("zero_inflated_n300", zi)

    # Reale Daten: winequality-dataset (Spalte "density" wie in compare.py)
    wine_path = os.path.join("testdata", "winequality-red.csv")
    if os.path.exists(wine_path):
        df = pd.read_csv(wine_path, sep=";", quotechar='"', skipinitialspace=True)
        wine = pd.to_numeric(df["density"], errors="coerce").to_numpy(dtype=float)
        wine = wine[np.isfinite(wine)]
        # Stichprobe von 500 Werten, damit der Bootstrap schnell bleibt
        add("winequality_density_n500", wine[:500])

    with open(OUT_PATH, "w", encoding="utf-8") as f:
        json.dump(cases, f, indent=1)
    print(f"\nFixtures geschrieben nach {OUT_PATH}")


if __name__ == "__main__":
    main()
