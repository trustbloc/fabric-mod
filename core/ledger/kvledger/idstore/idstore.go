/*
Copyright IBM Corp. 2016 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package idstore

import (
	"bytes"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/common/ledger/dataformat"
	"github.com/hyperledger/fabric/common/ledger/util/leveldbhelper"
	"github.com/hyperledger/fabric/core/ledger/kvledger/msgs"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
	"github.com/syndtr/goleveldb/leveldb"
)

var logger = flogging.MustGetLogger("idstore")

//////////////////////////////////////////////////////////////////////
// Ledger id persistence related code
///////////////////////////////////////////////////////////////////////

var (
	// ErrLedgerIDExists is thrown by a CreateLedger call if a ledger with the given id already exists
	ErrLedgerIDExists      = errors.New("LedgerID already exists")
	ErrNonExistingLedgerID = errors.New("LedgerID does not exist")

	underConstructionLedgerKey = []byte("underConstructionLedgerKey")
	ledgerKeyPrefix            = []byte("l")
	// ledgerKeyStop is the end key when querying IDStore db by ledger key
	ledgerKeyStop = []byte{'l' + 1}
	// metadataKeyPrefix is the prefix for each metadata key in IDStore db
	metadataKeyPrefix = []byte{'s'}
	// metadataKeyStop is the end key when querying IDStore db by metadata key
	metadataKeyStop = []byte{'s' + 1}
	formatKey       = []byte("f")
)

//////////////////////////////////////////////////////////////////////
// Ledger id persistence related code
///////////////////////////////////////////////////////////////////////
type IDStore struct {
	db     *leveldbhelper.DB
	dbPath string
}

func NewIDStoreWithLevelDB(path string, db *leveldbhelper.DB) *IDStore {
	return &IDStore{dbPath: path, db: db}
}

func OpenIDStore(path string) (s *IDStore, e error) {
	db := leveldbhelper.CreateDB(&leveldbhelper.Conf{DBPath: path})
	db.Open()
	defer func() {
		if e != nil {
			db.Close()
		}
	}()

	emptyDB, err := db.IsEmpty()
	if err != nil {
		return nil, err
	}

	expectedFormatBytes := []byte(dataformat.Version20)
	if emptyDB {
		// add format key to a new db
		err := db.Put(formatKey, expectedFormatBytes, true)
		if err != nil {
			return nil, err
		}
		return &IDStore{db, path}, nil
	}

	// verify the format is current for an existing db
	formatVersion, err := db.Get(formatKey)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(formatVersion, expectedFormatBytes) {
		logger.Errorf("The db at path [%s] contains data in unexpected format. expected data format = [%s] (%#v), data format = [%s] (%#v).",
			path, dataformat.Version20, expectedFormatBytes, formatVersion, formatVersion)
		return nil, &dataformat.ErrVersionMismatch{
			ExpectedVersion: dataformat.Version20,
			Version:         string(formatVersion),
			DBInfo:          fmt.Sprintf("leveldb at [%s]", path),
		}
	}
	return &IDStore{db, path}, nil
}

// GetFormat returns the database format
func (s *IDStore) GetFormat() ([]byte, error) {
	return s.db.Get(formatKey)
}

func (s *IDStore) UpgradeFormat() error {
	format, err := s.db.Get(formatKey)
	if err != nil {
		return err
	}
	idStoreFormatBytes := []byte(dataformat.Version20)
	if bytes.Equal(format, idStoreFormatBytes) {
		logger.Debug("Format is current, nothing to do")
		return nil
	}
	if format != nil {
		err = &dataformat.ErrVersionMismatch{
			ExpectedVersion: "",
			Version:         string(format),
			DBInfo:          fmt.Sprintf("leveldb at [%s]", s.dbPath),
		}
		logger.Errorf("Failed to upgrade format [%#v] to new format [%#v]: %s", format, idStoreFormatBytes, err)
		return err
	}

	logger.Infof("The ledgerProvider db format is old, upgrading to the new format %s", dataformat.Version20)

	batch := &leveldb.Batch{}
	batch.Put(formatKey, idStoreFormatBytes)

	// add new metadata key for each ledger (channel)
	metadata, err := protoutil.Marshal(&msgs.LedgerMetadata{Status: msgs.Status_ACTIVE})
	if err != nil {
		logger.Errorf("Error marshalling ledger metadata: %s", err)
		return errors.Wrapf(err, "error marshalling ledger metadata")
	}
	itr := s.db.GetIterator(ledgerKeyPrefix, ledgerKeyStop)
	defer itr.Release()
	for itr.Error() == nil && itr.Next() {
		id := s.DecodeLedgerID(itr.Key(), ledgerKeyPrefix)
		batch.Put(s.EncodeLedgerKey(id, metadataKeyPrefix), metadata)
	}
	if err = itr.Error(); err != nil {
		logger.Errorf("Error while upgrading IDStore format: %s", err)
		return errors.Wrapf(err, "error while upgrading IDStore format")
	}

	return s.db.WriteBatch(batch, true)
}

func (s *IDStore) SetUnderConstructionFlag(ledgerID string) error {
	return s.db.Put(underConstructionLedgerKey, []byte(ledgerID), true)
}

func (s *IDStore) UnsetUnderConstructionFlag() error {
	return s.db.Delete(underConstructionLedgerKey, true)
}

func (s *IDStore) GetUnderConstructionFlag() (string, error) {
	val, err := s.db.Get(underConstructionLedgerKey)
	if err != nil {
		return "", err
	}
	return string(val), nil
}

func (s *IDStore) CreateLedgerID(ledgerID string, gb *common.Block) error {
	gbKey := s.EncodeLedgerKey(ledgerID, ledgerKeyPrefix)
	metadataKey := s.EncodeLedgerKey(ledgerID, metadataKeyPrefix)
	var val []byte
	var metadata []byte
	var err error
	if val, err = s.db.Get(gbKey); err != nil {
		return err
	}
	if val != nil {
		return ErrLedgerIDExists
	}
	if val, err = proto.Marshal(gb); err != nil {
		return err
	}
	if metadata, err = protoutil.Marshal(&msgs.LedgerMetadata{Status: msgs.Status_ACTIVE}); err != nil {
		return err
	}
	batch := &leveldb.Batch{}
	batch.Put(gbKey, val)
	batch.Put(metadataKey, metadata)
	batch.Delete(underConstructionLedgerKey)
	return s.db.WriteBatch(batch, true)
}

func (s *IDStore) UpdateLedgerStatus(ledgerID string, newStatus msgs.Status) error {
	metadata, err := s.getLedgerMetadata(ledgerID)
	if err != nil {
		return err
	}
	if metadata == nil {
		logger.Errorf("LedgerID [%s] does not exist", ledgerID)
		return ErrNonExistingLedgerID
	}
	if metadata.Status == newStatus {
		logger.Infof("Ledger [%s] is already in [%s] status, nothing to do", ledgerID, newStatus)
		return nil
	}
	metadata.Status = newStatus
	metadataBytes, err := proto.Marshal(metadata)
	if err != nil {
		logger.Errorf("Error marshalling ledger metadata: %s", err)
		return errors.Wrapf(err, "error marshalling ledger metadata")
	}
	logger.Infof("Updating ledger [%s] status to [%s]", ledgerID, newStatus)
	key := s.EncodeLedgerKey(ledgerID, metadataKeyPrefix)
	return s.db.Put(key, metadataBytes, true)
}

func (s *IDStore) getLedgerMetadata(ledgerID string) (*msgs.LedgerMetadata, error) {
	val, err := s.db.Get(s.EncodeLedgerKey(ledgerID, metadataKeyPrefix))
	if val == nil || err != nil {
		return nil, err
	}
	metadata := &msgs.LedgerMetadata{}
	if err := proto.Unmarshal(val, metadata); err != nil {
		logger.Errorf("Error unmarshalling ledger metadata: %s", err)
		return nil, errors.Wrapf(err, "error unmarshalling ledger metadata")
	}
	return metadata, nil
}

func (s *IDStore) LedgerIDExists(ledgerID string) (bool, error) {
	key := s.EncodeLedgerKey(ledgerID, ledgerKeyPrefix)
	val := []byte{}
	err := error(nil)
	if val, err = s.db.Get(key); err != nil {
		return false, err
	}
	return val != nil, nil
}

// ledgerIDActive returns if a ledger is active and existed
func (s *IDStore) LedgerIDActive(ledgerID string) (bool, bool, error) {
	metadata, err := s.getLedgerMetadata(ledgerID)
	if metadata == nil || err != nil {
		return false, false, err
	}
	return metadata.Status == msgs.Status_ACTIVE, true, nil
}

func (s *IDStore) GetActiveLedgerIDs() ([]string, error) {
	var ids []string
	itr := s.db.GetIterator(metadataKeyPrefix, metadataKeyStop)
	defer itr.Release()
	for itr.Error() == nil && itr.Next() {
		metadata := &msgs.LedgerMetadata{}
		if err := proto.Unmarshal(itr.Value(), metadata); err != nil {
			logger.Errorf("Error unmarshalling ledger metadata: %s", err)
			return nil, errors.Wrapf(err, "error unmarshalling ledger metadata")
		}
		if metadata.Status == msgs.Status_ACTIVE {
			id := s.DecodeLedgerID(itr.Key(), metadataKeyPrefix)
			ids = append(ids, id)
		}
	}
	if err := itr.Error(); err != nil {
		logger.Errorf("Error getting ledger ids from idStore: %s", err)
		return nil, errors.Wrapf(err, "error getting ledger ids from idStore")
	}
	return ids, nil
}

// GetGenesisBlock returns the genesis block for the given ledger ID
func (s *IDStore) GetGenesisBlock(ledgerID string) (*common.Block, error) {
	bytes, err := s.db.Get(s.EncodeLedgerKey(ledgerID, ledgerKeyPrefix))
	if err != nil {
		return nil, err
	}

	b := &common.Block{}
	err = proto.Unmarshal(bytes, b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (s *IDStore) Close() {
	s.db.Close()
}

func (s *IDStore) EncodeLedgerKey(ledgerID string, prefix []byte) []byte {
	return append(prefix, []byte(ledgerID)...)
}

func (s *IDStore) DecodeLedgerID(key []byte, prefix []byte) string {
	return string(key[len(prefix):])
}
