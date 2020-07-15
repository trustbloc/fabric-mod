/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package statecouchdb

import (
	"fmt"
	"testing"

	"github.com/hyperledger/fabric-protos-go/ledger/rwset/kvrwset"
	"github.com/hyperledger/fabric/core/ledger/internal/version"
	"github.com/hyperledger/fabric/extensions/gossip/api"
	"github.com/stretchr/testify/require"
)

func TestDeleteCacheEntry(t *testing.T) {
	vdbEnv.init(t, []string{"lscc", "_lifecycle"})
	defer vdbEnv.cleanup()

	const chainID = "testgetstatefromcache"
	const key = "key1"
	const ns = "ns1"

	db, err := vdbEnv.DBProvider.GetDBHandle(chainID, nil)
	require.NoError(t, err)

	cacheValue := &CacheValue{
		Value:          []byte("value1"),
		Metadata:       []byte("meta1"),
		Version:        version.NewHeight(1, 1).ToBytes(),
		AdditionalInfo: []byte("rev1"),
	}
	require.NoError(t, vdbEnv.cache.putState(chainID, ns, key, cacheValue))

	vdb, ok := db.(*VersionedDB)
	require.True(t, ok)

	v, err := db.GetState(ns, key)
	require.NoError(t, err)
	require.Equal(t, cacheValue.Value, v.Value)
	require.NoError(t, vdb.deleteCacheEntry(api.TxMetadata{}, ns, &kvrwset.KVWrite{Key: key}))

	v, err = db.GetState(ns, key)
	require.NoError(t, err)
	fmt.Printf("%+v\n", v)
	require.Nil(t, v.Value)
}
