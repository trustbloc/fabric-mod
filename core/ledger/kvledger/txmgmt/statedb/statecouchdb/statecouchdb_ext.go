/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package statecouchdb

import (
	"encoding/base64"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/ledger/rwset/kvrwset"
	"github.com/pkg/errors"

	"github.com/hyperledger/fabric/core/ledger/internal/version"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/statedb"
	"github.com/hyperledger/fabric/core/ledger/util"
	"github.com/hyperledger/fabric/extensions/gossip/api"
	"github.com/hyperledger/fabric/extensions/gossip/state"
)

const (
	pvtDataDelimiter     = "$$p"
	pvtDataHashDelimiter = "$$h"
)

// deleteCacheEntryIfStale deletes the cache entry for the given KV write if it is determined to be stale (i.e. the cached
// version is less than the current version) so that it may be refreshed from the database
func (vdb *VersionedDB) deleteCacheEntryIfStale(metadata api.TxMetadata, namespace string, write *kvrwset.KVWrite) error {
	logger.Debugf("[%s] Checking cache entry for [%s:%s] in block [%d] and TxID [%s]", vdb.chainName, namespace, write.Key, metadata.BlockNum, metadata.TxID)

	return vdb.checkCacheEntry(metadata, namespace, write.Key, write.IsDelete)
}

// deleteCollHashCacheEntryIfStale deletes the cache entry for the given collection hash write if it is determined to be stale
// (i.e. the cached version is less than the current version)so that it may be refreshed from the database
func (vdb *VersionedDB) deleteCollHashCacheEntryIfStale(metadata api.TxMetadata, namespace string, collection string, write *kvrwset.KVWriteHash) error {
	ns := privateDataHashDBName(namespace, collection)
	key := base64.StdEncoding.EncodeToString(write.KeyHash)

	logger.Debugf("[%s] Checking cache entry for hashed key [%s:%s] in block [%d] and TxID [%s]", vdb.chainName, ns, key, metadata.BlockNum, metadata.TxID)

	return vdb.checkCacheEntry(metadata, ns, key, write.IsDelete)
}

func (vdb *VersionedDB) checkCacheEntry(metadata api.TxMetadata, namespace, key string, isDelete bool) error {
	cacheEnabled := vdb.cache.enabled(namespace)
	if !cacheEnabled {
		return nil
	}

	if isDelete {
		logger.Debugf("[%s] Deleting cache entry for [%s:%s] since it has been deleted from the ledger at [%d:%d]",
			vdb.chainName, namespace, key, metadata.BlockNum, metadata.TxNum)

		return vdb.cache.DelState(vdb.chainName, namespace, key)
	}

	cv, err := vdb.cache.getState(vdb.chainName, namespace, key)
	if err != nil {
		logger.Errorf("[%s] Error getting cache entry for [%s:%s] in block [%d] and TxID [%s]: %s", vdb.chainName, namespace, key, metadata.BlockNum, metadata.TxID, err)

		return err
	}

	if cv == nil {
		logger.Debugf("[%s] Key not cached for [%s:%s] in block [%d] and TxID [%s]", vdb.chainName, namespace, key, metadata.BlockNum, metadata.TxID)

		return nil
	}

	var vv *statedb.VersionedValue
	vv, err = constructVersionedValue(cv)
	if err != nil {
		logger.Errorf("[%s] Error constructing versioned value for [%s:%s] in block [%d] and TxID [%s]: %s", vdb.chainName, namespace, key, metadata.BlockNum, metadata.TxID, err)

		return err
	}

	if vv.Version.BlockNum < metadata.BlockNum || vv.Version.TxNum < metadata.TxNum {
		logger.Debugf("[%s] Deleting cache entry for [%s:%s] since its version [%d:%d] is less than the current version [%d:%d]",
			vdb.chainName, namespace, key, vv.Version.BlockNum, vv.Version.TxNum, metadata.BlockNum, metadata.TxNum)

		return vdb.cache.DelState(vdb.chainName, namespace, key)
	}

	logger.Debugf("[%s] Not deleting cache entry [%s:%s] since its version [%d:%d] is greater than or equal to the current version [%d:%d]",
		vdb.chainName, namespace, key, vv.Version.BlockNum, vv.Version.TxNum, metadata.BlockNum, metadata.TxNum)

	return nil
}

