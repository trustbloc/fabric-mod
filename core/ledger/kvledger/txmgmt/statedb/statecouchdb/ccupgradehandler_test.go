/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package statecouchdb

import (
	"fmt"
	"testing"

	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/statedb"
	"github.com/hyperledger/fabric/extensions/gossip/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	channelID = "testchannel"
	ccID1     = "samplecc"
	ccID2     = "samplecc2"
	coll      = "collection"
)

var (
	ccID1Coll1 = fmt.Sprintf("%s~%s", ccID1, coll)
	ccID2Coll1 = fmt.Sprintf("%s~%s", ccID2, coll)
)

func TestCCUpgradeHandler(t *testing.T) {
	testEnv.init(t, statedb.NewCache(1, []string{lsccNamespace}))
	defer testEnv.cleanup()

	db, err := testEnv.DBProvider.GetDBHandle(channelID)
	assert.NoError(t, err)
	db.Open()
	defer db.Close()

	cache := db.(*VersionedDB).cache
	require.NoError(t, cache.PutState(channelID, lsccNamespace, ccID1, &statedb.CacheValue{Value: []byte(ccID1)}))
	require.NoError(t, cache.PutState(channelID, lsccNamespace, ccID1Coll1, &statedb.CacheValue{Value: []byte(ccID1Coll1)}))
	require.NoError(t, cache.PutState(channelID, lsccNamespace, ccID2, &statedb.CacheValue{Value: []byte(ccID2)}))
	require.NoError(t, cache.PutState(channelID, lsccNamespace, ccID2Coll1, &statedb.CacheValue{Value: []byte(ccID2Coll1)}))

	v, err := cache.GetState(channelID, lsccNamespace, ccID1)
	require.NoError(t, err)
	require.NotNil(t, v)

	v, err = cache.GetState(channelID, lsccNamespace, ccID1Coll1)
	require.NoError(t, err)
	require.NotNil(t, v)

	v, err = cache.GetState(channelID, lsccNamespace, ccID2)
	require.NoError(t, err)
	require.NotNil(t, v)

	v, err = cache.GetState(channelID, lsccNamespace, ccID2Coll1)
	require.NoError(t, err)
	require.NotNil(t, v)

	handler := getCCUpgradeHandler(db.(*VersionedDB))
	txnMetadata := api.TxMetadata{}

	err = handler(txnMetadata, ccID1)
	require.NoError(t, err)

	v, err = cache.GetState(channelID, lsccNamespace, ccID1)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = cache.GetState(channelID, lsccNamespace, ccID1Coll1)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = cache.GetState(channelID, lsccNamespace, ccID2)
	require.NoError(t, err)
	require.NotNil(t, v)

	v, err = cache.GetState(channelID, lsccNamespace, ccID2Coll1)
	require.NoError(t, err)
	require.NotNil(t, v)

	err = handler(txnMetadata, ccID2)
	require.NoError(t, err)

	v, err = cache.GetState(channelID, lsccNamespace, ccID2)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = cache.GetState(channelID, lsccNamespace, ccID2Coll1)
	require.NoError(t, err)
	require.Nil(t, v)
}
