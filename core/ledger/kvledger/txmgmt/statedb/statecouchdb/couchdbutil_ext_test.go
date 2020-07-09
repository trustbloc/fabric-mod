/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package statecouchdb

import (
	"testing"

	"github.com/hyperledger/fabric/core/ledger"
	"github.com/stretchr/testify/require"
)

func TestConstructBlockchainDBName(t *testing.T) {
	dbName := ConstructBlockchainDBName("testchannel", "dbname")
	require.Equal(t, "testchannel$$dbname_", dbName)
}

func TestNewCouchDatabase(t *testing.T) {
	_, err := NewCouchDatabase(nil, "_dbtest")
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "'_dbtest' does not match pattern")

	dbName := ConstructBlockchainDBName("testchannel", "dbname")
	_, err = NewCouchDatabase(&CouchInstance{Conf: &ledger.CouchDBConfig{}}, dbName)
	require.Nil(t, err)
}
