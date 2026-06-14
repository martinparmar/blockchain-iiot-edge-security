// Package main implements the SmartSpace chaincode for the four-tier
// blockchain-edge IoT security framework described in:
//
//   "A Hardware-Validated Permissioned Blockchain-Edge Security Framework
//    for Industrial IoT Smart Spaces"
//
// The chaincode exposes three security-critical transaction functions
// (Section IV-D of the manuscript):
//
//   RegisterDevice()  - DID + CA certificate verification, firmware-hash anchoring
//   AuthorizeUser()   - owner-role enforced access-control updates
//   LogData()         - SHA-256 hash-only ledger commit with gateway co-endorsement
//
// Target platform: Hyperledger Fabric v2.4.9 (Raft ordering service).
//
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// SmartSpaceContract provides functions for managing IoT device identity,
// access control, and immutable data logging on a permissioned ledger.
type SmartSpaceContract struct {
	contractapi.Contract
}

// Device is the world-state record for a registered IoT endpoint.
// Only a 32-byte SHA-256 firmware hash and metadata are stored on-chain;
// raw payloads are kept off-chain in IPFS (Section IV-C, IV-E).
type Device struct {
	DeviceID     string   `json:"deviceID"`
	OwnerMSP     string   `json:"ownerMSP"`
	Authorized   []string `json:"authorized"`   // authorized user IDs
	CertPEM      string   `json:"certPEM"`      // X.509 v3 certificate (Fabric CA issued)
	FirmwareHash string   `json:"firmwareHash"` // SHA-256 of golden firmware image
	DIDDocument  string   `json:"didDocument"`  // W3C DID v1.0 reference
	CreatedAt    string   `json:"createdAt"`
}

// DataLogEntry is the on-chain record produced by LogData(). It stores only
// the SHA-256 hash of the sensor payload, yielding the 98.6% on-chain
// storage reduction reported in Section V-H.
type DataLogEntry struct {
	DeviceID  string `json:"deviceID"`
	SHA256    string `json:"sha256"`    // 32-byte hex hash of the off-chain payload
	Timestamp string `json:"timestamp"` // RFC 3339
}

const (
	logObjectType = "Log"
)

// RegisterDevice registers a new IoT device. It (1) verifies the supplied
// X.509 certificate was issued by the Fabric CA, (2) rejects duplicate
// device IDs, and (3) anchors the firmware hash on the ledger.
// Corresponds to RegisterDevice() in Table III.
func (s *SmartSpaceContract) RegisterDevice(
	ctx contractapi.TransactionContextInterface,
	deviceID string, ownerMSP string, certPEM string, firmwareHash string, didDoc string,
) error {
	// (1) Verify the certificate is well-formed and CA-issued.
	if err := verifyCertificate(certPEM); err != nil {
		return fmt.Errorf("certificate verification failed: %w", err)
	}

	// (2) Reject duplicate registration.
	existing, err := ctx.GetStub().GetState(deviceID)
	if err != nil {
		return fmt.Errorf("world-state read failed: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("device %s is already registered", deviceID)
	}

	// (3) Bind the caller's MSP identity as the device owner.
	clientMSP, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return fmt.Errorf("could not read client MSP identity: %w", err)
	}
	if ownerMSP == "" {
		ownerMSP = clientMSP
	}

	device := Device{
		DeviceID:     deviceID,
		OwnerMSP:     ownerMSP,
		Authorized:   []string{},
		CertPEM:      certPEM,
		FirmwareHash: firmwareHash,
		DIDDocument:  didDoc,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	deviceJSON, err := json.Marshal(device)
	if err != nil {
		return err
	}
	if err := ctx.GetStub().PutState(deviceID, deviceJSON); err != nil {
		return fmt.Errorf("failed to write device record: %w", err)
	}

	// Emit an event consumed by the off-chain IDS (Section IV-D).
	_ = ctx.GetStub().SetEvent("DeviceRegistered", []byte(deviceID))
	return nil
}

// AuthorizeUser adds or revokes a user's access to a device. Only the
// device owner (verified via the client MSP identity) may call this.
// Corresponds to AuthorizeUser() in Table III. The op argument is
// "grant" or "revoke".
func (s *SmartSpaceContract) AuthorizeUser(
	ctx contractapi.TransactionContextInterface,
	deviceID string, userID string, op string,
) error {
	device, err := s.readDevice(ctx, deviceID)
	if err != nil {
		return err
	}

	// Owner-only enforcement: caller MSP must equal the record owner.
	clientMSP, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return fmt.Errorf("could not read client MSP identity: %w", err)
	}
	if clientMSP != device.OwnerMSP {
		return fmt.Errorf("access denied: caller %s is not the owner of device %s", clientMSP, deviceID)
	}

	switch op {
	case "grant":
		if !contains(device.Authorized, userID) {
			device.Authorized = append(device.Authorized, userID)
		}
	case "revoke":
		device.Authorized = remove(device.Authorized, userID)
	default:
		return fmt.Errorf("unknown operation %q (expected grant or revoke)", op)
	}

	deviceJSON, err := json.Marshal(device)
	if err != nil {
		return err
	}
	if err := ctx.GetStub().PutState(deviceID, deviceJSON); err != nil {
		return err
	}

	_ = ctx.GetStub().SetEvent("AccessChanged", []byte(deviceID+":"+userID+":"+op))
	return nil
}

