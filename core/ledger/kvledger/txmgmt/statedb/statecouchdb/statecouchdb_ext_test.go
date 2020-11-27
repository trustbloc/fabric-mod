/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package statecouchdb

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/ledger/rwset/kvrwset"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric/core/ledger/internal/version"
	"github.com/hyperledger/fabric/core/ledger/util"
	"github.com/hyperledger/fabric/extensions/gossip/api"
)

func TestDeleteCacheEntryIfStale(t *testing.T) {
	vdbEnv.init(t, []string{"lscc", "_lifecycle"})
	defer vdbEnv.cleanup()

	const chainID = "testgetstatefromcache"
	const key = "key1"
	const ns = "ns1"
	const coll = "coll1"

	db, err := vdbEnv.DBProvider.GetDBHandle(chainID, nil)
	require.NoError(t, err)

	cacheValue := &CacheValue{
		Value:          []byte("value1"),
		Metadata:       []byte("meta1"),
		Version:        version.NewHeight(1, 1).ToBytes(),
		AdditionalInfo: []byte("rev1"),
	}
	require.NoError(t, vdbEnv.cache.putState(chainID, ns, key, cacheValue))

	collNs := privateDataHashDBName(ns, coll)
	keyHash := []byte("coll key hash")
	collKey := base64.StdEncoding.EncodeToString(keyHash)
	require.NoError(t, vdbEnv.cache.putState(chainID, collNs, collKey, cacheValue))

	vdb, ok := db.(*VersionedDB)
	require.True(t, ok)

	v, err := db.GetState(ns, key)
	require.NoError(t, err)
	require.Equal(t, cacheValue.Value, v.Value)

	require.NoError(t, vdb.deleteCacheEntryIfStale(api.TxMetadata{BlockNum: 1, TxNum: 1}, ns, &kvrwset.KVWrite{Key: key}))
	require.NoError(t, vdb.deleteCollHashCacheEntryIfStale(api.TxMetadata{BlockNum: 1, TxNum: 1}, ns, coll, &kvrwset.KVWriteHash{KeyHash: keyHash}))

	v, err = db.GetState(ns, key)
	require.NoError(t, err)
	require.NotNil(t, v)

	v, err = db.GetState(collNs, collKey)
	require.NoError(t, err)
	require.NotNil(t, v)

	require.NoError(t, vdb.deleteCacheEntryIfStale(api.TxMetadata{BlockNum: 2, TxNum: 1}, ns, &kvrwset.KVWrite{Key: key}))
	require.NoError(t, vdb.deleteCollHashCacheEntryIfStale(api.TxMetadata{BlockNum: 1, TxNum: 2}, ns, coll, &kvrwset.KVWriteHash{KeyHash: keyHash}))

	v, err = db.GetState(ns, key)
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = db.GetState(collNs, collKey)
	require.NoError(t, err)
	require.Nil(t, v)
}

func TestEnsureKeyHashVersionMatches(t *testing.T) {
	const (
		chainID        = "testensurekeyhashmatches"
		ns1            = "ns1"
		coll1          = "coll1"
		block1  uint64 = 1000
		tx1     uint64 = 0
		tx2     uint64 = 1
		key            = "key1"
	)

	vdbEnv.init(t, []string{"lscc", "_lifecycle"})
	defer vdbEnv.cleanup()

	db, err := vdbEnv.DBProvider.GetDBHandle(chainID, nil)
	require.NoError(t, err)

	pvtns1 := privateDataDBName(ns1, coll1)
	hpvtns1 := privateDataHashDBName(ns1, coll1)

	vdb, ok := db.(*VersionedDB)
	require.True(t, ok)

	t.Run("Both cached and versions match", func(t *testing.T) {
		require.NoError(t, vdbEnv.cache.putState(chainID, hpvtns1, newHashKey(key), newCacheValue(block1, tx1)))

		vv, err := constructVersionedValue(newCacheValue(block1, tx1))
		require.NoError(t, err)

		vv2, err := vdb.ensureKeyHashVersionMatches(pvtns1, key, vv)
		require.NoError(t, err)
		require.Equal(t, vv, vv2)
	})

	t.Run("Both cached but versions don't match", func(t *testing.T) {
		require.NoError(t, vdbEnv.cache.putState(chainID, hpvtns1, newHashKey(key), newCacheValue(block1, tx2)))

		vv, err := constructVersionedValue(newCacheValue(block1, tx1))
		require.NoError(t, err)

		vv2, err := vdb.ensureKeyHashVersionMatches(pvtns1, key, vv)
		require.NoError(t, err)
		require.Nil(t, vv2)
	})

	t.Run("Not a private collection", func(t *testing.T) {
		vv, err := constructVersionedValue(newCacheValue(block1, tx1))
		require.NoError(t, err)

		vv2, err := vdb.ensureKeyHashVersionMatches("ns1", key, vv)
		require.NoError(t, err)
		require.Equal(t, vv, vv2)
	})

	t.Run("Neither key cached", func(t *testing.T) {
		vv2, err := vdb.ensureKeyHashVersionMatches(pvtns1, key, nil)
		require.NoError(t, err)
		require.Nil(t, vv2)
	})

	t.Run("Key not cached but key hash cached", func(t *testing.T) {
		require.NoError(t, vdbEnv.cache.putState(chainID, hpvtns1, newHashKey(key), newCacheValue(block1, tx1)))

		vv2, err := vdb.ensureKeyHashVersionMatches(pvtns1, key, nil)
		require.NoError(t, err)
		require.Nil(t, vv2)
	})

	t.Run("Key cached but key hash not cached", func(t *testing.T) {
		vv, err := constructVersionedValue(newCacheValue(block1, tx1))
		require.NoError(t, err)

		vv2, err := vdb.ensureKeyHashVersionMatches(pvtns1, key, vv)
		require.NoError(t, err)
		require.Equal(t, vv, vv2)
	})
}

func TestVersionedDB_UpdateCache(t *testing.T) {
	const (
		ns1 = "ns1"
		key = "key1"
	)

	vdb := &VersionedDB{
		cache: newCache(10, nil),
	}

	t.Run("UpdateCache", func(t *testing.T) {
		updates := cacheUpdates{ns1: cacheKVs{key: &CacheValue{}}}
		updateBytes, err := proto.Marshal(toCacheUpdatesEnvelope(updates))
		require.NoError(t, err)

		require.NoError(t, vdb.UpdateCache(1001, updateBytes))
	})

	t.Run("SaveCacheUpdates", func(t *testing.T) {
		updates := cacheUpdates{ns1: cacheKVs{key: &CacheValue{}}}
		require.NotPanics(t, func() { vdb.saveCacheUpdates(&version.Height{BlockNum: 1001}, updates) })
		require.NotPanics(t, func() { vdb.saveCacheUpdates(nil, updates) })
	})
}

func privateDataDBName(namespace, collection string) string {
	return strings.Join([]string{namespace, collection}, pvtDataDelimiter)
}

func newCacheValue(blockNum, txNum uint64) *CacheValue {
	return &CacheValue{Version: version.NewHeight(blockNum, txNum).ToBytes()}
}

func newHashKey(key string) string {
	return base64.StdEncoding.EncodeToString(util.ComputeStringHash(key))
}
