#!/usr/bin/env python3
"""Misst Gesamtlaufzeit, reine Algorithmuszeit und Speicherbedarf von godist gegen SciPy.

Aufruf aus dem Repository-Wurzelverzeichnis:
    python evaluation/bench.py run [--repeat 10]

Ergebnisse landen in evaluation/bench_results/<Zeitstempel>/.
Die Daten entstehen aus demselben SplitMix64-Generator wie in
pkg/distribution/bench_test.go (benchData), damit beide Seiten identische Eingaben fitten.
"""
import argparse
import datetime as dt
import json
import os
import platform
import statistics
import subprocess
import sys
import tempfile
import time
import timeit
import tracemalloc
from pathlib import Path

import numpy as np
import pandas as pd
import scipy
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

from compare import fit_scipy, load_data

REPO = Path(__file__).resolve().parents[1]
SIZES = [100, 10_000, 1_000_000]
SEED = 42
GOLDEN_GAMMA = 0x9E3779B97F4A7C15
NAMES = [
    "Beta", "Exponential", "Normal", "Gumbel", "Gamma",
    "LogNormal", "Weibull", "Laplace", "Rayleigh",
]


def gen_data(n):
    """Exponentialverteilte Daten: SplitMix64 mit Seed 42, Inversionsmethode -ln(U)."""
    with np.errstate(over="ignore"):
        idx = np.arange(1, n + 1, dtype=np.uint64)
        z = np.uint64(SEED) + idx * np.uint64(GOLDEN_GAMMA)
        z = (z ^ (z >> np.uint64(30))) * np.uint64(0xBF58476D1CE4E5B9)
        z = (z ^ (z >> np.uint64(27))) * np.uint64(0x94D049BB133111EB)
        z = z ^ (z >> np.uint64(31))
    u = ((z >> np.uint64(11)).astype(np.float64) + 0.5) / 2.0 ** 53
    return -np.log(u)


def _beta_fit(data):
    # Gleiche Randkonvention wie der Go-BetaFitter: 1 % der Spannweite als Zugabe.
    lo, hi = float(np.min(data)), float(np.max(data))
    margin = 0.01 * (hi - lo)
    if margin == 0:
        margin = 0.001
    min_val = lo - margin
    width = (hi + margin) - min_val
    return beta_dist.fit(data, floc=min_val, fscale=width)


# Reine Fit-Aufrufe, identisch zu fit_scipy in compare.py (ohne Metrikberechnung).
FIT_CALLS = {
    "Beta": _beta_fit,
    "Exponential": lambda d: expon.fit(d, floc=0),
    "Normal": lambda d: norm.fit(d),
    "Gumbel": lambda d: gumbel_r.fit(d),
    "Gamma": lambda d: gamma.fit(d, floc=0),
    "LogNormal": lambda d: lognorm.fit(d, floc=0),
    "Weibull": lambda d: weibull_min.fit(d, floc=0),
    "Laplace": lambda d: laplace.fit(d),
    "Rayleigh": lambda d: rayleigh.fit(d, floc=0),
}


def time_per_call(fn, repeat):
    fn()  # Warmup
    timer = timeit.Timer(fn)
    number, _ = timer.autorange()
    totals = timer.repeat(repeat=repeat, number=number)
    per_call = [t / number for t in totals]
    return {
        "median_s": statistics.median(per_call),
        "min_s": min(per_call),
        "number": number,
        "repeat": repeat,
    }


def bench_algo(repeat):
    results = []
    for n in SIZES:
        data = gen_data(n)
        for name in NAMES:
            stats = time_per_call(lambda: FIT_CALLS[name](data), repeat)
            results.append({"name": name, "n": n, **stats})
            print(f"algo  {name:<12} n={n:<9} median={stats['median_s']:.6f}s", flush=True)
        # Entspricht SelectAllModels in Go: Fit plus Gütemetriken für alle Modelle.
        stats = time_per_call(lambda: [fit_scipy(name, data) for name in NAMES], repeat)
        results.append({"name": "AllModels", "n": n, **stats})
        print(f"algo  {'AllModels':<12} n={n:<9} median={stats['median_s']:.6f}s", flush=True)
    return results


def bench_mem():
    results = []
    for n in SIZES:
        data = gen_data(n)
        for name in NAMES:
            tracemalloc.start()
            FIT_CALLS[name](data)
            _, peak = tracemalloc.get_traced_memory()
            tracemalloc.stop()
            results.append({"name": name, "n": n, "peak_bytes": peak})
    return results


def _measure(cmd):
    """Liefert Wandzeit und Peak-RSS des Kindprozesses; wait4 liefert die Ressourcen pro Kind."""
    with tempfile.TemporaryFile() as out:
        start = time.perf_counter()
        proc = subprocess.Popen(cmd, stdout=out, stderr=subprocess.DEVNULL)
        _, status, usage = os.wait4(proc.pid, 0)
        elapsed = time.perf_counter() - start
    proc.returncode = os.waitstatus_to_exitcode(status)
    if proc.returncode != 0:
        raise RuntimeError(f"{cmd[0]} endete mit Code {proc.returncode}")
    # ru_maxrss: Bytes auf macOS, Kilobytes auf Linux.
    rss_bytes = usage.ru_maxrss if sys.platform == "darwin" else usage.ru_maxrss * 1024
    return elapsed, rss_bytes


