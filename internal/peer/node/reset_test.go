/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/hyperledger/fabric/core/config"
	xtestutil "github.com/hyperledger/fabric/extensions/testutil"
	"github.com/hyperledger/fabric/internal/fileutil"
	viper "github.com/spf13/viper2015"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResetCmd(t *testing.T) {
	_, _, destroy := xtestutil.SetupExtTestEnv()
	defer destroy()

	testPath := "/tmp/hyperledger/test"
	os.RemoveAll(testPath)
	viper.Set("peer.fileSystemPath", testPath)
	defer os.RemoveAll(testPath)

	viper.Set("logging.ledger", "INFO")
	rootFSPath := filepath.Join(config.GetPath("peer.fileSystemPath"), "ledgersData")
	historyDBPath := filepath.Join(rootFSPath, "historyLeveldb")
	assert.NoError(t,
		os.MkdirAll(historyDBPath, 0755),
	)
	assert.NoError(t,
		ioutil.WriteFile(path.Join(historyDBPath, "dummyfile.txt"), []byte("this is a dummy file for test"), 0644),
	)
	cmd := resetCmd()

	_, err := os.Stat(historyDBPath)
	require.False(t, os.IsNotExist(err))
	require.NoError(t, cmd.Execute())
	empty, err := fileutil.DirEmpty(historyDBPath)
	require.NoError(t, err)
	require.True(t, empty)
}
