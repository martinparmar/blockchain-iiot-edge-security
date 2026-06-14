# Results — Data Dictionary

Raw experimental data supporting the paper. Each CSV maps to specific tables/
figures in the manuscript.

| File | Manuscript reference | Columns |
|---|---|---|
| `throughput_runs.csv` | Table VII, Fig. 6 | system, run, tps |
| `latency_samples.csv` | Table VIII, Fig. 7 | system, sample_idx, latency_ms |
| `scalability.csv` | Table IX, Fig. 8 | peer_nodes, mean_tps, sigma, scaling_efficiency_vs_3peer, block_time_ms |
| `aes_overhead.csv` | Table X, Fig. 9 | device, aes_mode, payload_kb, enc_ms, dec_ms, energy_uj, overhead |
| `storage.csv` | Table XI, Fig. 11 | devices, full_payload_mb_day, hash_only_kb_day, saving, annual_hash_mb, annual_full_gb |
| `ablation.csv` | Table XII | configuration, tps, p99_ms, cpu_pct, effect |
| `attack_validation.csv` | Section V-K | attack, security_goal, trials, successful_breaches, result |

## Regenerating aggregate statistics

```bash
cd ../benchmark
python analyze.py
```

This reproduces the mean, standard deviation, 95% confidence intervals, and
P50/P95/P99 percentiles reported in the manuscript from these raw files.

## Provenance

- `throughput_runs.csv` and `latency_samples.csv` were collected over a
  10-minute steady-state window after a 2-minute warm-up, using the
  nanosecond-timestamp instrumentation in `benchmark/bench.go` (Section V-A).
- `aes_overhead.csv` energy figures were measured with an INA219 current sensor
  sampled at 1 kHz.
- `attack_validation.csv` records the three executed attacks (FDIA, masquerading,
  orderer-crash) described in Section V-K.
