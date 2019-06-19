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

	extension := NewGossipStateProviderExtension("test", nil, nil)
	require.Error(t, sampleError, extension.AddPayload(handleAddPayload))
	require.True(t, extension.Predicate(predicate)(discovery.NetworkMember{}))
	require.Error(t, sampleError, extension.StoreBlock(handleStoreBlock))

}
