/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package couchdb

import (
	storageapi "github.com/hyperledger/fabric/extensions/storage/api"
)

// CreateCouchDatabase is a handle function type for create couch db
type CreateCouchDatabase func(couchInstance storageapi.CouchInstance, dbName string) (storageapi.CouchDatabase, error)

// HandleCreateCouchDatabase can be used to extend create couch db feature
func HandleCreateCouchDatabase(handle CreateCouchDatabase) CreateCouchDatabase {
	return handle
}
