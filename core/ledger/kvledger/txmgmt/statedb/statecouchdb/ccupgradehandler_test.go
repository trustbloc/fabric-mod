/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package statecouchdb

import (
	"fmt"
	"testing"

	"github.com/hyperledger/fabric/extensions/gossip/api"
	"github.com/stretchr/testify/require"
)

const (
	ccID1 = "samplecc"
	ccID2 = "samplecc2"
	coll  = "collection"
)

var (
	ccID1Coll1 = fmt.Sprintf("%s~%s", ccID1, coll)
	ccID2Coll1 = fmt.Sprintf("%s~%s", ccID2, coll)
)

func TestCCUpgradeHandler(t *testing.T) {
	vdbEnv.init(t, nil)
	defer vdbEnv.cleanup()

	db, err := vdbEnv.DBProvider.GetDBHandle("test-cc-upgrade", nil)
	require.NoError(t, err)

	channelID := db.(*VersionedDB).chainName

	cache := db.(*VersionedDB).cache
	require.NoError(t, cache.putState(channelID, lsccNamespace, ccID1, &CacheValue{Value: []byte(ccID1)}))
	require.NoError(t, cache.putState(channelID, lsccNamespace, ccID1Coll1, &CacheValue{Value: []byte(ccID1Coll1)}))
	require.NoError(t, cache.putState(channelID, lsccNamespace, ccID2, &CacheValue{Value: []byte(ccID2)}))
	require.NoError(t, cache.putState(channelID, lsccNamespace, ccID2Coll1, &CacheValue{Value: []byte(ccID2Coll1)}))

	v, err := cache.getState(channelID, lsccNamespace, ccID1)
	require.NoError(t, err)
	require.NotNil(t, v)

	v, err = cache.getState(channelID, lsccNamespace, ccID1Coll1)
	require.NoError(t, err)
	require.NotNil(t, v)

	v, err = cache.getState(channelID, lsccNamespace, ccID2)
	require.NoError(t, err)
	require.NotNil(t, v)

	v, err = cache.getState(channelID, lsccNamespace, ccID2Coll1)
	require.NoError(t, err)
	require.NotNil(t, v)

	handler := getCCUpgradeHandler(db.(*VersionedDB))
	txnMetadata := api.TxMetadata{}

	err = handler(txnMetadata, ccID1)
	require.NoError(t, err)

	v, err = cache.getState(channelID, lsccNamespace, ccID1)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = cache.getState(channelID, lsccNamespace, ccID1Coll1)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = cache.getState(channelID, lsccNamespace, ccID2)
	require.NoError(t, err)
	require.NotNil(t, v)

	v, err = cache.getState(channelID, lsccNamespace, ccID2Coll1)
	require.NoError(t, err)
	require.NotNil(t, v)

	err = handler(txnMetadata, ccID2)
	require.NoError(t, err)

	v, err = cache.getState(channelID, lsccNamespace, ccID2)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = cache.getState(channelID, lsccNamespace, ccID2Coll1)
	require.NoError(t, err)
	require.Nil(t, v)
}
