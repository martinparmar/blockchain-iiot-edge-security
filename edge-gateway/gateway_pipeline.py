#!/usr/bin/env python3
"""
gateway_pipeline.py - Tier 2 Edge Gateway reference pipeline (Raspberry Pi 4)

Part of the four-tier blockchain-edge IoT security framework:
  "A Hardware-Validated Permissioned Blockchain-Edge Security Framework
   for Industrial IoT Smart Spaces"  (Section IV-C)

Implements the gateway-side processing pipeline (Fig. 2, steps 3-5):

  3. Decrypt AES-128-GCM + verify device DID
  4. SHA-256 hash + local-alert threshold check (< 12.5 ms)
  5. ECDSA P-256 sign + re-encrypt AES-256-GCM, then submit LogData() to Fabric

This is a reference / documentation implementation showing the data path and
timing instrumentation. Fabric submission is delegated to the Go benchmark
client (benchmark/bench.go); here we focus on the cryptographic pipeline that
produces the 12.5 ms edge-processing stage reported in Section V-D.

Dependencies: cryptography
    pip install cryptography

SPDX-License-Identifier: Apache-2.0
"""

import hashlib
import time
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives import hashes

# Demo keys (provisioned via Fabric CA / key-management lifecycle, Section IV-H).
DEVICE_KEY_128 = bytes.fromhex("2b7e151628aed2a6abf7158809cf4f3c")     # AES-128 (Zone 2)
CLOUD_KEY_256 = bytes.fromhex(
    "603deb1015ca71be2b73aef0857d77811f352c073b6108d72d9810a30914dff4")  # AES-256 (Zone 3)

LOCAL_ALERT_THRESHOLD_MS = 12.5  # Section IV-C / V-D


class EdgeGateway:
    """Reference implementation of the Tier 2 gateway pipeline."""

    def __init__(self):
        # Gateway signing key (ECDSA P-256, Section IV-D).
        self.signing_key = ec.generate_private_key(ec.SECP256R1())
        self.registered_dids = set()

    def register_device(self, did: str):
        """Mirror of on-chain RegisterDevice() world-state for fast lookup."""
        self.registered_dids.add(did)

    def process(self, did: str, nonce: bytes, ciphertext: bytes) -> dict:
        """Run the full gateway pipeline and return timing + the on-chain hash."""
        t_start = time.perf_counter()

        # --- Step 3: AES-128-GCM decrypt + DID verify (Zone 2 AEAD) ---
        aes128 = AESGCM(DEVICE_KEY_128)
        # The device DID is bound as associated data: tampering invalidates the tag.
        plaintext = aes128.decrypt(nonce, ciphertext, did.encode())

        if did not in self.registered_dids:
            raise PermissionError(f"unregistered DID rejected: {did}")

        # --- Step 4: SHA-256 hash + local alert threshold ---
        digest = hashlib.sha256(plaintext).hexdigest()
        t_hash = time.perf_counter()
        edge_ms = (t_hash - t_start) * 1000.0
        alert = edge_ms < LOCAL_ALERT_THRESHOLD_MS

        # --- Step 5: ECDSA P-256 sign + re-encrypt AES-256-GCM (Zone 3) ---
        signature = self.signing_key.sign(
            digest.encode(), ec.ECDSA(hashes.SHA256()))
        aes256 = AESGCM(CLOUD_KEY_256)
        uplink_nonce = b"\x00" * 12  # demo nonce; use a fresh random nonce in production
        uplink_ct = aes256.encrypt(uplink_nonce, digest.encode(), did.encode())

        t_end = time.perf_counter()
        total_ms = (t_end - t_start) * 1000.0

        return {
            "did": did,
            "sha256": digest,            # 32-byte hash committed on-chain (LogData)
            "signature_len": len(signature),
            "uplink_ciphertext_len": len(uplink_ct),
            "edge_stage_ms": round(edge_ms, 3),
            "total_pipeline_ms": round(total_ms, 3),
            "local_alert": alert,
        }


def _demo():
    gw = EdgeGateway()
    did = "did:smartspace:esp32:0001"
    gw.register_device(did)

    # Simulate an encrypted device frame.
    aes128 = AESGCM(DEVICE_KEY_128)
    nonce = b"\x01" * 12
    msg = b'{"did":"%s","v":23.7}' % did.encode()
    ct = aes128.encrypt(nonce, msg, did.encode())

    result = gw.process(did, nonce, ct)
    print("Gateway pipeline result:")
    for k, v in result.items():
        print(f"  {k}: {v}")


if __name__ == "__main__":
    _demo()
