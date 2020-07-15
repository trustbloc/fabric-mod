/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package state

import (
	"testing"

	"github.com/hyperledger/fabric-protos-go/common"
	proto "github.com/hyperledger/fabric-protos-go/gossip"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/gossip/discovery"
	"github.com/hyperledger/fabric/gossip/util"
)

func TestProviderExtension(t *testing.T) {

	predicate := func(peer discovery.NetworkMember) bool {
		return true
	}

	sampleError := errors.New("not implemented")

	handleAddPayload := func(payload *proto.Payload, blockingMode bool) error {
		return sampleError
	}

	handleStoreBlock := func(block *common.Block, pvtData util.PvtDataCollections) (*ledger.BlockAndPvtData, error) {
		return nil, sampleError
	}

	handleLedgerheight := func() (uint64, error) {
		return 99, nil
	}

	extension := NewGossipStateProviderExtension("test", nil, nil, false)
	require.Error(t, sampleError, extension.AddPayload(handleAddPayload))
	require.True(t, extension.Predicate(predicate)(discovery.NetworkMember{}))
	_, err := extension.StoreBlock(handleStoreBlock)(&common.Block{}, nil)
	require.Error(t, err, sampleError)
	height, err := extension.LedgerHeight(handleLedgerheight)()
	require.Equal(t, 99, int(height))
	require.NoError(t, err)
}
