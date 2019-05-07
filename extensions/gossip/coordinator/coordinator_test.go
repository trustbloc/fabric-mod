/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package coordinator

import (
	"testing"

	"github.com/hyperledger/fabric/extensions/mocks"
	"github.com/hyperledger/fabric/protos/transientstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoordinator_StorePvtData(t *testing.T) {
	tStore := &mockTransientStore{}
	collStore := mocks.NewDataStore()

	c := New("testchannel", tStore, collStore)
	require.NotNil(t, c)
	err := c.StorePvtData("tx1", &transientstore.TxPvtReadWriteSetWithConfigInfo{}, 1000)
	assert.NoError(t, err)
}

type mockTransientStore struct {
}

func (m *mockTransientStore) PersistWithConfig(txid string, blockHeight uint64, privateSimulationResultsWithConfig *transientstore.TxPvtReadWriteSetWithConfigInfo) error {
	return nil
}
