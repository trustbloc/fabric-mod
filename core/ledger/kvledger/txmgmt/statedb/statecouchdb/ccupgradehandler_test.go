/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package statecouchdb

import (
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/statedb"
	"github.com/hyperledger/fabric/extensions/gossip/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCCUpgradeHandler(t *testing.T) {
	env := NewTestVDBEnv(t)
	defer env.Cleanup()

	db, err := env.DBProvider.GetDBHandle("testinit")
	assert.NoError(t, err)
	db.Open()
	defer db.Close()

	initLen := len(db.(*VersionedDB).lsccStateCache.cache)
	db.(*VersionedDB).lsccStateCache.cache["samplecc"] = &statedb.VersionedValue{Value: []byte("samplecc")}
	db.(*VersionedDB).lsccStateCache.cache["samplecc~collection"] = &statedb.VersionedValue{Value: []byte("samplecc~collection")}
	db.(*VersionedDB).lsccStateCache.cache["samplecc2"] = &statedb.VersionedValue{Value: []byte("samplecc2")}
	db.(*VersionedDB).lsccStateCache.cache["samplecc2~collection"] = &statedb.VersionedValue{Value: []byte("samplecc2~collection")}

	handler := getCCUpgradeHandler(db.(*VersionedDB))
	err = handler(api.TxMetadata{}, "samplecc")
	require.NoError(t, err)
	require.Len(t, db.(*VersionedDB).lsccStateCache.cache, initLen+2)

	err = handler(api.TxMetadata{}, "samplecc2")
	require.NoError(t, err)
	require.Len(t, db.(*VersionedDB).lsccStateCache.cache, initLen)
}
