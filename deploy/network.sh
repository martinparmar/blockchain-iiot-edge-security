#!/usr/bin/env bash
#
# network.sh - Deploy the SmartSpace chaincode on a Hyperledger Fabric v2.4.9
#              test network (3 peers, Raft ordering) for the framework in:
#
#   "A Hardware-Validated Permissioned Blockchain-Edge Security Framework
#    for Industrial IoT Smart Spaces"
#
# Prerequisites:
#   - Docker & Docker Compose
#   - Hyperledger Fabric v2.4.9 binaries + fabric-samples on PATH
#   - Set FABRIC_SAMPLES to your fabric-samples directory
#
# Usage:
#   ./network.sh up        # start orderer + 3 peers (Raft) + create channel
#   ./network.sh deploy    # package, install, approve & commit chaincode
#   ./network.sh down       # tear down
#
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

CHANNEL="smartspacechannel"
CC_NAME="smartspace"
CC_PATH="../chaincode/smartspace"
CC_LANG="golang"
CC_VERSION="1.0"
CC_SEQUENCE="1"
ENDORSEMENT_POLICY_FILE="endorsement-policy.json"

: "${FABRIC_SAMPLES:?Set FABRIC_SAMPLES to your fabric-samples directory}"
TEST_NETWORK="${FABRIC_SAMPLES}/test-network"

cmd="${1:-help}"

case "$cmd" in
  up)
    echo ">> Starting Fabric test network with Raft ordering + CouchDB ..."
    pushd "$TEST_NETWORK" >/dev/null
    ./network.sh up createChannel -c "$CHANNEL" -s couchdb
    popd >/dev/null
    echo ">> Network up. Channel '$CHANNEL' created."
    ;;

  deploy)
    echo ">> Deploying chaincode '$CC_NAME' ..."
    pushd "$TEST_NETWORK" >/dev/null
    # The endorsement policy requires the device home-gateway peer to co-sign
    # LogData() transactions (FDIA defense, Section IV-D). For the 2-org test
    # network this maps to AND('Org1MSP.peer','Org2MSP.peer').
    ./network.sh deployCC \
      -c "$CHANNEL" \
      -ccn "$CC_NAME" \
      -ccp "$(cd "$OLDPWD/$CC_PATH" && pwd)" \
      -ccl "$CC_LANG" \
      -ccv "$CC_VERSION" \
      -ccs "$CC_SEQUENCE" \
      -ccep "AND('Org1MSP.peer','Org2MSP.peer')"
    popd >/dev/null
    echo ">> Chaincode '$CC_NAME' committed on channel '$CHANNEL'."
    ;;

  down)
    echo ">> Tearing down network ..."
    pushd "$TEST_NETWORK" >/dev/null
    ./network.sh down
    popd >/dev/null
    ;;

  *)
    echo "Usage: $0 {up|deploy|down}"
    exit 1
    ;;
esac