// LogData commits a SHA-256 payload hash to the ledger under a composite
// key (Log, [deviceID, timestamp]). The endorsement policy must require the
// device's home-gateway peer to co-sign this transaction, which blocks
// false-data-injection (FDIA) at the consensus layer (Section IV-D, G2).
// Corresponds to LogData() in Table III.
func (s *SmartSpaceContract) LogData(
	ctx contractapi.TransactionContextInterface,
	deviceID string, sha256Hash string, timestamp string,
) error {
	// Device must be registered.
	device, err := s.readDevice(ctx, deviceID)
	if err != nil {
		return err
	}

	if len(sha256Hash) != 64 {
		return fmt.Errorf("invalid SHA-256 hash length: got %d, want 64 hex chars", len(sha256Hash))
	}
	if timestamp == "" {
		timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	key, err := ctx.GetStub().CreateCompositeKey(logObjectType, []string{deviceID, timestamp})
	if err != nil {
		return fmt.Errorf("composite-key creation failed: %w", err)
	}

	entry := DataLogEntry{DeviceID: device.DeviceID, SHA256: sha256Hash, Timestamp: timestamp}
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := ctx.GetStub().PutState(key, entryJSON); err != nil {
		return fmt.Errorf("ledger commit failed: %w", err)
	}

	_ = ctx.GetStub().SetEvent("DataLogged", []byte(deviceID+":"+sha256Hash))
	return nil
}

// QueryDevice returns the world-state record for a device (read-only).
func (s *SmartSpaceContract) QueryDevice(
	ctx contractapi.TransactionContextInterface, deviceID string,
) (*Device, error) {
	return s.readDevice(ctx, deviceID)
}

// GetDataHistory returns all data-log entries for a device using a
// composite-key partial query.
func (s *SmartSpaceContract) GetDataHistory(
	ctx contractapi.TransactionContextInterface, deviceID string,
) ([]*DataLogEntry, error) {
	iter, err := ctx.GetStub().GetStateByPartialCompositeKey(logObjectType, []string{deviceID})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var entries []*DataLogEntry
	for iter.HasNext() {
		kv, err := iter.Next()
		if err != nil {
			return nil, err
		}
		var e DataLogEntry
		if err := json.Unmarshal(kv.Value, &e); err != nil {
			return nil, err
		}
		entries = append(entries, &e)
	}
	return entries, nil
}

// ---- helpers ----

func (s *SmartSpaceContract) readDevice(
	ctx contractapi.TransactionContextInterface, deviceID string,
) (*Device, error) {
	data, err := ctx.GetStub().GetState(deviceID)
	if err != nil {
		return nil, fmt.Errorf("world-state read failed: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("device %s is not registered", deviceID)
	}
	var d Device
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// ComputeSHA256 is a convenience helper used by the edge gateway to produce
// the 32-byte hash stored on-chain.
func ComputeSHA256(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func remove(s []string, v string) []string {
	out := s[:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func main() {
	cc, err := contractapi.NewChaincode(&SmartSpaceContract{})
	if err != nil {
		panic(fmt.Sprintf("error creating SmartSpace chaincode: %v", err))
	}
	if err := cc.Start(); err != nil {
		panic(fmt.Sprintf("error starting SmartSpace chaincode: %v", err))
	}
}
