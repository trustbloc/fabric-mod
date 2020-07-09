/*
Copyright SecureKey Technologies Inc. All Rights Reserved.
SPDX-License-Identifier: Apache-2.0
*/

package statecouchdb

import (
	"testing"
	"time"

	"github.com/hyperledger/fabric/common/metrics/disabled"
	"github.com/stretchr/testify/require"
)

const testIndexDef = `
	{
		"index": {
			"fields": ["testnumber"]
		},
		"name": "by_test_number",
		"ddoc": "indexTestNumber",
		"type": "json"
	}`

func TestCreateIndexWithRetry(t *testing.T) {
	config := testConfig()
	couchDBEnv.startCouchDB(t)
	config.Address = couchDBEnv.couchAddress
	defer couchDBEnv.cleanup(config)

	database := "testcreateindexwithretry"

	//create a new instance and database object
	couchInstance, err := CreateCouchInstance(config, &disabled.Provider{})
	require.NoError(t, err, "Error when trying to create couch instance")
	db := CouchDatabase{CouchInstance: couchInstance, DBName: database}

	errdb := db.CreateDatabaseIfNotExist()
	require.NoError(t, errdb, "Error when trying to create database")

	// Create successful index
	err = db.CreateNewIndexWithRetry(testIndexDef, "indexTestNumber")
	require.NoError(t, err)

	// Create wrong index
	err = db.CreateNewIndexWithRetry("wrongindex", "wrongindex")
	require.Error(t, err)
	require.Contains(t, err.Error(), "JSON format is not valid")

}

func TestIndexDesignDocExists(t *testing.T) {
	config := testConfig()
	couchDBEnv.startCouchDB(t)
	config.Address = couchDBEnv.couchAddress
	defer couchDBEnv.cleanup(config)

	database := "testindexdesigndocexists"

	//create a new instance and database object
	couchInstance, err := CreateCouchInstance(config, &disabled.Provider{})
	require.NoError(t, err, "Error when trying to create couch instance")
	db := CouchDatabase{CouchInstance: couchInstance, DBName: database}

	errdb := db.CreateDatabaseIfNotExist()
	require.NoError(t, errdb, "Error when trying to create database")

	// check index if exist before create
	exists, err := db.IndexDesignDocExists("indexTestNumber")
	require.NoError(t, err)
	require.False(t, exists)

	// Create successful index
	err = db.CreateNewIndexWithRetry(testIndexDef, "indexTestNumber")
	require.NoError(t, err)

	// check index if exist after create
	exists, err = db.IndexDesignDocExists("indexTestNumber")
	require.NoError(t, err)
	require.True(t, exists)
}

func TestIndexDesignDocExistsWithRetry(t *testing.T) {
	config := testConfig()
	config.MaxRetries = 5

	couchDBEnv.startCouchDB(t)
	config.Address = couchDBEnv.couchAddress
	defer couchDBEnv.cleanup(config)

	database := "testindexdesigndocexistswithretry"

	//create a new instance and database object
	couchInstance, err := CreateCouchInstance(config, &disabled.Provider{})
	require.NoError(t, err, "Error when trying to create couch instance")
	db := CouchDatabase{CouchInstance: couchInstance, DBName: database}

	errdb := db.CreateDatabaseIfNotExist()
	require.NoError(t, errdb, "Error when trying to create database")

	// check index if exist before create
	exists, err := db.IndexDesignDocExistsWithRetry("indexTestNumber")
	require.Error(t, err)
	require.Equal(t, exists, false)

	go func() {
		time.Sleep(300 * time.Millisecond)
		// Create successful index
		err := db.CreateNewIndexWithRetry(testIndexDef, "indexTestNumber")
		require.NoError(t, err)
	}()

	// check index if exist after create
	exists, err = db.IndexDesignDocExistsWithRetry("indexTestNumber")
	require.NoError(t, err)
	require.Equal(t, exists, true)

}

func TestDBExists(t *testing.T) {
	config := testConfig()
	couchDBEnv.startCouchDB(t)
	config.Address = couchDBEnv.couchAddress
	defer couchDBEnv.cleanup(config)

	database := "testdbexists"

	//create a new instance and database object
	couchInstance, err := CreateCouchInstance(config, &disabled.Provider{})
	require.NoError(t, err, "Error when trying to create couch instance")
	db := CouchDatabase{CouchInstance: couchInstance, DBName: database}

	// check if db exists before create
	exists, err := db.Exists()
	require.NoError(t, err)
	require.Equal(t, exists, false)

	errdb := db.CreateDatabaseIfNotExist()
	require.NoError(t, errdb, "Error when trying to create database")

	// check if db exists after create
	exists, err = db.Exists()
	require.NoError(t, err)
	require.Equal(t, exists, true)

}

func TestDBExistsWithRetry(t *testing.T) {
	config := testConfig()
	couchDBEnv.startCouchDB(t)
	config.Address = couchDBEnv.couchAddress
	defer couchDBEnv.cleanup(config)

	database := "testdbexistswithretry"

	//create a new instance and database object
	couchInstance, err := CreateCouchInstance(config, &disabled.Provider{})
	require.NoError(t, err, "Error when trying to create couch instance")
	db := CouchDatabase{CouchInstance: couchInstance, DBName: database}

	// check if db exists before create
	exists, err := db.ExistsWithRetry()
	require.NoError(t, err)
	require.Equal(t, exists, false)

	go func() {
		time.Sleep(300 * time.Millisecond)
		errdb := db.CreateDatabaseIfNotExist()
		require.NoError(t, errdb, "Error when trying to create database")
	}()
	// check if db exists after create
	exists, err = db.ExistsWithRetry()
	require.NoError(t, err)
	require.Equal(t, exists, true)

}
