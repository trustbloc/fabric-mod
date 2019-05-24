/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blkstorage

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hyperledger/fabric/common/ledger/blkstorage"
	coreconfig "github.com/hyperledger/fabric/core/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	cleanup := setupPath(t)
	defer cleanup()
	require.NotEmpty(t, NewProvider(NewConf(filepath.Join(coreconfig.GetPath("peer.fileSystemPath"), "chains"),
		-1), &blkstorage.IndexConfig{}))
}

func setupPath(t *testing.T) (cleanup func()) {
	tempDir, err := ioutil.TempDir("", "transientstore")
	require.NoError(t, err)

	viper.Set("peer.fileSystemPath", tempDir)
	return func() { os.RemoveAll(tempDir) }
}
