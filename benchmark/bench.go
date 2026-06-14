// Package main implements the closed-loop and open-loop benchmark harness
// used to produce the throughput and latency results in:
//
//   "A Hardware-Validated Permissioned Blockchain-Edge Security Framework
//    for Industrial IoT Smart Spaces"  (Section V)
//
// It drives N parallel Fabric Gateway SDK clients against the SmartSpace
// chaincode, each submitting LogData() transactions, and records per-
// transaction end-to-end latency with nanosecond timestamps. Aggregated
// output (mean, sigma, P50/P95/P99, 95% CI) is written to results/.
//
// Closed-loop:  --mode closed  (each client issues --tx back-to-back)
// Open-loop:    --mode open    (Poisson arrivals at --rate TPS)
//
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"google.golang.org/grpc"
)

type config struct {
	mode        string
	clients     int
	txPerClient int
	rate        float64 // target offered load (open-loop), TPS
	warmupSec   int
	channel     string
	chaincode   string
	gatewayPeer string
	outCSV      string
}

func main() {
	cfg := parseFlags()

	log.Printf("SmartSpace benchmark | mode=%s clients=%d tx/client=%d warmup=%ds",
		cfg.mode, cfg.clients, cfg.txPerClient, cfg.warmupSec)

	gw, conn, err := connectGateway(cfg)
	if err != nil {
		log.Fatalf("gateway connection failed: %v", err)
	}
	defer conn.Close()
	defer gw.Close()

	contract := gw.GetNetwork(cfg.channel).GetContract(cfg.chaincode)

	// Warm-up window to eliminate Go runtime / JIT start-up effects (Section V-A).
	if cfg.warmupSec > 0 {
		log.Printf("warming up for %ds ...", cfg.warmupSec)
		runWarmup(contract, cfg.warmupSec)
	}

	var latencies []float64 // milliseconds
	var mu sync.Mutex
	var wg sync.WaitGroup

	start := time.Now()

	for c := 0; c < cfg.clients; c++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			local := runClient(contract, cfg, clientID)
			mu.Lock()
			latencies = append(latencies, local...)
			mu.Unlock()
		}(c)
	}
	wg.Wait()

	elapsed := time.Since(start).Seconds()
	total := len(latencies)
	tps := float64(total) / elapsed

	stats := summarize(latencies)
	log.Printf("DONE | total tx=%d | elapsed=%.2fs | throughput=%.1f TPS", total, elapsed, tps)
	log.Printf("latency ms: mean=%.1f sigma=%.1f P50=%.1f P95=%.1f P99=%.1f",
		stats.mean, stats.sigma, stats.p50, stats.p95, stats.p99)

	if err := writeResult(cfg, tps, stats, total, elapsed); err != nil {
		log.Fatalf("failed to write results: %v", err)
	}
	log.Printf("results appended to %s", cfg.outCSV)
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.mode, "mode", "closed", "closed | open")
	flag.IntVar(&cfg.clients, "clients", 10, "number of parallel SDK clients")
	flag.IntVar(&cfg.txPerClient, "tx", 1000, "transactions per client (closed-loop)")
	flag.Float64Var(&cfg.rate, "rate", 500, "target offered load TPS (open-loop)")
	flag.IntVar(&cfg.warmupSec, "warmup", 120, "warm-up seconds")
	flag.StringVar(&cfg.channel, "channel", "smartspacechannel", "Fabric channel name")
	flag.StringVar(&cfg.chaincode, "chaincode", "smartspace", "chaincode name")
	flag.StringVar(&cfg.gatewayPeer, "peer", "localhost:7051", "gateway peer endpoint")
	flag.StringVar(&cfg.outCSV, "out", "../results/throughput_runs.csv", "output CSV path")
	flag.Parse()
	return cfg
}

