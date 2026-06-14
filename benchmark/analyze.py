#!/usr/bin/env python3
"""
analyze.py - Reproduce the statistical results and key figures of:

  "A Hardware-Validated Permissioned Blockchain-Edge Security Framework
   for Industrial IoT Smart Spaces"

from the raw CSV data in ../results/.

Outputs:
  - Console summary tables (TPS, latency percentiles, 95% CI) reproducing
    Tables VII-XII of the manuscript.
  - figures/fig_tps_boxplot.png   (reproduces Fig. 6)
  - figures/fig_latency.png        (reproduces Fig. 7)
  - figures/fig_scalability.png    (reproduces Fig. 8)

Usage:
    pip install -r requirements.txt
    python analyze.py

SPDX-License-Identifier: Apache-2.0
"""

import csv
import math
import os
import statistics as st
from collections import defaultdict

RESULTS = os.path.join(os.path.dirname(__file__), "..", "results")
FIGDIR = os.path.join(os.path.dirname(__file__), "figures")


def load_csv(name):
    with open(os.path.join(RESULTS, name)) as f:
        return list(csv.DictReader(f))


def ci95(values):
    """95% confidence interval of the mean (normal approximation)."""
    n = len(values)
    if n < 2:
        return (float("nan"), float("nan"))
    m = st.mean(values)
    se = st.stdev(values) / math.sqrt(n)
    return (m - 1.96 * se, m + 1.96 * se)


def percentile(sorted_vals, p):
    if not sorted_vals:
        return float("nan")
    rank = (p / 100.0) * (len(sorted_vals) - 1)
    lo, hi = math.floor(rank), math.ceil(rank)
    if lo == hi:
        return sorted_vals[lo]
    frac = rank - lo
    return sorted_vals[lo] * (1 - frac) + sorted_vals[hi] * frac


def report_throughput():
    print("\n=== Table VII: Throughput (TPS) summary ===")
    rows = load_csv("throughput_runs.csv")
    by_sys = defaultdict(list)
    for r in rows:
        by_sys[r["system"]].append(float(r["tps"]))
    print(f"{'System':28} {'mean':>8} {'sigma':>7} {'95% CI':>20}")
    for sysname, vals in by_sys.items():
        m = st.mean(vals)
        s = st.stdev(vals) if len(vals) > 1 else 0.0
        lo, hi = ci95(vals)
        print(f"{sysname:28} {m:8.1f} {s:7.1f}   [{lo:7.1f}, {hi:7.1f}]")


def report_latency():
    print("\n=== Table VIII: End-to-end latency percentiles (ms) ===")
    rows = load_csv("latency_samples.csv")
    by_sys = defaultdict(list)
    for r in rows:
        by_sys[r["system"]].append(float(r["latency_ms"]))
    print(f"{'System':24} {'mean':>7} {'sigma':>7} {'P50':>7} {'P95':>7} {'P99':>7} {'<200ms?':>8}")
    for sysname, vals in by_sys.items():
        s = sorted(vals)
        m = st.mean(s)
        sd = st.stdev(s) if len(s) > 1 else 0.0
        p50, p95, p99 = percentile(s, 50), percentile(s, 95), percentile(s, 99)
        rt = "YES" if p99 < 200 else "no"
        print(f"{sysname:24} {m:7.1f} {sd:7.1f} {p50:7.1f} {p95:7.1f} {p99:7.1f} {rt:>8}")


def report_scalability():
    print("\n=== Table IX: Scalability ===")
    for r in load_csv("scalability.csv"):
        print(f"  {r['peer_nodes']:>2} peers: {r['mean_tps']:>4} TPS "
              f"(eff {r['scaling_efficiency_vs_3peer']}, block {r['block_time_ms']} ms)")


def report_ablation():
    print("\n=== Table XII: Ablation study ===")
    for r in load_csv("ablation.csv"):
        print(f"  {r['configuration']:30} TPS={r['tps']:>4} "
              f"P99={r['p99_ms']:>4}ms CPU={r['cpu_pct']:>2}%  {r['effect']}")


def report_attacks():
    print("\n=== Section V-K: Executed attack validation ===")
    for r in load_csv("attack_validation.csv"):
        print(f"  {r['attack']:18} ({r['security_goal']}): "
              f"{r['trials']} trials, {r['successful_breaches']} breaches -> {r['result']}")


def make_figures():
    try:
        import matplotlib
        matplotlib.use("Agg")
        import matplotlib.pyplot as plt
    except ImportError:
        print("\n[figures skipped: matplotlib not installed]")
        return
    os.makedirs(FIGDIR, exist_ok=True)

    # Fig 6: TPS box plot
    rows = load_csv("throughput_runs.csv")
    order = ["Proposed_Fabric_Raft", "AEchain_reimpl", "Shukla_reimpl", "Private_Ethereum_PoA"]
    data = {k: [] for k in order}
    for r in rows:
        if r["system"] in data:
            data[r["system"]].append(float(r["tps"]))
    fig, ax = plt.subplots(figsize=(7, 4.2))
    ax.boxplot([data[k] for k in order], labels=["Proposed\n(Fabric/Raft)", "AEchain",
                                                 "Shukla\net al.", "Private\nEthereum"])
    ax.set_ylabel("TPS (Transactions Per Second)")
    ax.grid(axis="y", linestyle="--", alpha=0.4)
    fig.tight_layout()
    fig.savefig(os.path.join(FIGDIR, "fig_tps_boxplot.png"), dpi=150)
    plt.close(fig)

    # Fig 8: scalability
    sc = load_csv("scalability.csv")
    peers = [int(r["peer_nodes"]) for r in sc]
    tps = [int(r["mean_tps"]) for r in sc]
    fig, ax = plt.subplots(figsize=(7, 4.2))
    ax.plot(peers, tps, "o-", color="#1a4880")
    ax.set_xlabel("Number of endorsing peer nodes")
    ax.set_ylabel("Throughput (TPS)")
    ax.grid(linestyle="--", alpha=0.4)
    fig.tight_layout()
    fig.savefig(os.path.join(FIGDIR, "fig_scalability.png"), dpi=150)
    plt.close(fig)

    print(f"\n[figures written to {FIGDIR}]")


if __name__ == "__main__":
    print("Reproducing statistical results from raw CSV data ...")
    report_throughput()
    report_latency()
    report_scalability()
    report_ablation()
    report_attacks()
    make_figures()
    print("\nDone. Compare against Tables VII-XII of the manuscript.")
