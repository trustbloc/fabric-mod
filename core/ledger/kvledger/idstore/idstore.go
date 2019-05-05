/*
Copyright IBM Corp. 2016 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package idstore

import (
	"bytes"
	"errors"

	"github.com/syndtr/goleveldb/leveldb"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/ledger/util/leveldbhelper"
	"github.com/hyperledger/fabric/protos/common"
)

//////////////////////////////////////////////////////////////////////
// Ledger id persistence related code
///////////////////////////////////////////////////////////////////////

var (
	// ErrLedgerIDExists is thrown by a CreateLedger call if a ledger with the given id already exists
	ErrLedgerIDExists          = errors.New("LedgerID already exists")
	underConstructionLedgerKey = []byte("underConstructionLedgerKey")
	ledgerKeyPrefix            = []byte("l")
)

type IDStore interface {
	SetUnderConstructionFlag(string) error
	UnsetUnderConstructionFlag() error
	GetUnderConstructionFlag() (string, error)
	CreateLedgerID(ledgerID string, gb *common.Block) error
	LedgerIDExists(ledgerID string) (bool, error)
	GetAllLedgerIds() ([]string, error)
	GetLedgeIDValue(ledgerID string) ([]byte, error)
	Close()
}

type idStore struct {
	db *leveldbhelper.DB
}

func OpenIDStore(path string) IDStore {
	db := leveldbhelper.CreateDB(&leveldbhelper.Conf{DBPath: path})
	db.Open()
	return &idStore{db}
}

func (s *idStore) SetUnderConstructionFlag(ledgerID string) error {
	return s.db.Put(underConstructionLedgerKey, []byte(ledgerID), true)
}

func (s *idStore) UnsetUnderConstructionFlag() error {
	return s.db.Delete(underConstructionLedgerKey, true)
}

func (s *idStore) GetUnderConstructionFlag() (string, error) {
	val, err := s.db.Get(underConstructionLedgerKey)
	if err != nil {
		return "", err
	}
	return string(val), nil
}

func (s *idStore) CreateLedgerID(ledgerID string, gb *common.Block) error {
	key := s.encodeLedgerKey(ledgerID)
	var val []byte
	var err error
	if val, err = s.db.Get(key); err != nil {
		return err
	}
	if val != nil {
		return ErrLedgerIDExists
	}
	if val, err = proto.Marshal(gb); err != nil {
		return err
	}
	batch := &leveldb.Batch{}
	batch.Put(key, val)
	batch.Delete(underConstructionLedgerKey)
	return s.db.WriteBatch(batch, true)
}

func (s *idStore) LedgerIDExists(ledgerID string) (bool, error) {
	key := s.encodeLedgerKey(ledgerID)
	val := []byte{}
	err := error(nil)
	if val, err = s.db.Get(key); err != nil {
		return false, err
	}
	return val != nil, nil
}

func (s *idStore) GetAllLedgerIds() ([]string, error) {
	var ids []string
	itr := s.db.GetIterator(nil, nil)
	defer itr.Release()
	itr.First()
	for itr.Valid() {
		if bytes.Equal(itr.Key(), underConstructionLedgerKey) {
			continue
		}
		id := string(s.decodeLedgerID(itr.Key()))
		ids = append(ids, id)
		itr.Next()
	}
	return ids, nil
}

func (s *idStore) GetLedgeIDValue(ledgerID string) ([]byte, error) {
	return s.db.Get(s.encodeLedgerKey(ledgerID))
}

func (s *idStore) Close() {
	s.db.Close()
}

func (s *idStore) encodeLedgerKey(ledgerID string) []byte {
	return append(ledgerKeyPrefix, []byte(ledgerID)...)
}

func (s *idStore) decodeLedgerID(key []byte) string {
	return string(key[len(ledgerKeyPrefix):])
}
