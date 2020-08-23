/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package statecouchdb

import (
	"encoding/base64"
	"strings"

	"github.com/hyperledger/fabric-protos-go/ledger/rwset/kvrwset"
	"github.com/hyperledger/fabric/core/ledger/internal/version"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/statedb"
	"github.com/hyperledger/fabric/core/ledger/util"
	"github.com/hyperledger/fabric/extensions/gossip/api"
)

const (
	pvtDataDelimiter     = "$$p"
	pvtDataHashDelimiter = "$$h"
)

// deleteCacheEntry deletes the cache entry for the given KV write so that it may be refreshed from the database
func (vdb *VersionedDB) deleteCacheEntry(metadata api.TxMetadata, namespace string, write *kvrwset.KVWrite) error {
	logger.Debugf("[%s] Deleting cache entry for [%s:%s] in block [%d] and TxID [%s]", vdb.chainName, namespace, write.Key, metadata.BlockNum, metadata.TxID)

	return vdb.cache.DelState(vdb.chainName, namespace, write.Key)
}

// deleteCollHashCacheEntry deletes the cache entry for the given collection hash write so that it may be refreshed from the database
func (vdb *VersionedDB) deleteCollHashCacheEntry(metadata api.TxMetadata, namespace string, collection string, write *kvrwset.KVWriteHash) error {
	ns := privateDataHashDBName(namespace, collection)
	key := base64.StdEncoding.EncodeToString(write.KeyHash)

	logger.Debugf("[%s] Deleting cache entry for hashed key [%s:%s] in block [%d] and TxID [%s]", vdb.chainName, ns, key, metadata.BlockNum, metadata.TxID)

	return vdb.cache.DelState(vdb.chainName, ns, key)
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

	return vv, nil
}

func privateDataHashDBName(namespace, collection string) string {
	return strings.Join([]string{namespace, collection}, pvtDataHashDelimiter)
}
