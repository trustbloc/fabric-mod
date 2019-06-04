/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	cb "github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/ledger/rwset/kvrwset"
	pb "github.com/hyperledger/fabric/protos/peer"
)

// ConfigUpdateHandler handles a config update
type ConfigUpdateHandler func(blockNum uint64, configUpdate *cb.ConfigUpdate) error

// WriteHandler handles a KV write
type WriteHandler func(blockNum uint64, channelID, txID, namespace string, kvWrite *kvrwset.KVWrite) error

// ReadHandler handles a KV read
type ReadHandler func(blockNum uint64, channelID, txID, namespace string, kvRead *kvrwset.KVRead) error

// ChaincodeEventHandler handles a chaincode event
type ChaincodeEventHandler func(blockNum uint64, channelID, txID string, event *pb.ChaincodeEvent) error

// ChaincodeUpgradeHandler handles chaincode upgrade events
type ChaincodeUpgradeHandler func(blockNum uint64, txID string, chaincodeName string) error

// BlockPublisher allows clients to add handlers for various block events
type BlockPublisher interface {
	// AddCCUpgradeHandler adds a handler for chaincode upgrades
	AddCCUpgradeHandler(handler ChaincodeUpgradeHandler)
	// AddConfigUpdateHandler adds a handler for config updates
	AddConfigUpdateHandler(handler ConfigUpdateHandler)
	// AddWriteHandler adds a handler for KV writes
	AddWriteHandler(handler WriteHandler)
	// AddReadHandler adds a handler for KV reads
	AddReadHandler(handler ReadHandler)
	// AddCCEventHandler adds a handler for chaincode events
	AddCCEventHandler(handler ChaincodeEventHandler)
	// Publish traverses the block and invokes all applicable handlers
	Publish(block *cb.Block)
}
