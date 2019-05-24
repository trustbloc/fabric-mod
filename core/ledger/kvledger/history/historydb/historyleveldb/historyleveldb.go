/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package historyleveldb

import (
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/common/ledger/blkstorage"
	"github.com/hyperledger/fabric/common/ledger/util/leveldbhelper"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/kvledger/history/historydb"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/rwsetutil"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/version"
	"github.com/hyperledger/fabric/core/ledger/util"
	"github.com/hyperledger/fabric/protos/common"
	protoutil "github.com/hyperledger/fabric/protoutil"
)

var logger = flogging.MustGetLogger("historyleveldb")

var savePointKey = []byte{0x00}
var emptyValue = []byte{}

// HistoryDBProvider implements interface HistoryDBProvider
type HistoryDBProvider struct {
	dbProvider *leveldbhelper.Provider
}

// NewHistoryDBProvider instantiates HistoryDBProvider
func NewHistoryDBProvider(dbPath string) *HistoryDBProvider {
	logger.Debugf("constructing HistoryDBProvider dbPath=%s", dbPath)
	dbProvider := leveldbhelper.NewProvider(&leveldbhelper.Conf{DBPath: dbPath})
	return &HistoryDBProvider{
		dbProvider: dbProvider,
	}
}

// GetDBHandle gets the handle to a named database
func (p *HistoryDBProvider) GetDBHandle(dbName string) (historydb.HistoryDB, error) {
	return newHistoryDB(
			p.dbProvider.GetDBHandle(dbName),
			dbName,
		),
		nil
}

// Close closes the underlying db
func (p *HistoryDBProvider) Close() {
	p.dbProvider.Close()
}

// historyDB implements HistoryDB interface
type historyDB struct {
	db     *leveldbhelper.DBHandle
	dbName string
}

// newHistoryDB constructs an instance of HistoryDB
func newHistoryDB(db *leveldbhelper.DBHandle, dbName string) *historyDB {
	return &historyDB{
		db:     db,
		dbName: dbName,
	}
}

// Commit implements method in HistoryDB interface
func (h *historyDB) Commit(block *common.Block) error {

	blockNo := block.Header.Number
	//Set the starting tranNo to 0
	var tranNo uint64

	dbBatch := leveldbhelper.NewUpdateBatch()

	logger.Debugf("Channel [%s]: Updating history database for blockNo [%v] with [%d] transactions",
		h.dbName, blockNo, len(block.Data.Data))

	// Get the invalidation byte array for the block
	txsFilter := util.TxValidationFlags(block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER])

	// write each tran's write set to history db
	for _, envBytes := range block.Data.Data {

		// If the tran is marked as invalid, skip it
		if txsFilter.IsInvalid(int(tranNo)) {
			logger.Debugf("Channel [%s]: Skipping history write for invalid transaction number %d",
				h.dbName, tranNo)
			tranNo++
			continue
		}

		env, err := protoutil.GetEnvelopeFromBlock(envBytes)
		if err != nil {
			return err
		}

		payload, err := protoutil.GetPayload(env)
		if err != nil {
			return err
		}

		chdr, err := protoutil.UnmarshalChannelHeader(payload.Header.ChannelHeader)
		if err != nil {
			return err
		}

		if common.HeaderType(chdr.Type) == common.HeaderType_ENDORSER_TRANSACTION {

			// extract actions from the envelope message
			respPayload, err := protoutil.GetActionFromEnvelope(envBytes)
			if err != nil {
				return err
			}

			//preparation for extracting RWSet from transaction
			txRWSet := &rwsetutil.TxRwSet{}

			// Get the Result from the Action and then Unmarshal
			// it into a TxReadWriteSet using custom unmarshalling
			if err = txRWSet.FromProtoBytes(respPayload.Results); err != nil {
				return err
			}
			// for each transaction, loop through the namespaces and writesets
			// and add a history record for each write
			for _, nsRWSet := range txRWSet.NsRwSets {
				ns := nsRWSet.NameSpace

				for _, kvWrite := range nsRWSet.KvRwSet.Writes {
					writeKey := kvWrite.Key

					//composite key for history records is in the form ns~key~blockNo~tranNo
					compositeHistoryKey := historydb.ConstructCompositeHistoryKey(ns, writeKey, blockNo, tranNo)

					// No value is required, write an empty byte array (emptyValue) since Put() of nil is not allowed
					dbBatch.Put(compositeHistoryKey, emptyValue)
				}
			}

		} else {
			logger.Debugf("Skipping transaction [%d] since it is not an endorsement transaction\n", tranNo)
		}
		tranNo++
	}

	// add savepoint for recovery purpose
	height := version.NewHeight(blockNo, tranNo)
	dbBatch.Put(savePointKey, height.ToBytes())

	// write the block's history records and savepoint to LevelDB
	// Setting snyc to true as a precaution, false may be an ok optimization after further testing.
	if err := h.db.WriteBatch(dbBatch, true); err != nil {
		return err
	}

	logger.Debugf("Channel [%s]: Updates committed to history database for blockNo [%v]", h.dbName, blockNo)
	return nil
}

// NewHistoryQueryExecutor implements method in HistoryDB interface
func (h *historyDB) NewHistoryQueryExecutor(blockStore blkstorage.BlockStore) (ledger.HistoryQueryExecutor, error) {
	return &LevelHistoryDBQueryExecutor{h, blockStore}, nil
}

// GetBlockNumFromSavepoint implements method in HistoryDB interface
func (h *historyDB) GetLastSavepoint() (*version.Height, error) {
	versionBytes, err := h.db.Get(savePointKey)
	if err != nil || versionBytes == nil {
		return nil, err
	}
	height, _ := version.NewHeightFromBytes(versionBytes)
	return height, nil
}

// ShouldRecover implements method in interface kvledger.Recoverer
func (h *historyDB) ShouldRecover(lastAvailableBlock uint64) (bool, uint64, error) {
	savepoint, err := h.GetLastSavepoint()
	if err != nil {
		return false, 0, err
	}
	if savepoint == nil {
		return true, 0, nil
	}
	return savepoint.BlockNum != lastAvailableBlock, savepoint.BlockNum + 1, nil
}

// CommitLostBlock implements method in interface kvledger.Recoverer
func (h *historyDB) CommitLostBlock(blockAndPvtdata *ledger.BlockAndPvtData) error {
	block := blockAndPvtdata.Block

	// log every 1000th block at Info level so that history rebuild progress can be tracked in production envs.
	if block.Header.Number%1000 == 0 {
		logger.Infof("Recommitting block [%d] to history database", block.Header.Number)
	} else {
		logger.Debugf("Recommitting block [%d] to history database", block.Header.Number)
	}

	if err := h.Commit(block); err != nil {
		return err
	}
	return nil
}
