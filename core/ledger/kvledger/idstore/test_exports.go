/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package idstore

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/spf13/viper"

	"github.com/hyperledger/fabric/core/ledger/ledgerconfig"
	"github.com/stretchr/testify/require"
)

// StoreEnv provides the  store env for testing
type StoreEnv struct {
	t         testing.TB
	TestStore IDStore
}

// NewTestStoreEnv construct a StoreEnv for testing
func NewTestStoreEnv(t *testing.T) *StoreEnv {
	tempDir, err := ioutil.TempDir("", "idstore")
	require.NoError(t, err)
	viper.Set("peer.fileSystemPath", tempDir)
	removeStorePath(t)
	testStore := OpenIDStore(tempDir)
	return &StoreEnv{t, testStore}
}

// Cleanup cleansup the  store env after testing
func (env *StoreEnv) Cleanup() {
	removeStorePath(env.t)
}

func removeStorePath(t testing.TB) {
	dbPath := ledgerconfig.GetLedgerProviderPath()
	if err := os.RemoveAll(dbPath); err != nil {
		t.Fatalf("Err: %s", err)
		t.FailNow()
	}
}
