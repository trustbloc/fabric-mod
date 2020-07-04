/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package couchdb

import (
	"errors"
	"testing"

	storageapi "github.com/hyperledger/fabric/extensions/storage/api"
	"github.com/stretchr/testify/require"
)

func TestHandleCreateCouchDatabase(t *testing.T) {
	errExpected := errors.New("injected couch error")
	handle := func(couchInstance storageapi.CouchInstance, dbName string) (storageapi.CouchDatabase, error) {
		return nil, errExpected
	}

	db, err := HandleCreateCouchDatabase(handle)(nil, "")
	require.EqualErrorf(t, err, errExpected.Error(), "expecting default handler to be called")
	require.Nil(t, db)
}
