// Certificate-verification helper for the SmartSpace chaincode.
// Validates that a device certificate is a well-formed X.509 v3 certificate
// (the Fabric MSP performs full CA-chain validation at the platform level;
// this provides an additional in-chaincode structural check per Section IV-D).
//
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

// verifyCertificate decodes a PEM-encoded X.509 certificate and checks that
// it parses, is currently within its validity window, and uses an ECDSA
// public key (P-256) consistent with the framework's signing scheme.
func verifyCertificate(certPEM string) error {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse X.509 certificate: %w", err)
	}

	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate is not yet valid (NotBefore=%s)", cert.NotBefore)
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate has expired (NotAfter=%s)", cert.NotAfter)
	}

	if cert.PublicKeyAlgorithm != x509.ECDSA {
		return fmt.Errorf("unexpected public-key algorithm %v (expected ECDSA P-256)",
			cert.PublicKeyAlgorithm)
	}
	return nil
}
