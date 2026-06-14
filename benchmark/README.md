# Benchmark Harness

Reproduces the throughput and latency measurements of Section V.

## Reproduce statistics without Fabric (fastest)

```bash
pip install -r requirements.txt
python analyze.py
```

This reads `../results/*.csv` and prints Tables VII–XII plus regenerates
Figures 6 and 8 into `figures/`.

## Run the live benchmark against a Fabric network

### 1. Bring up the network and deploy chaincode

```bash
cd ../deploy
export FABRIC_SAMPLES=/path/to/fabric-samples
./network.sh up
./network.sh deploy
```

### 2. Configure MSP credentials

`bench.go` connects through the Fabric Gateway SDK and needs three things from
your test-network's crypto material (under
`fabric-samples/test-network/organizations/`):

| Environment variable | Points to |
|---|---|
| `FABRIC_MSP_ID` | e.g. `Org1MSP` |
| `FABRIC_CERT_PATH` | the user signing certificate (`User1@org1.../signcerts/*.pem`) |
| `FABRIC_KEY_PATH` | the user private key (`User1@org1.../keystore/*`) |
| `FABRIC_TLS_CERT_PATH` | the peer TLS CA cert (`peers/peer0.../tls/ca.crt`) |
| `FABRIC_PEER_ENDPOINT` | e.g. `localhost:7051` |
| `FABRIC_GATEWAY_PEER` | e.g. `peer0.org1.example.com` |

The `connectGateway()` function in `bench.go` is a documented template — wire in
`newGrpcConnection`, `newIdentity`, and `newSign` using these variables, following
the [fabric-gateway Go tutorial](https://hyperledger.github.io/fabric-gateway/).

### 3. Run

```bash
# Closed-loop microbenchmark (Section V-B): 10 clients × 1000 tx, 120 s warm-up
go run bench.go --mode closed --clients 10 --tx 1000 --warmup 120

# Open-loop Poisson workload (Section V-J): target offered load 500 TPS
go run bench.go --mode open --rate 500 --warmup 120

# Scalability sweep (Section V-E): vary peers in the network, re-run per config
go run bench.go --mode closed --clients 10 --tx 1000
```

Each run appends a row to `../results/throughput_runs.csv`. Re-run `analyze.py`
to recompute the aggregate statistics.

## Notes on reproducibility

- Absolute TPS/latency depend on hardware. The committed CSVs were produced on
  the testbed described in the paper (Raspberry Pi 4 peers, Jetson Orin orderer).
- The statistics (`analyze.py`) are deterministic given the CSVs.
- For node-failure / attack experiments (Section V-K), see `../docs/REPRODUCE.md`.
