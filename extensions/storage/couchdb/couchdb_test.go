/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package couchdb

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric/core/ledger/util/couchdb"
)

func TestHandleCreateCouchDatabase(t *testing.T) {

	sampleDB := &couchdb.CouchDatabase{DBName: "sample-test-run-db"}
	handle := func(couchInstance *couchdb.CouchInstance, dbName string) (*couchdb.CouchDatabase, error) {
		return sampleDB, nil
	}
	db, err := HandleCreateCouchDatabase(handle)(nil, "")
	require.Equal(t, sampleDB, db)
	require.NoError(t, err)

}
