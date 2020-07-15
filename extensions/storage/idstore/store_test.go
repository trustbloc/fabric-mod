/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package idstore

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/hyperledger/fabric/core/ledger"
	storageapi "github.com/hyperledger/fabric/extensions/storage/api"
	"github.com/hyperledger/fabric/extensions/testutil"

	viper "github.com/spf13/viper2015"

	"github.com/stretchr/testify/require"
)

func TestOpenIDStore(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "idstore")
	require.NoError(t, err)
	viper.Set("peer.fileSystemPath", tempDir)
	defer os.RemoveAll(tempDir)

	idStore := &MockIDStore{}

	s, err := OpenIDStore(tempDir, testutil.TestLedgerConf(),
		func(path string, ledgerconfig *ledger.Config) (storageapi.IDStore, error) {
			return idStore, nil
		},
	)
	require.NoError(t, err)
	require.Equalf(t, idStore, s, "expecting default ID store to be created")
}
