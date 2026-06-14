# Reproduction Guide

This document describes how to reproduce each experimental result in the paper.

## Environment

| Component | Version |
|---|---|
| Hyperledger Fabric | v2.4.9 |
| Fabric Go SDK / Gateway | v1.0.0 / v1.4.0 |
| Go | 1.20+ |
| OpenSSL | 3.0.9 (with AES-NI) |
| Raspberry Pi OS | 12 (64-bit) |
| CouchDB | as bundled with Fabric 2.4.9 |

## Hardware testbed

- 3 × Raspberry Pi 4 Model B (quad-core Cortex-A72 @ 1.5 GHz, 4 GB) — Fabric peers/gateways
- 1 × NVIDIA Jetson Orin (octa-core @ 2.2 GHz, 32 GB) — Fabric CA, Raft ordering, CouchDB
- 6 × ESP32-WROOM-32 — sensor endpoints
- 1 Gbps managed switch — wired inter-peer backbone
- 802.11n / WPA2-Enterprise — device-to-gateway link

## Experiment map

### Throughput (Table VII, Fig. 6) — Section V-B/C
```bash
cd benchmark
go run bench.go --mode closed --clients 10 --tx 1000 --warmup 120
python analyze.py    # prints mean, sigma, 95% CI
```

### End-to-end latency (Table VIII, Fig. 7) — Section V-D
Latency per transaction is recorded by `bench.go` and aggregated by
`analyze.py` into P50/P95/P99. The proposed system should show P99 < 200 ms.

### Scalability (Table IX, Fig. 8) — Section V-E
Vary the number of endorsing peers in `deploy/network.sh` (2–10) and re-run the
closed-loop benchmark for each configuration.

### Tiered AES overhead (Table X, Fig. 9) — Section V-F
- Raspberry Pi 4: benchmark AES-128 vs AES-256 (CBC/GCM) via OpenSSL `speed`.
- ESP32: the firmware in `iot-device/esp32_sensor.ino` prints per-block hardware
  AES timing over serial; compare against a software build to obtain the 14.4×
  speedup.

### Edge resource utilization (Fig. 10) — Section V-G
Monitor Pi 4 CPU/RAM with `mpstat`/`free` during the 500 TPS run.

### On-chain storage (Table XI, Fig. 11) — Section V-H
`results/storage.csv` projects ledger growth for hash-only vs full-payload
storage across fleet sizes; the 98.6% reduction is constant.

### Ablation (Table XII) — Section V-I
Each row corresponds to a one-component change from the full framework:
- Uniform AES-256: set all zones to AES-256 in the gateway pipeline.
- Full-payload on-chain: store the payload instead of its hash in `LogData()`.
- Cloud-first: disable edge-local processing, forward raw to cloud.
- No gateway co-endorsement: change the endorsement policy to a single peer.
- Block interval 0.5 s: set `BatchTimeout: 0.5s` in the orderer config.

### Security validation — executed attacks (Section V-K)
1. **FDIA**: submit a `LogData()` transaction without the home-gateway
   endorsement → rejected at validation. 200 attempts, 0 commits.
2. **Masquerading**: replay a captured frame with a forged DID in the AES-GCM
   associated-data field → authentication tag fails, packet dropped. 500 trials.
3. **Orderer crash / DoS**: kill 2 of 5 Raft orderers under load → leader
   re-elected in ~1.8 s, 0 data loss, throughput recovers to ~480 TPS.

See `results/attack_validation.csv` for the recorded outcomes.