def bench_e2e(repeat, go_binary, tmp):
    results = []
    for n in SIZES:
        csv_path = Path(tmp) / f"data_{n}.csv"
        pd.DataFrame({"x": gen_data(n)}).to_csv(csv_path, sep=";", index=False)
        commands = {
            "go": [str(go_binary), "-file", str(csv_path), "-column", "x", "-all", "-format=json"],
            "python": [sys.executable, str(Path(__file__).resolve()), "child",
                       "--file", str(csv_path), "--column", "x"],
        }
        for impl, cmd in commands.items():
            _measure(cmd)  # Warmup, nicht gewertet
            samples = [_measure(cmd) for _ in range(repeat)]
            times = [s[0] for s in samples]
            rss = [s[1] for s in samples]
            results.append({
                "impl": impl,
                "n": n,
                "median_s": statistics.median(times),
                "min_s": min(times),
                "peak_rss_bytes_median": statistics.median(rss),
                "repeat": repeat,
            })
            print(f"e2e   {impl:<7} n={n:<9} median={statistics.median(times):.3f}s "
                  f"rss={statistics.median(rss) / 2**20:.1f} MiB", flush=True)
    return results


def child(path, column):
    """Python-Gegenstück zur Go-CLI: Einlesen, alle Modelle fitten, Metriken berechnen."""
    data, _ = load_data(path, column)
    for name in NAMES:
        fit_scipy(name, data)


def environment():
    def run(cmd):
        try:
            return subprocess.run(cmd, cwd=REPO, capture_output=True, text=True, check=True).stdout.strip()
        except (OSError, subprocess.CalledProcessError):
            return None

    cpu = run(["sysctl", "-n", "machdep.cpu.brand_string"]) if sys.platform == "darwin" else platform.processor()
    dirty = run(["git", "status", "--porcelain"])
    return {
        "timestamp": dt.datetime.now().isoformat(timespec="seconds"),
        "platform": platform.platform(),
        "cpu": cpu,
        "cpu_count": os.cpu_count(),
        "python": sys.version,
        "numpy": np.__version__,
        "pandas": pd.__version__,
        "scipy": scipy.__version__,
        "go": run(["go", "version"]),
        "git_commit": run(["git", "rev-parse", "HEAD"]),
        "git_dirty": bool(dirty),
    }


def run_all(repeat):
    out_dir = REPO / "evaluation" / "bench_results" / dt.datetime.now().strftime("%Y%m%d-%H%M%S")
    out_dir.mkdir(parents=True)

    def save(name, obj):
        (out_dir / name).write_text(json.dumps(obj, indent=2, ensure_ascii=False), encoding="utf-8")

    save("environment.json", environment())

    print("Go-Benchmarks (go test -bench) ...", flush=True)
    go_bench = subprocess.run(
        ["go", "test", "./pkg/distribution", "-run", "^$", "-bench", ".", "-benchmem",
         "-benchtime=3s", f"-count={repeat}"],
        cwd=REPO, capture_output=True, text=True,
    )
    if go_bench.returncode != 0:
        raise RuntimeError(f"go test fehlgeschlagen:\n{go_bench.stderr or go_bench.stdout}")
    (out_dir / "go-bench.txt").write_text(go_bench.stdout, encoding="utf-8")

    print("Python-Algorithmuszeit ...", flush=True)
    save("python-algo.json", bench_algo(repeat))

    print("Python-Speicher (tracemalloc) ...", flush=True)
    save("python-mem.json", bench_mem())

    with tempfile.TemporaryDirectory() as tmp:
        go_binary = Path(tmp) / "distfit"
        build = subprocess.run(
            ["go", "build", "-o", str(go_binary), "./cmd/distfit"],
            cwd=REPO, capture_output=True, text=True,
        )
        if build.returncode != 0:
            raise RuntimeError(f"go build fehlgeschlagen:\n{build.stderr}")
        print("Gesamtlaufzeit (Prozess) ...", flush=True)
        save("e2e.json", bench_e2e(repeat, go_binary, tmp))

    print(f"\nErgebnisse: {out_dir}")


def main():
    parser = argparse.ArgumentParser(description="Benchmark godist vs. SciPy")
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_run = sub.add_parser("run", help="alle Messungen ausführen")
    p_run.add_argument("--repeat", type=int, default=10)

    p_child = sub.add_parser("child", help="intern: Python-Seite der Gesamtlaufzeitmessung")
    p_child.add_argument("--file", required=True)
    p_child.add_argument("--column", required=True)

    args = parser.parse_args()
    if args.cmd == "run":
        run_all(args.repeat)
    else:
        child(args.file, args.column)


if __name__ == "__main__":
    main()
