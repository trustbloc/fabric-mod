/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"github.com/hyperledger/fabric-protos-go/common"
	tsproto "github.com/hyperledger/fabric-protos-go/transientstore"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/kvledger/msgs"
	"github.com/hyperledger/fabric/core/ledger/pvtdatapolicy"
	"github.com/hyperledger/fabric/core/transientstore"
)

//go:generate counterfeiter -o idstore/mock_idstore.go --fake-name MockIDStore . IDStore

type IDStore interface {
	SetUnderConstructionFlag(string) error
	UnsetUnderConstructionFlag() error
	GetUnderConstructionFlag() (string, error)
	CreateLedgerID(ledgerID string, gb *common.Block) error
	LedgerIDExists(ledgerID string) (bool, error)
	LedgerIDActive(ledgerID string) (active bool, exists bool, err error)
	GetActiveLedgerIDs() ([]string, error)
	UpdateLedgerStatus(ledgerID string, newStatus msgs.Status) error
	GetFormat() ([]byte, error)
	UpgradeFormat() error
	GetGenesisBlock(ledgerID string) (*common.Block, error)
	Close()
}

// TransientStore is a transient store
type TransientStore interface {
	// Persist stores the private write set of a transaction along with the collection config
	// in the transient store based on txid and the block height the private data was received at
	Persist(txid string, blockHeight uint64, privateSimulationResultsWithConfig *tsproto.TxPvtReadWriteSetWithConfigInfo) error

	// GetTxPvtRWSetByTxid returns an iterator due to the fact that the txid may have multiple private
	// write sets persisted from different endorsers.
	GetTxPvtRWSetByTxid(txid string, filter ledger.PvtNsCollFilter) (transientstore.RWSetScanner, error)

	// PurgeByTxids removes private write sets of a given set of transactions from the
	// transient store. PurgeByTxids() is expected to be called by coordinator after
	// committing a block to ledger.
	PurgeByTxids(txids []string) error

	// PurgeBelowHeight removes private write sets at block height lesser than
	// a given maxBlockNumToRetain. In other words, Purge only retains private write sets
	// that were persisted at block height of maxBlockNumToRetain or higher. Though the private
	// write sets stored in transient store is removed by coordinator using PurgebyTxids()
	// after successful block commit, PurgeBelowHeight() is still required to remove orphan entries (as
	// transaction that gets endorsed may not be submitted by the client for commit)
	PurgeBelowHeight(maxBlockNumToRetain uint64) error

	// GetMinTransientBlkHt returns the lowest block height remaining in transient store
	GetMinTransientBlkHt() (uint64, error)
}

// TransientStoreProvider is a transient store provider
type TransientStoreProvider interface {
	OpenStore(ledgerID string) (TransientStore, error)
	Close()
}

// PrivateDataProvider provides handle to specific 'PrivateDataStore' that in turn manages
// private write sets for a ledger
type PrivateDataProvider interface {
	OpenStore(id string) (PrivateDataStore, error)
	Close()
}

// PrivateDataStore manages the permanent storage of private write sets for a ledger
// Because the pvt data is supposed to be in sync with the blocks in the
// ledger, both should logically happen in an atomic operation. In order
// to accomplish this, an implementation of this store should provide
// support for a two-phase like commit/rollback capability.
// The expected use is such that - first the private data will be given to
// this store (via `Prepare` function) and then the block is appended to the block storage.
// Finally, one of the functions `Commit` or `Rollback` is invoked on this store based
// on whether the block was written successfully or not. The store implementation
// is expected to survive a server crash between the call to `Prepare` and `Commit`/`Rollback`
type PrivateDataStore interface {
	// Init initializes the store. This function is expected to be invoked before using the store
	Init(btlPolicy pvtdatapolicy.BTLPolicy)
	// GetPvtDataByBlockNum returns only the pvt data  corresponding to the given block number
	// The pvt data is filtered by the list of 'ns/collections' supplied in the filter
	// A nil filter does not filter any results
	GetPvtDataByBlockNum(blockNum uint64, filter ledger.PvtNsCollFilter) ([]*ledger.TxPvtData, error)
	// GetMissingPvtDataInfoForMostRecentBlocks returns the missing private data information for the
	// most recent `maxBlock` blocks which miss at least a private data of a eligible collection.
	GetMissingPvtDataInfoForMostRecentBlocks(maxBlock int) (ledger.MissingPvtDataInfo, error)
	// Commit commits the pvt data as well as both the eligible and ineligible
	// missing private data --- `eligible` denotes that the missing private data belongs to a collection
	// for which this peer is a member; `ineligible` denotes that the missing private data belong to a
	// collection for which this peer is not a member.
	Commit(blockNum uint64, pvtData []*ledger.TxPvtData, missingPvtData ledger.TxMissingPvtDataMap) error
	// ProcessCollsEligibilityEnabled notifies the store when the peer becomes eligible to receive data for an
	// existing collection. Parameter 'committingBlk' refers to the block number that contains the corresponding
	// collection upgrade transaction and the parameter 'nsCollMap' contains the collections for which the peer
	// is now eligible to receive pvt data
	ProcessCollsEligibilityEnabled(committingBlk uint64, nsCollMap map[string][]string) error
	// CommitPvtDataOfOldBlocks commits the pvtData (i.e., previously missing data) of old blocks.
	// The parameter `blocksPvtData` refers a list of old block's pvtdata which are missing in the pvtstore.
	// This call stores an additional entry called `lastUpdatedOldBlocksList` which keeps the exact list
	// of updated blocks. This list would be used during recovery process. Once the stateDB is updated with
	// these pvtData, the `lastUpdatedOldBlocksList` must be removed. During the peer startup,
	// if the `lastUpdatedOldBlocksList` exists, stateDB needs to be updated with the appropriate pvtData.
	CommitPvtDataOfOldBlocks(blocksPvtData map[uint64][]*ledger.TxPvtData) error
	// GetLastUpdatedOldBlocksPvtData returns the pvtdata of blocks listed in `lastUpdatedOldBlocksList`
	GetLastUpdatedOldBlocksPvtData() (map[uint64][]*ledger.TxPvtData, error)
	// ResetLastUpdatedOldBlocksList removes the `lastUpdatedOldBlocksList` entry from the store
	ResetLastUpdatedOldBlocksList() error
	// LastCommittedBlockHeight returns the height of the last committed block
	LastCommittedBlockHeight() (uint64, error)
}

// CouchDatabase is a handle to a Couch Database implementation
type CouchDatabase interface {
	CreateDatabaseIfNotExist() error
}

// CouchInstance is a handle to a Couch Instance implementation
type CouchInstance interface {
	URL() string
}
