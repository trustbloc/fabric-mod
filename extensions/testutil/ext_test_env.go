/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package testutil

import (
	"testing"

	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/statedb"
)

//SetupExtTestEnv creates new extension test environment,
// it creates couchdb instance for test, returns couchdbd address, cleanup and destroy function handle.
func SetupExtTestEnv() (addr string, cleanup func(string), stop func()) {
	return "", func(string) {
			//do nothing
		}, func() {
			//do nothing
		}
}

// SetupResources sets up all of the mock resource providers
func SetupResources() func() {
	return func() {
		//do nothing
	}
}

func GetExtStateDBProvider(t testing.TB, dbProvider statedb.VersionedDBProvider) statedb.VersionedDBProvider {
	return nil
}

func TestLedgerConf() *ledger.Config {
	conf := &ledger.Config{
		RootFSPath: "",
		StateDBConfig: &ledger.StateDBConfig{
			CouchDB: &ledger.CouchDBConfig{},
		},
		PrivateDataConfig: &ledger.PrivateDataConfig{},
		HistoryDBConfig:   &ledger.HistoryDBConfig{},
	}

	return conf
}

// Skip skips the unit test for extensions
func Skip(t *testing.T, msg string) {
}
