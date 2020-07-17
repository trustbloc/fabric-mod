/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package statecouchdb

import (
	"encoding/base64"
	"fmt"

	"github.com/hyperledger/fabric-protos-go/ledger/rwset/kvrwset"
	"github.com/hyperledger/fabric/extensions/gossip/api"
)

// deleteCacheEntry deletes the cache entry for the given KV write so that it may be refreshed from the database
func (vdb *VersionedDB) deleteCacheEntry(metadata api.TxMetadata, namespace string, write *kvrwset.KVWrite) error {
	logger.Debugf("[%s] Deleting cache entry for [%s:%s] in block [%d] and TxID [%s]", vdb.chainName, namespace, write.Key, metadata.BlockNum, metadata.TxID)

	return vdb.cache.DelState(vdb.chainName, namespace, write.Key)
}

// deleteCollHashCacheEntry deletes the cache entry for the given collection hash write so that it may be refreshed from the database
func (vdb *VersionedDB) deleteCollHashCacheEntry(metadata api.TxMetadata, namespace string, collection string, write *kvrwset.KVWriteHash) error {
	ns := fmt.Sprintf("%s$$h%s", namespace, collection)
	key := base64.StdEncoding.EncodeToString(write.KeyHash)

	logger.Debugf("[%s] Deleting cache entry for hashed key [%s:%s] in block [%d] and TxID [%s]", vdb.chainName, ns, key, metadata.BlockNum, metadata.TxID)

	return vdb.cache.DelState(vdb.chainName, ns, key)
}