// ensureKeyHashVersionMatches checks the version on the given private data key and ensures the version of the
// corresponding key hash matches. If so, the original versioned value is returned, otherwise nil is returned
// and the private data value will need to be refreshed from the database.
func (vdb *VersionedDB) ensureKeyHashVersionMatches(namespace, key string, vv *statedb.VersionedValue) (*statedb.VersionedValue, error) {
	nsAndColl := strings.Split(namespace, pvtDataDelimiter)
	if len(nsAndColl) < 2 {
		return vv, nil
	}

	ns := privateDataHashDBName(nsAndColl[0], nsAndColl[1])

	logger.Debugf("[%s] Looking for corresponding key hash for [%s:%s] ...", vdb.chainName, ns, key)

	keyHash := base64.StdEncoding.EncodeToString(util.ComputeStringHash(key))
	hcv, err := vdb.cache.getState(vdb.chainName, ns, keyHash)
	if err != nil {
		return nil, err
	}

	if vv == nil && hcv == nil {
		logger.Debugf("[%s] Neither key hash nor key for [%s:%s] was found in cache.", vdb.chainName, ns, key)

		return nil, nil
	}

	if vv == nil {
		logger.Debugf("[%s] Key hash for [%s:%s] was found in cache but key was not in cache. Deleting key hash from cache", vdb.chainName, ns, key)

		return nil, vdb.cache.DelState(vdb.chainName, ns, keyHash)
	}

	var hvv *statedb.VersionedValue
	if hcv == nil {
		logger.Infof("[%s] Key hash for [%s:%s] not found in cache. Will read key from database", vdb.chainName, ns, key)

		kv, err := vdb.readFromDB(ns, keyHash)
		if err != nil {
			return nil, err
		}

		if kv == nil {
			logger.Infof("[%s] Key hash for [%s:%s] not found in database.", vdb.chainName, ns, key)

			return vv, nil
		}

		logger.Debugf("[%s] Caching key hash for [%s:%s] - Version %s", vdb.chainName, ns, key, kv.Version)

		err = vdb.cache.putState(vdb.chainName, ns, keyHash, constructCacheValue(kv.VersionedValue, kv.revision))
		if err != nil {
			return nil, err
		}

		hvv = kv.VersionedValue
	} else {
		hvv, err = constructVersionedValue(hcv)
		if err != nil {
			return nil, err
		}

		logger.Debugf("[%s] Key hash for [%s:%s] was found in cache - Version: %s", vdb.chainName, ns, key, hvv.Version)
	}

	if !version.AreSame(hvv.Version, vv.Version) {
		logger.Infof("[%s] Key hash version %s for [%s:%s] does not match the key version in cache %s. Deleting key hash from cache.", vdb.chainName, hvv.Version, ns, key, vv.Version)

		return nil, vdb.cache.DelState(vdb.chainName, ns, keyHash)
	}

	logger.Debugf("[%s] Key hash version %s for [%s:%s] matches the key version in cache %s.", vdb.chainName, hvv.Version, ns, key, vv.Version)

	return vv, nil
}

// UpdateCache updates the state cache with the given cache updates
func (vdb *VersionedDB) UpdateCache(blockNum uint64, updates []byte) error {
	logger.Infof("[%s] Updating state cache for block [%d]", vdb.chainName, blockNum)

	envelope := &CacheUpdatesEnvelope{}
	if err := proto.Unmarshal(updates, envelope); err != nil {
		return errors.WithMessagef(err, "unable to unmarshal cache-updates")
	}

	return vdb.cache.UpdateStates(vdb.chainName, fromCacheUpdatesEnvelope(envelope))
}

func (vdb *VersionedDB) saveCacheUpdates(height *version.Height, updates cacheUpdates) {
	if height == nil || !vdb.storeCacheUpdates {
		// Nothing to do
		return
	}

	if updateBytes, err := proto.Marshal(toCacheUpdatesEnvelope(updates)); err != nil {
		logger.Errorf("[%s] Error marshalling cache updates for block %d: %s", vdb.chainName, height.BlockNum, err)
	} else {
		state.SaveCacheUpdates(vdb.chainName, height.BlockNum, updateBytes)
	}
}

func privateDataHashDBName(namespace, collection string) string {
	return strings.Join([]string{namespace, collection}, pvtDataHashDelimiter)
}

func toCacheUpdatesEnvelope(updates cacheUpdates) *CacheUpdatesEnvelope {
	envelope := &CacheUpdatesEnvelope{
		Updates: make(map[string]*CacheKeyValues),
	}

	for key, values := range updates {
		envelope.Updates[key] = &CacheKeyValues{
			Values: values,
		}
	}

	return envelope
}

func fromCacheUpdatesEnvelope(envelope *CacheUpdatesEnvelope) cacheUpdates {
	updates := make(cacheUpdates)
	for key, values := range envelope.Updates {
		updates[key] = values.Values
	}

	return updates
}
