/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package privacyenabledstate

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/hyperledger/fabric/core/ledger/kvledger/txmgmt/statedb/stateleveldb"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	cleanup := setupPath(t)
	defer cleanup()

	require.NotEmpty(t, NewVersionedDBProvider(stateleveldb.NewVersionedDBProvider()))

	require.Empty(t, NewVersionedDBProvider(nil))
}

func setupPath(t *testing.T) (cleanup func()) {
	tempDir, err := ioutil.TempDir("", "statedb")
	require.NoError(t, err)

	viper.Set("peer.fileSystemPath", tempDir)
	return func() { os.RemoveAll(tempDir) }
}
