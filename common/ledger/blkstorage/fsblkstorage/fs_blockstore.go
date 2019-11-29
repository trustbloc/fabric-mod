/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package fsblkstorage

import (
	"time"

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/common/ledger"
	"github.com/hyperledger/fabric/common/ledger/blkstorage"
	"github.com/hyperledger/fabric/common/ledger/util/leveldbhelper"
)

// fsBlockStore - filesystem based implementation for `BlockStore`
type fsBlockStore struct {
	id      string
	conf    *Conf
	fileMgr *blockfileMgr
	stats   *ledgerStats
}

// NewFsBlockStore constructs a `FsBlockStore`
func newFsBlockStore(id string, conf *Conf, indexConfig *blkstorage.IndexConfig,
	dbHandle *leveldbhelper.DBHandle, stats *stats) *fsBlockStore {
	fileMgr := newBlockfileMgr(id, conf, indexConfig, dbHandle)

	// create ledgerStats and initialize blockchain_height stat
	ledgerStats := stats.ledgerStats(id)
	info := fileMgr.getBlockchainInfo()
	ledgerStats.updateBlockchainHeight(info.Height)

	return &fsBlockStore{id, conf, fileMgr, ledgerStats}
}

// AddBlock adds a new block
func (store *fsBlockStore) AddBlock(block *common.Block) error {
	// track elapsed time to collect block commit time
	startBlockCommit := time.Now()
	result := store.fileMgr.addBlock(block)
	elapsedBlockCommit := time.Since(startBlockCommit)

	store.updateBlockStats(block.Header.Number, elapsedBlockCommit)

	return result
}

// GetBlockchainInfo returns the current info about blockchain
func (store *fsBlockStore) GetBlockchainInfo() (*common.BlockchainInfo, error) {
	return store.fileMgr.getBlockchainInfo(), nil
}

// RetrieveBlocks returns an iterator that can be used for iterating over a range of blocks
func (store *fsBlockStore) RetrieveBlocks(startNum uint64) (ledger.ResultsIterator, error) {
	return store.fileMgr.retrieveBlocks(startNum)
}

// RetrieveBlockByHash returns the block for given block-hash
func (store *fsBlockStore) RetrieveBlockByHash(blockHash []byte) (*common.Block, error) {
	return store.fileMgr.retrieveBlockByHash(blockHash)
}

// RetrieveBlockByNumber returns the block at a given blockchain height
func (store *fsBlockStore) RetrieveBlockByNumber(blockNum uint64) (*common.Block, error) {
	return store.fileMgr.retrieveBlockByNumber(blockNum)
}

// RetrieveTxByID returns a transaction for given transaction id
func (store *fsBlockStore) RetrieveTxByID(txID string) (*common.Envelope, error) {
	return store.fileMgr.retrieveTransactionByID(txID)
}

// RetrieveTxByID returns a transaction for given transaction id
func (store *fsBlockStore) RetrieveTxByBlockNumTranNum(blockNum uint64, tranNum uint64) (*common.Envelope, error) {
	return store.fileMgr.retrieveTransactionByBlockNumTranNum(blockNum, tranNum)
}

func (store *fsBlockStore) RetrieveBlockByTxID(txID string) (*common.Block, error) {
	return store.fileMgr.retrieveBlockByTxID(txID)
}

func (store *fsBlockStore) RetrieveTxValidationCodeByTxID(txID string) (peer.TxValidationCode, error) {
	return store.fileMgr.retrieveTxValidationCodeByTxID(txID)
}

func (store *fsBlockStore) CheckpointBlock(block *common.Block) error {
	return nil
}

// Shutdown shuts down the block store
func (store *fsBlockStore) Shutdown() {
	logger.Debugf("closing fs blockStore:%s", store.id)
	store.fileMgr.close()
}

func (store *fsBlockStore) updateBlockStats(blockNum uint64, blockstorageCommitTime time.Duration) {
	store.stats.updateBlockchainHeight(blockNum + 1)
	store.stats.updateBlockstorageCommitTime(blockstorageCommitTime)
}
