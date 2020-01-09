/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package pvtdatastorage

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/pvtdatastorage"
	"github.com/hyperledger/fabric/extensions/testutil"
	viper "github.com/spf13/viper2015"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	path, cleanup := setupPath(t)
	defer cleanup()
	conf := &pvtdatastorage.PrivateDataConfig{
		PrivateDataConfig: &ledger.PrivateDataConfig{
			PurgeInterval: 1,
		},
		StorePath: filepath.Join(path, "pvtdatastorage"),
	}

	p, err := NewProvider(conf, testutil.TestLedgerConf())
	require.NoError(t, err)
	require.NotEmpty(t, p)
}

func setupPath(t *testing.T) (string, func()) {
	tempDir, err := ioutil.TempDir("", "pvtdatastorage")
	require.NoError(t, err)

	viper.Set("peer.fileSystemPath", tempDir)
	return tempDir, func() { os.RemoveAll(tempDir) }
}
