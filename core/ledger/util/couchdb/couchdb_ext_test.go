/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package couchdb

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
	database := "testcreateindexwithretry"
	err := cleanup(database)
	require.NoError(t, err, "Error when trying to cleanup  Error: %s", err)
	defer cleanup(database)

	//create a new instance and database object
	couchInstance, err := CreateCouchInstance(couchDBDef.URL, couchDBDef.Username, couchDBDef.Password,
		couchDBDef.MaxRetries, couchDBDef.MaxRetriesOnStartup, couchDBDef.RequestTimeout, couchDBDef.CreateGlobalChangesDB, &disabled.Provider{})
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
	database := "testindexdesigndocexists"
	err := cleanup(database)
	require.NoError(t, err, "Error when trying to cleanup  Error: %s", err)
	defer cleanup(database)

	//create a new instance and database object
	couchInstance, err := CreateCouchInstance(couchDBDef.URL, couchDBDef.Username, couchDBDef.Password,
		couchDBDef.MaxRetries, couchDBDef.MaxRetriesOnStartup, couchDBDef.RequestTimeout, couchDBDef.CreateGlobalChangesDB, &disabled.Provider{})
	require.NoError(t, err, "Error when trying to create couch instance")
	db := CouchDatabase{CouchInstance: couchInstance, DBName: database}

	errdb := db.CreateDatabaseIfNotExist()
	require.NoError(t, errdb, "Error when trying to create database")

	// check index if exist before create
	exists, err := db.IndexDesignDocExists("indexTestNumber")
	require.NoError(t, err)
	require.Equal(t, exists, false)

	// Create successful index
	err = db.CreateNewIndexWithRetry(testIndexDef, "indexTestNumber")
	require.NoError(t, err)

	// check index if exist after create
	exists, err = db.IndexDesignDocExists("indexTestNumber")
	require.NoError(t, err)
	require.Equal(t, exists, true)

}

func TestIndexDesignDocExistsWithRetry(t *testing.T) {
	database := "testindexdesigndocexistswithretry"
	err := cleanup(database)
	require.NoError(t, err, "Error when trying to cleanup  Error: %s", err)
	defer cleanup(database)

	//create a new instance and database object
	couchInstance, err := CreateCouchInstance(couchDBDef.URL, couchDBDef.Username, couchDBDef.Password,
		5, couchDBDef.MaxRetriesOnStartup, couchDBDef.RequestTimeout, couchDBDef.CreateGlobalChangesDB, &disabled.Provider{})
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
	database := "testdbexists"
	err := cleanup(database)
	require.NoError(t, err, "Error when trying to cleanup  Error: %s", err)
	defer cleanup(database)

	//create a new instance and database object
	couchInstance, err := CreateCouchInstance(couchDBDef.URL, couchDBDef.Username, couchDBDef.Password,
		couchDBDef.MaxRetries, couchDBDef.MaxRetriesOnStartup, couchDBDef.RequestTimeout, couchDBDef.CreateGlobalChangesDB, &disabled.Provider{})
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
	database := "testdbexistswithretry"
	err := cleanup(database)
	require.NoError(t, err, "Error when trying to cleanup  Error: %s", err)
	defer cleanup(database)

	//create a new instance and database object
	couchInstance, err := CreateCouchInstance(couchDBDef.URL, couchDBDef.Username, couchDBDef.Password,
		5, couchDBDef.MaxRetriesOnStartup, couchDBDef.RequestTimeout, couchDBDef.CreateGlobalChangesDB, &disabled.Provider{})
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
