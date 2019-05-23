/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package state

import (
	"testing"

	"github.com/pkg/errors"

	"github.com/hyperledger/fabric/gossip/discovery"
	"github.com/hyperledger/fabric/gossip/util"
	"github.com/hyperledger/fabric/protos/common"
	proto "github.com/hyperledger/fabric/protos/gossip"
	"github.com/hyperledger/fabric/protos/transientstore"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/stretchr/testify/require"
)

func TestProviderExtension(t *testing.T) {

	predicate := func(peer discovery.NetworkMember) bool {
		return true
	}

	sampleError := errors.New("not implemented")

	handleAddPayload := func(payload *proto.Payload, blockingMode bool) error {
		return sampleError
	}

	handleStoreBlock := func(block *common.Block, pvtData util.PvtDataCollections) error {
		return sampleError
	}

	extension := NewGossipStateProviderExtension("test", &coordinatorMock{}, nil, nil)
	require.Error(t, sampleError, extension.AddPayload(handleAddPayload))
	require.True(t, extension.Predicate(predicate)(discovery.NetworkMember{}))
	require.Error(t, sampleError, extension.StoreBlock(handleStoreBlock))

}

// coordinatorMock mock implementation of LedgerResources
type coordinatorMock struct {
}

func (mock *coordinatorMock) GetPvtDataAndBlockByNum(seqNum uint64, _ protoutil.SignedData) (*common.Block, util.PvtDataCollections, error) {
	return nil, nil, nil
}

func (mock *coordinatorMock) GetBlockByNum(seqNum uint64) (*common.Block, error) {
	return nil, nil
}

func (mock *coordinatorMock) StoreBlock(block *common.Block, data util.PvtDataCollections) error {
	return nil
}

func (mock *coordinatorMock) LedgerHeight() (uint64, error) {
	return 0, nil
}

func (mock *coordinatorMock) Close() {
	return
}

// StorePvtData used to persist private date into transient store
func (mock *coordinatorMock) StorePvtData(txid string, privData *transientstore.TxPvtReadWriteSetWithConfigInfo, blkHeight uint64) error {
	return nil
}
