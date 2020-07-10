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
	"github.com/hyperledger/fabric/core/ledger/kvledger"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestRebuildDBsCmd(t *testing.T) {
	t.Skip("Re-enable this test when upgrade is supported by fabric-peer-ext")

	testPath := "/tmp/hyperledger/test"
	os.RemoveAll(testPath)
	viper.Set("peer.fileSystemPath", testPath)
	defer os.RemoveAll(testPath)

	viper.Set("logging.ledger", "INFO")
	rootFSPath := filepath.Join(config.GetPath("peer.fileSystemPath"), "ledgersData")
	bookkeeperDBPath := kvledger.BookkeeperDBPath(rootFSPath)
	configHistoryDBPath := kvledger.ConfigHistoryDBPath(rootFSPath)
	historyDBPath := kvledger.HistoryDBPath(rootFSPath)
	stateDBPath := kvledger.StateDBPath(rootFSPath)
	blockstoreIndexDBPath := filepath.Join(kvledger.BlockStorePath(rootFSPath), "index")

	dbPaths := []string{bookkeeperDBPath, configHistoryDBPath, historyDBPath, stateDBPath, blockstoreIndexDBPath}
	for _, dbPath := range dbPaths {
		require.NoError(t, os.MkdirAll(dbPath, 0755))
		require.NoError(t, ioutil.WriteFile(path.Join(dbPath, "dummyfile.txt"), []byte("this is a dummy file for test"), 0644))
	}

	// check dbs exist before upgrade
	for _, dbPath := range dbPaths {
		_, err := os.Stat(dbPath)
		require.False(t, os.IsNotExist(err))
	}

	cmd := rebuildDBsCmd()
	require.NoError(t, cmd.Execute())

	// check dbs do not exist after upgrade
	for _, dbPath := range dbPaths {
		_, err := os.Stat(dbPath)
		require.True(t, os.IsNotExist(err))
	}
}
