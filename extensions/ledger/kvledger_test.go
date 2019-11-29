/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package ledger

import (
	"errors"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/common/ledger/blkstorage"
	"github.com/stretchr/testify/require"
	"testing"
)

const errMsg = "not implemented"

func TestKVLedgerExtension(t *testing.T) {
	xtn := NewKVLedgerExtension(&mockBlockStore{})
	require.NotNil(t, xtn)
	require.Error(t, xtn.CheckpointBlock(nil), errMsg)
}

type mockBlockStore struct {
	blkstorage.BlockStore
}

func (mbs *mockBlockStore) CheckpointBlock(block *common.Block) error {
	return errors.New(errMsg)
}
