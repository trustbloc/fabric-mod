/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/ledger/rwset/kvrwset"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/core/common/ccprovider"
	"github.com/hyperledger/fabric/core/ledger"
)

// ConfigUpdateHandler handles a config update
type ConfigUpdateHandler func(blockNum uint64, configUpdate *cb.ConfigUpdate) error

// WriteHandler handles a KV write
type WriteHandler func(txMetadata TxMetadata, namespace string, kvWrite *kvrwset.KVWrite) error

// ReadHandler handles a KV read
type ReadHandler func(txMetadata TxMetadata, namespace string, kvRead *kvrwset.KVRead) error

// CollHashWriteHandler handles a KV collection hash write
type CollHashWriteHandler func(txMetadata TxMetadata, namespace, collection string, kvWrite *kvrwset.KVWriteHash) error

// ChaincodeEventHandler handles a chaincode event
type ChaincodeEventHandler func(txMetadata TxMetadata, event *pb.ChaincodeEvent) error

// ChaincodeUpgradeHandler handles chaincode upgrade events
type ChaincodeUpgradeHandler func(txMetadata TxMetadata, chaincodeName string) error

// LSCCWriteHandler handles chaincode instantiation/upgrade events
type LSCCWriteHandler func(txMetadata TxMetadata, chaincodeName string, ccData *ccprovider.ChaincodeData, ccp *pb.CollectionConfigPackage) error

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
	// AddCollHashWriteHandler adds a new handler for KV collection hash writes
	AddCollHashWriteHandler(handler CollHashWriteHandler)
	// AddLSCCWriteHandler adds a handler for LSCC writes (for chaincode instantiate/upgrade)
	AddLSCCWriteHandler(handler LSCCWriteHandler)
	// AddCCEventHandler adds a handler for chaincode events
	AddCCEventHandler(handler ChaincodeEventHandler)
	// Publish traverses the block and private data and invokes all applicable handlers
	Publish(block *cb.Block, pvtData ledger.TxPvtDataMap)
	//LedgerHeight returns current in memory ledger height
	LedgerHeight() uint64
}

// TxMetadata contain txn metadata
type TxMetadata struct {
	BlockNum  uint64
	TxNum     uint64
	ChannelID string
	TxID      string
}

//LedgerHeightProvider provides current ledger height
type LedgerHeightProvider interface {
	//LedgerHeight  returns current in-memory ledger height
	LedgerHeight() uint64
}

// Support aggregates functionality of several
// interfaces required by gossip service
type Support struct {
	Ledger               ledger.PeerLedger
	LedgerHeightProvider LedgerHeightProvider
}

// GossipService contains Gossip function
type GossipService interface {
	// It's up to extensions which functions are exposed
}
