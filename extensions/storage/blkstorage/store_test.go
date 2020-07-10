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

	"github.com/hyperledger/fabric/common/metrics/metricsfakes"
	"github.com/hyperledger/fabric/extensions/testutil"

	"github.com/hyperledger/fabric/common/ledger/blkstorage"
	coreconfig "github.com/hyperledger/fabric/core/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	cleanup := setupPath(t)
	defer cleanup()

	p, err := NewProvider(
		NewConf(filepath.Join(coreconfig.GetPath("peer.fileSystemPath"), "chains"), -1),
		&blkstorage.IndexConfig{},
		testutil.TestLedgerConf(),
		&metricsfakes.Provider{},
	)
	require.NoError(t, err)
	require.NotEmpty(t, p)
}

func setupPath(t *testing.T) (cleanup func()) {
	tempDir, err := ioutil.TempDir("", "transientstore")
	require.NoError(t, err)

	viper.Set("peer.fileSystemPath", tempDir)
	return func() { os.RemoveAll(tempDir) }
}