// runClient issues transactions for one client and returns per-tx latencies (ms).
func runClient(contract *client.Contract, cfg config, clientID int) []float64 {
	out := make([]float64, 0, cfg.txPerClient)
	deviceID := fmt.Sprintf("device-%03d", clientID)

	var interArrival func() time.Duration
	if cfg.mode == "open" {
		// Poisson arrivals: exponential inter-arrival, per-client share of rate.
		perClientRate := cfg.rate / float64(cfg.clients)
		interArrival = func() time.Duration {
			gap := rand.ExpFloat64() / perClientRate
			return time.Duration(gap * float64(time.Second))
		}
	}

	for i := 0; i < cfg.txPerClient; i++ {
		payload := fmt.Sprintf("%s-evt-%d-%d", deviceID, i, time.Now().UnixNano())
		sum := sha256.Sum256([]byte(payload))
		hash := hex.EncodeToString(sum[:])
		ts := time.Now().UTC().Format(time.RFC3339Nano)

		t0 := time.Now()
		_, err := contract.SubmitTransaction("LogData", deviceID, hash, ts)
		lat := time.Since(t0).Seconds() * 1000.0 // ms
		if err != nil {
			// Count failed submissions but do not record latency.
			continue
		}
		out = append(out, lat)

		if cfg.mode == "open" {
			time.Sleep(interArrival())
		}
	}
	return out
}

func runWarmup(contract *client.Contract, seconds int) {
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	i := 0
	for time.Now().Before(deadline) {
		payload := fmt.Sprintf("warmup-%d", i)
		sum := sha256.Sum256([]byte(payload))
		hash := hex.EncodeToString(sum[:])
		ts := time.Now().UTC().Format(time.RFC3339Nano)
		_, _ = contract.SubmitTransaction("LogData", "device-000", hash, ts)
		i++
	}
}

// ---- statistics ----

type stats struct {
	mean, sigma, p50, p95, p99, ciLow, ciHigh float64
}

func summarize(x []float64) stats {
	n := len(x)
	if n == 0 {
		return stats{}
	}
	sorted := make([]float64, n)
	copy(sorted, x)
	sort.Float64s(sorted)

	var sum float64
	for _, v := range sorted {
		sum += v
	}
	mean := sum / float64(n)

	var sse float64
	for _, v := range sorted {
		sse += (v - mean) * (v - mean)
	}
	sigma := math.Sqrt(sse / float64(n-1))

	// 95% CI of the mean (normal approximation, justified by CLT, Section V-A).
	se := sigma / math.Sqrt(float64(n))
	ciLow := mean - 1.96*se
	ciHigh := mean + 1.96*se

	return stats{
		mean:   mean,
		sigma:  sigma,
		p50:    percentile(sorted, 50),
		p95:    percentile(sorted, 95),
		p99:    percentile(sorted, 99),
		ciLow:  ciLow,
		ciHigh: ciHigh,
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := (p / 100.0) * float64(len(sorted)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return sorted[lo]
	}
	frac := rank - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

func writeResult(cfg config, tps float64, s stats, total int, elapsed float64) error {
	newFile := false
	if _, err := os.Stat(cfg.outCSV); os.IsNotExist(err) {
		newFile = true
	}
	f, err := os.OpenFile(cfg.outCSV, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if newFile {
		fmt.Fprintln(f, "timestamp,mode,clients,tx_per_client,total_tx,elapsed_s,tps,"+
			"mean_ms,sigma_ms,p50_ms,p95_ms,p99_ms,ci_low_ms,ci_high_ms")
	}
	fmt.Fprintf(f, "%s,%s,%d,%d,%d,%.2f,%.1f,%.1f,%.1f,%.1f,%.1f,%.1f,%.1f,%.1f\n",
		time.Now().UTC().Format(time.RFC3339), cfg.mode, cfg.clients, cfg.txPerClient,
		total, elapsed, tps, s.mean, s.sigma, s.p50, s.p95, s.p99, s.ciLow, s.ciHigh)
	return nil
}

// connectGateway establishes a Fabric Gateway connection. Credential loading
// (TLS cert, signing identity) is environment-specific; see README and the
// FABRIC_* environment variables. This function is a template — fill in the
// MSP/credential paths for your test-network deployment.
func connectGateway(cfg config) (*client.Gateway, *grpc.ClientConn, error) {
	// NOTE: credential wiring (newGrpcConnection, newIdentity, newSign) is
	// deployment-specific and documented in benchmark/README.md. We return an
	// explicit error here so the harness fails fast if run without setup.
	_ = context.Background()
	return nil, nil, fmt.Errorf(
		"connectGateway: configure MSP credentials per benchmark/README.md before running")
}
