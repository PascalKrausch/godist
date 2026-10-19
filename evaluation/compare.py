
import argparse
import json
import os
import subprocess
import sys
import tempfile

import numpy as np
import pandas as pd
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


def load_data(path, column):
    if path.lower().endswith(".json"):
        df = pd.read_json(path)
    else:
        df = pd.read_csv(
            path,
            sep=";" if path.lower().endswith(".csv") else ",",
            quotechar='"',
            skipinitialspace=True,
        )

    if column.isdigit():
        col = df.columns[int(column)]
    else:
        col = column

    data = pd.to_numeric(df[col], errors="coerce").to_numpy(dtype=float)
    data = data[np.isfinite(data)]

    if len(data) == 0:
        raise ValueError(f"Keine gültigen Daten in Spalte {col!r}")

    return data, str(col)


def build_go_binary(target_dir):
    """Kompiliert die CLI einmalig, damit Laufzeitmessungen keine Build-Zeit enthalten."""
    binary = os.path.join(target_dir, "distfit")
    proc = subprocess.run(
        ["go", "build", "-o", binary, "./cmd/distfit"],
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        raise RuntimeError(f"go build fehlgeschlagen:\n{proc.stderr}")
    return binary


def run_go(binary, path, column):
    cmd = [
        binary,
        "-file", path,
        "-column", column,
        "-all",
        "-format=json",
    ]

    proc = subprocess.run(
        cmd,
        capture_output=True,
        text=True,
        encoding="utf-8",
    )

    if proc.returncode != 0:
        raise RuntimeError(
            f"Go-CLI fehlgeschlagen:\n{proc.stderr or proc.stdout}"
        )

    try:
        return json.loads(proc.stdout)
    except json.JSONDecodeError as exc:
        raise RuntimeError(
            "Die Go-CLI hat keine gültige JSON-Ausgabe geliefert.\n"
            f"stdout:\n{proc.stdout}\nstderr:\n{proc.stderr}"
        ) from exc


def cdf_rmse(data, cdf):
    """RMSE zwischen empirischer und angepasster CDF."""
    x = np.sort(data)
    n = len(x)

    # Mittelpunkte der Sprünge der empirischen CDF
    empirical = (np.arange(n) + 0.5) / n
    theoretical = cdf(x)

    if not np.all(np.isfinite(theoretical)):
        return float("nan")

    return float(np.sqrt(np.mean((empirical - theoretical) ** 2)))


def fit_scipy(name, data):
    n = len(data)

    if name == "Weibull":
        if np.any(data <= 0):
            raise ValueError("Weibull mit loc=0 benötigt positive Daten")
        shape, _, scale = weibull_min.fit(data, floc=0)
        loglik = np.sum(weibull_min.logpdf(data, shape, loc=0, scale=scale))
        cdf = lambda x: weibull_min.cdf(x, shape, loc=0, scale=scale)
        params = {"shape": shape, "scale": scale}
        k = 2

    elif name == "Gamma":
        if np.any(data <= 0):
            raise ValueError("Gamma mit loc=0 benötigt positive Daten")
        alpha, _, scale = gamma.fit(data, floc=0)
        loglik = np.sum(gamma.logpdf(data, alpha, loc=0, scale=scale))
        cdf = lambda x: gamma.cdf(x, alpha, loc=0, scale=scale)
        params = {"alpha": alpha, "scale": scale}
        k = 2

    elif name == "LogNormal":
        if np.any(data <= 0):
            raise ValueError("LogNormal benötigt positive Daten")
        sigma, _, scale = lognorm.fit(data, floc=0)
        mu = np.log(scale)
        loglik = np.sum(lognorm.logpdf(data, sigma, loc=0, scale=scale))
        cdf = lambda x: lognorm.cdf(x, sigma, loc=0, scale=scale)
        params = {"mu": mu, "sigma": sigma}
        k = 2

    elif name == "Normal":
        mu, sigma = norm.fit(data)
        loglik = np.sum(norm.logpdf(data, loc=mu, scale=sigma))
        cdf = lambda x: norm.cdf(x, loc=mu, scale=sigma)
        params = {"mu": mu, "sigma": sigma}
        k = 2

    elif name == "Gumbel":
        mu, scale = gumbel_r.fit(data)
        loglik = np.sum(gumbel_r.logpdf(data, loc=mu, scale=scale))
        cdf = lambda x: gumbel_r.cdf(x, loc=mu, scale=scale)
        params = {"mu": mu, "beta": scale}
        k = 2

    elif name == "Exponential":
        if np.any(data < 0):
            raise ValueError("Exponential mit loc=0 benötigt x >= 0")
        _, scale = expon.fit(data, floc=0)
        loglik = np.sum(expon.logpdf(data, loc=0, scale=scale))
        cdf = lambda x: expon.cdf(x, loc=0, scale=scale)
        params = {"rate": 1.0 / scale}
        k = 1

    elif name == "Laplace":
        # Go-Fitter: mu = Median, scale = mittlere absolute Abweichung vom Median.
        # Das ist genau die MLE, die scipy.stats.laplace.fit ebenfalls liefert.
        mu, scale = laplace.fit(data)
        loglik = np.sum(laplace.logpdf(data, loc=mu, scale=scale))
        cdf = lambda x: laplace.cdf(x, loc=mu, scale=scale)
        params = {"mu": mu, "scale": scale}
        k = 2

    elif name == "Rayleigh":
        if np.any(data <= 0):
            raise ValueError("Rayleigh mit loc=0 benötigt positive Daten")
        _, scale = rayleigh.fit(data, floc=0)
        loglik = np.sum(rayleigh.logpdf(data, loc=0, scale=scale))
        cdf = lambda x: rayleigh.cdf(x, loc=0, scale=scale)
        params = {"scale": scale}
        k = 1

    elif name == "Beta":
        # Dieselbe automatische Randkonvention wie im Go-BetaFitter:
        # 1 % der beobachteten Spannweite als Randzugabe.
        lo = float(np.min(data))
        hi = float(np.max(data))
        margin = 0.01 * (hi - lo)
        if margin == 0:
            margin = 0.001
        min_val = lo - margin
        max_val = hi + margin
        width = max_val - min_val

        # Grenzen fixieren, nur alpha und beta optimieren.
        a, b, _, _ = beta_dist.fit(
            data, floc=min_val, fscale=width
        )
        loglik = np.sum(
            beta_dist.logpdf(data, a, b, loc=min_val, scale=width)
        )
        cdf = lambda x: beta_dist.cdf(
            x, a, b, loc=min_val, scale=width
        )
        params = {
            "alpha": a,
            "beta": b,
            "min": min_val,
            "max": max_val,
        }
        k = 2

    else:
        raise ValueError(f"Unbekannte Verteilung: {name}")

    aic = 2 * k - 2 * loglik
    bic = k * np.log(n) - 2 * loglik

    # KS-Statistik: beide empirische CDF-Sprungseiten berücksichtigen.
    x = np.sort(data)
    fx = cdf(x)
    i = np.arange(1, n + 1)
    d_plus = np.max(i / n - fx)
    d_minus = np.max(fx - (i - 1) / n)
    ks = float(max(d_plus, d_minus))

    return {
        "params": params,
        "loglik": float(loglik),
        "aic": float(aic),
        "bic": float(bic),
        "ks": ks,
        "rmse": cdf_rmse(data, cdf),
        "n": n,
        "k": k,
    }


def main():
    parser = argparse.ArgumentParser(
        description="Vergleicht godist mit SciPy"
    )
    parser.add_argument("--path", required=True)
    parser.add_argument("--column", required=True)
    parser.add_argument(
        "--tol",
        type=float,
        default=1e-3,
        help="maximale Abweichung für AIC und KS, sonst Exit-Code 1",
    )
    args = parser.parse_args()

    data, col_name = load_data(args.path, args.column)
    with tempfile.TemporaryDirectory() as tmp:
        go_result = run_go(build_go_binary(tmp), args.path, args.column)

    go_models = {
        model["name"]: model
        for model in go_result.get("models", [])
    }

    # Go verwendet eventuell andere Schreibweisen.
    aliases = {
        "LogNormal": "LogNormal",
        "Normal": "Normal",
        "Gaussian": "Normal",
    }

    print(f"\nVergleich: {args.path} / {col_name}")
    print(f"Beobachtungen: {len(data)}")
    print()
    print(
        f"{'Verteilung':<13} {'AIC Go':>10} {'AIC SciPy':>10} "
        f"{'Delta':>10} {'KS Go':>9} {'KS SciPy':>10}"
    )
    print("-" * 68)

    mismatches = []
    scipy_results = {}
    for name in [
        "Gumbel", "Beta", "LogNormal", "Gamma",
        "Normal", "Weibull", "Exponential",
        "Laplace", "Rayleigh",
    ]:
        try:
            scipy_results[name] = fit_scipy(name, data)
        except Exception as exc:
            print(f"{name:<13} SciPy-Fit fehlgeschlagen: {exc}")

    for name, sr in scipy_results.items():
        go_name = aliases.get(name, name)
        gr = go_models.get(go_name)

        if gr is None:
            print(f"{name:<13} {'—':>10} {sr['aic']:>10.2f} "
                  f"{'—':>10} {'—':>9} {sr['ks']:>10.4f}")
            mismatches.append(f"{name}: fehlt im Go-Ergebnis")
            continue

        aic_go = gr["aic"]
        ks_go = gr["ks_statistic"]

        if abs(aic_go - sr["aic"]) > args.tol:
            mismatches.append(f"{name}: AIC-Abweichung {aic_go - sr['aic']:.6f}")
        if abs(ks_go - sr["ks"]) > args.tol:
            mismatches.append(f"{name}: KS-Abweichung {ks_go - sr['ks']:.6f}")

        print(
            f"{name:<13} {aic_go:>10.2f} {sr['aic']:>10.2f} "
            f"{sr['aic'] - aic_go:>10.2f} "
            f"{ks_go:>9.4f} {sr['ks']:>10.4f}"
        )

        print(f"  Go:    {gr.get('params', {})}")
        print(f"  SciPy: {sr['params']}")
        print(
            f"  RMSE Go={gr.get('rmse', float('nan')):.5f}; "
            f"SciPy={sr['rmse']:.5f}; "
            f"LogLik SciPy={sr['loglik']:.3f}"
        )

    if go_result.get("best_distribution"):
        print(
            "\nGo-Sieger:",
            go_result["best_distribution"],
        )

    if scipy_results:
        best_scipy = min(
            scipy_results.items(), key=lambda item: item[1]["aic"]
        )
        print("SciPy-Sieger:", best_scipy[0])

    if mismatches:
        print(f"\nAbweichungen über Toleranz {args.tol}:")
        for m in mismatches:
            print(f"  - {m}")
        sys.exit(1)
    print(f"\nAlle Modelle innerhalb Toleranz {args.tol}.")


if __name__ == "__main__":
    main()
