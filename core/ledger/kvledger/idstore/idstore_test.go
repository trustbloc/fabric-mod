/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package idstore

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/stretchr/testify/require"
)

func TestUnderConstructionFlag(t *testing.T) {
	ledgerID := "testunderconstructionglag"
	env := NewTestStoreEnv(t)
	req := require.New(t)
	store := env.TestStore

	// set under construction flag
	req.NoError(store.SetUnderConstructionFlag(ledgerID))

	// get under construction flag should exist
	value, err := store.GetUnderConstructionFlag()
	req.NoError(err)
	req.Equal(ledgerID, value)

	// unset under construction flag
	req.NoError(store.UnsetUnderConstructionFlag())

	// get under construction flag should not exist after unset
	value, err = store.GetUnderConstructionFlag()
	req.NoError(err)
	req.Empty(value)

}

func TestLedgerID(t *testing.T) {
	ledgerID := "testledgerid"
	ledgerID1 := "testledgerid1"
	env := NewTestStoreEnv(t)
	req := require.New(t)
	store := env.TestStore

	block := &common.Block{}
	block.Data = &common.BlockData{Data: [][]byte{[]byte("testblock")}}

	// create ledger id
	req.NoError(store.CreateLedgerID(ledgerID, block))
	req.NoError(store.CreateLedgerID(ledgerID1, block))

	// create exist ledger id should fail
	req.Error(store.CreateLedgerID(ledgerID, block))

	// get ledger ids
	ledgerIDs, err := store.GetAllLedgerIds()
	req.NoError(err)
	req.Equal(2, len(ledgerIDs))
	req.Contains(ledgerIDs, ledgerID)
	req.Contains(ledgerIDs, ledgerID1)

	// get ledger id value
	ledgerIdValue, err := store.GetLedgeIDValue(ledgerID)
	req.NoError(err)
	gb := &common.Block{}
	req.NoError(proto.Unmarshal(ledgerIdValue, gb))
	req.Equal("testblock", string(gb.Data.Data[0]))

	//check ledger id exist
	exist, err := store.LedgerIDExists(ledgerID)
	req.NoError(err)
	req.Equal(exist, true)

}
