# Blockchain-Edge Security Framework for Industrial IoT Smart Spaces

This repository contains the chaincode, benchmark harness, edge/device reference
implementations, and raw experimental data supporting the paper:

> **A Hardware-Validated Permissioned Blockchain-Edge Security Framework for
> Industrial IoT Smart Spaces**
> M. Parmar, H. Khatusuriya, D. Chauhan, M. Rahevar, B. Patel, and H. Mewada
The framework is a four-tier architecture integrating Hyperledger Fabric v2.4.9
(Raft ordering), a tiered AES-128/AES-256 cryptographic strategy, and DID/OAuth2
smart-contract access control, validated on Raspberry Pi 4 and NVIDIA Jetson Orin
edge hardware.

## Headline results (reproducible from `results/`)

| Metric | Value | Source |
|---|---|---|
| Throughput | 500 ± 23 TPS | `results/throughput_runs.csv` |
| End-to-end latency | 110 ± 18 ms (P50), 167 ms (P99) | `results/latency_samples.csv` |
| Latency reduction vs cloud | 86.6% | `results/latency_samples.csv` |
| Crypto overhead reduction | 15–20% vs uniform AES-256 | `results/aes_overhead.csv` |
| On-chain storage reduction | 98.6% | `results/storage.csv` |
| ESP32 HW AES speedup | 14.4× | `results/aes_overhead.csv` |

## Repository structure

```
.
├── chaincode/smartspace/      Hyperledger Fabric chaincode (Go)
│   ├── smartspace.go          RegisterDevice(), AuthorizeUser(), LogData()
│   ├── certutil.go            X.509 / ECDSA-P256 certificate verification
│   └── smartspace_test.go     Unit tests
├── benchmark/                 Throughput / latency measurement harness
│   ├── bench.go               Closed-loop & open-loop Fabric Gateway client
│   ├── analyze.py             Reproduces Tables VII–XII and Figs 6–8 from CSVs
│   └── figures/               Regenerated figures (created by analyze.py)
├── edge-gateway/              Tier 2 Raspberry Pi gateway pipeline (Python)
│   └── gateway_pipeline.py    Decrypt→DID verify→hash→sign→re-encrypt path
├── iot-device/                Tier 1 ESP32 firmware (Arduino/ESP-IDF)
│   └── esp32_sensor.ino       Hardware AES-128-CBC + MQTT/WPA2 transmit
├── deploy/                    Fabric test-network deployment scripts
│   ├── network.sh             Bring up channel + deploy chaincode
│   └── endorsement-policy.json Gateway co-endorsement policy (FDIA defense)
├── results/                   Raw experimental data (CSV)
├── docs/                      Architecture figures and reproduction notes
├── CITATION.cff               Citation metadata
├── LICENSE                    Apache-2.0
└── .zenodo.json               Zenodo archival metadata (DOI minting)
```

## Quick start

### 1. Reproduce the statistics and figures (no Fabric required)

```bash
cd benchmark
pip install -r requirements.txt
python analyze.py
```

This reads the raw CSVs in `results/` and prints the throughput, latency-percentile,
scalability, ablation, and attack-validation tables, then regenerates the box-plot
and scalability figures into `benchmark/figures/`.

### 2. Deploy the chaincode on a Fabric test network

Prerequisites: Docker, Go 1.20+, and the
[Hyperledger Fabric v2.4.9 samples / binaries](https://hyperledger-fabric.readthedocs.io/en/release-2.4/install.html).

```bash
cd deploy
./network.sh up            # start orderer + 3 peers (Raft)
./network.sh deploy        # package & commit the smartspace chaincode
```

### 3. Run the benchmark harness

Configure your MSP credentials (see `benchmark/README.md`), then:

```bash
cd benchmark
# closed-loop microbenchmark (Section V-B): 10 clients × 1000 tx
go run bench.go --mode closed --clients 10 --tx 1000

# open-loop Poisson workload (Section V-J): target 500 TPS
go run bench.go --mode open --rate 500
```

Results are appended to `results/throughput_runs.csv`.

## Hardware testbed

| Tier | Device | Role |
|---|---|---|
| 1 | ESP32-WROOM-32 ×6 | Sensor endpoints, AES-128 HW encryption |
| 2 | Raspberry Pi 4 (4 GB) ×3 | Fabric peers / gateways |
| 3 | NVIDIA Jetson Orin | Fabric CA, Raft ordering, CouchDB |
| — | 1 Gbps switch | Inter-peer wired backbone |

## License

Released under the [Apache License 2.0](LICENSE).

## How to cite

If you use this code or data, please cite the paper and this repository
(see [`CITATION.cff`](CITATION.cff)).
