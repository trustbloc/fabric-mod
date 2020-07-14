/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/common/ledger"
	"github.com/hyperledger/fabric/common/ledger/snapshot"
)

//PeerLedgerExtension is an extension to PeerLedger interface which can be used to extend existing peer ledger features.
type PeerLedgerExtension interface {
	//CheckpointBlock updates check point info in underlying store
	CheckpointBlock(block *common.Block) error
}

// SnapshotInfo captures some of the details about the snapshot
type SnapshotInfo struct {
	LedgerID          string
	LastBlockNum      uint64
	LastBlockHash     []byte
	PreviousBlockHash []byte
}

// BlockStoreProvider provides an handle to a BlockStore
type BlockStoreProvider interface {
	Open(ledgerid string) (BlockStore, error)
	Close()
}

// BlockStore - an interface for persisting and retrieving blocks
// An implementation of this interface is expected to take an argument
// of type `IndexConfig` which configures the block store on what items should be indexed
type BlockStore interface {
	PeerLedgerExtension
	AddBlock(block *common.Block) error
	GetBlockchainInfo() (*common.BlockchainInfo, error)
	RetrieveBlocks(startNum uint64) (ledger.ResultsIterator, error)
	RetrieveBlockByHash(blockHash []byte) (*common.Block, error)
	RetrieveBlockByNumber(blockNum uint64) (*common.Block, error) // blockNum of  math.MaxUint64 will return last block
	RetrieveTxByID(txID string) (*common.Envelope, error)
	RetrieveTxByBlockNumTranNum(blockNum uint64, tranNum uint64) (*common.Envelope, error)
	RetrieveBlockByTxID(txID string) (*common.Block, error)
	RetrieveTxValidationCodeByTxID(txID string) (peer.TxValidationCode, error)
	ExportTxIds(dir string, newHashFunc snapshot.NewHashFunc) (map[string][]byte, error)
	Shutdown()
}
