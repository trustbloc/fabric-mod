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
	"github.com/hyperledger/fabric/core/transientstore"
)

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
