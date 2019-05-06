/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package pvtdatahandler

import (
	"testing"

	"github.com/hyperledger/fabric/protos/common"
	"github.com/stretchr/testify/assert"
)

func TestHandler_HandleGetPrivateData(t *testing.T) {
	h := New("testchannel", nil)

	config := &common.StaticCollectionConfig{
		Name: "coll1",
		Type: common.CollectionType_COL_TRANSIENT,
	}

	value, handled, err := h.HandleGetPrivateData("tx1", "ns1", config, "key1")
	assert.NoError(t, err)
	assert.False(t, handled)
	assert.Nil(t, value)
}

func TestHandler_HandleGetPrivateDataMultipleKeys(t *testing.T) {
	h := New("testchannel", nil)

	config := &common.StaticCollectionConfig{
		Name: "coll1",
		Type: common.CollectionType_COL_TRANSIENT,
	}

	value, handled, err := h.HandleGetPrivateDataMultipleKeys("tx1", "ns1", config, []string{"key1", "key2"})
	assert.NoError(t, err)
	assert.False(t, handled)
	assert.Nil(t, value)
}
