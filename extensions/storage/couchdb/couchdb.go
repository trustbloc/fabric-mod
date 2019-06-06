/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package couchdb

import "github.com/hyperledger/fabric/core/ledger/util/couchdb"

type CreateCouchDatabase func(couchInstance *couchdb.CouchInstance, dbName string) (*couchdb.CouchDatabase, error)

func HandleCreateCouchDatabase(handle CreateCouchDatabase) CreateCouchDatabase {
	return handle
}
