/*
Copyright IBM Corp. 2016 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"testing"
	"time"

	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/util/couchdb"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestLedgerConfig(t *testing.T) {
	defer viper.Set("ledger.state.stateDatabase", "goleveldb")
	var tests = []struct {
		name     string
		config   map[string]interface{}
		expected *ledger.Config
	}{
		{
			name: "goleveldb",
			config: map[string]interface{}{
				"peer.fileSystemPath":        "/peerfs",
				"ledger.state.stateDatabase": "goleveldb",
			},
			expected: &ledger.Config{
				RootFSPath: "/peerfs/ledgersData",
				StateDB: &ledger.StateDB{
					StateDatabase: "goleveldb",
					LevelDBPath:   "/peerfs/ledgersData/stateLeveldb",
					CouchDB:       &couchdb.Config{},
				},
				PrivateData: &ledger.PrivateData{
					StorePath:       "/peerfs/ledgersData/pvtdataStore",
					MaxBatchSize:    5000,
					BatchesInterval: 1000,
					PurgeInterval:   100,
				},
				HistoryDB: &ledger.HistoryDB{
					Enabled: false,
				},
			},
		},
		{
			name: "CouchDB Defaults",
			config: map[string]interface{}{
				"peer.fileSystemPath":                              "/peerfs",
				"ledger.state.stateDatabase":                       "CouchDB",
				"ledger.state.couchDBConfig.couchDBAddress":        "localhost:5984",
				"ledger.state.couchDBConfig.username":              "username",
				"ledger.state.couchDBConfig.password":              "password",
				"ledger.state.couchDBConfig.maxRetries":            3,
				"ledger.state.couchDBConfig.maxRetriesOnStartup":   10,
				"ledger.state.couchDBConfig.requestTimeout":        "30s",
				"ledger.state.couchDBConfig.createGlobalChangesDB": true,
			},
			expected: &ledger.Config{
				RootFSPath: "/peerfs/ledgersData",
				StateDB: &ledger.StateDB{
					StateDatabase: "CouchDB",
					LevelDBPath:   "/peerfs/ledgersData/stateLeveldb",
					CouchDB: &couchdb.Config{
						Address:                 "localhost:5984",
						Username:                "username",
						Password:                "password",
						MaxRetries:              3,
						MaxRetriesOnStartup:     10,
						RequestTimeout:          30 * time.Second,
						InternalQueryLimit:      1000,
						MaxBatchUpdateSize:      500,
						WarmIndexesAfterNBlocks: 1,
						CreateGlobalChangesDB:   true,
						RedoLogPath:             "/peerfs/ledgersData/couchdbRedoLogs",
					},
				},
				PrivateData: &ledger.PrivateData{
					StorePath:       "/peerfs/ledgersData/pvtdataStore",
					MaxBatchSize:    5000,
					BatchesInterval: 1000,
					PurgeInterval:   100,
				},
				HistoryDB: &ledger.HistoryDB{
					Enabled: false,
				},
			},
		},
		{
			name: "CouchDB Explicit",
			config: map[string]interface{}{
				"peer.fileSystemPath":                                "/peerfs",
				"ledger.state.stateDatabase":                         "CouchDB",
				"ledger.state.couchDBConfig.couchDBAddress":          "localhost:5984",
				"ledger.state.couchDBConfig.username":                "username",
				"ledger.state.couchDBConfig.password":                "password",
				"ledger.state.couchDBConfig.maxRetries":              3,
				"ledger.state.couchDBConfig.maxRetriesOnStartup":     10,
				"ledger.state.couchDBConfig.requestTimeout":          "30s",
				"ledger.state.couchDBConfig.internalQueryLimit":      500,
				"ledger.state.couchDBConfig.maxBatchUpdateSize":      600,
				"ledger.state.couchDBConfig.warmIndexesAfterNBlocks": 5,
				"ledger.state.couchDBConfig.createGlobalChangesDB":   true,
				"ledger.pvtdataStore.collElgProcMaxDbBatchSize":      50000,
				"ledger.pvtdataStore.collElgProcDbBatchesInterval":   10000,
				"ledger.pvtdataStore.purgeInterval":                  1000,
				"ledger.history.enableHistoryDatabase":               true,
			},
			expected: &ledger.Config{
				RootFSPath: "/peerfs/ledgersData",
				StateDB: &ledger.StateDB{
					StateDatabase: "CouchDB",
					LevelDBPath:   "/peerfs/ledgersData/stateLeveldb",
					CouchDB: &couchdb.Config{
						Address:                 "localhost:5984",
						Username:                "username",
						Password:                "password",
						MaxRetries:              3,
						MaxRetriesOnStartup:     10,
						RequestTimeout:          30 * time.Second,
						InternalQueryLimit:      500,
						MaxBatchUpdateSize:      600,
						WarmIndexesAfterNBlocks: 5,
						CreateGlobalChangesDB:   true,
						RedoLogPath:             "/peerfs/ledgersData/couchdbRedoLogs",
					},
				},
				PrivateData: &ledger.PrivateData{
					StorePath:       "/peerfs/ledgersData/pvtdataStore",
					MaxBatchSize:    50000,
					BatchesInterval: 10000,
					PurgeInterval:   1000,
				},
				HistoryDB: &ledger.HistoryDB{
					Enabled: true,
				},
			},
		},
	}

	for _, test := range tests {
		_test := test
		t.Run(_test.name, func(t *testing.T) {
			for k, v := range _test.config {
				viper.Set(k, v)
			}
			conf := ledgerConfig()
			assert.EqualValues(t, _test.expected, conf)
		})
	}
}
