/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package couchdb

import (
	"encoding/hex"

	"github.com/hyperledger/fabric/common/util"
)

// ConstructBlockchainDBName truncates the db name to couchdb allowed length to
// construct the blockchain-related databases.
func ConstructBlockchainDBName(chainName, dbName string) string {
	chainDBName := joinSystemDBName(chainName, dbName)

	if len(chainDBName) > maxLength {
		untruncatedDBName := chainDBName

		// As truncated namespaceDBName is of form 'chainName_escapedNamespace', both chainName
		// and escapedNamespace need to be truncated to defined allowed length.
		if len(chainName) > chainNameAllowedLength {
			// Truncate chainName to chainNameAllowedLength
			chainName = chainName[:chainNameAllowedLength]
		}

		// For metadataDB (i.e., chain/channel DB), the dbName contains <first 50 chars
		// (i.e., chainNameAllowedLength) of chainName> + (SHA256 hash of actual chainName)
		chainDBName = joinSystemDBName(chainName, dbName) + "(" + hex.EncodeToString(util.ComputeSHA256([]byte(untruncatedDBName))) + ")"
		// 50 chars for dbName + 1 char for ( + 64 chars for sha256 + 1 char for ) = 116 chars
	}
	return chainDBName + "_"
}

func joinSystemDBName(chainName, dbName string) string {
	systemDBName := chainName
	if len(dbName) > 0 {
		systemDBName += "$$" + dbName
	}
	return systemDBName
}

// NewCouchDatabase creates a CouchDB database object, but not the underlying database if it does not exist
func NewCouchDatabase(couchInstance *CouchInstance, dbName string) (*CouchDatabase, error) {

	databaseName, err := mapAndValidateDatabaseName(dbName)
	if err != nil {
		logger.Errorf("Error during CouchDB CreateDatabaseIfNotExist() for dbName: %s  error: %s", dbName, err.Error())
		return nil, err
	}

	couchDBDatabase := CouchDatabase{CouchInstance: couchInstance, DBName: databaseName, IndexWarmCounter: 1}
	return &couchDBDatabase, nil
}
