/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package kvledger

import (
	"path/filepath"
	"testing"

	xtestutil "github.com/hyperledger/fabric/extensions/testutil"
	"github.com/hyperledger/fabric/internal/fileutil"

	configtxtest "github.com/hyperledger/fabric/common/configtx/test"
	"github.com/hyperledger/fabric/core/ledger/mock"
	"github.com/stretchr/testify/require"
)

func TestRebuildDBs(t *testing.T) {
	xtestutil.Skip(t, "This test is only valid for LevelDB ID store, Corresponding CouchDB unit test is exist")

	conf, cleanup := testConfig(t)
	defer cleanup()
	provider := testutilNewProvider(conf, t, &mock.DeployedChaincodeInfoProvider{})

	numLedgers := 3
	for i := 0; i < numLedgers; i++ {
		genesisBlock, _ := configtxtest.MakeGenesisBlock(constructTestLedgerID(i))
		provider.Create(genesisBlock)
	}

	// rebuild should fail when provider is still open
	err := RebuildDBs(conf)
	require.Error(t, err, "as another peer node command is executing, wait for that command to complete its execution or terminate it before retrying")
	provider.Close()

	err = RebuildDBs(conf)
	require.NoError(t, err)

	// verify blockstoreIndex, configHistory, history, state, bookkeeper dbs are deleted
	rootFSPath := conf.RootFSPath
	empty, err := fileutil.DirEmpty(filepath.Join(BlockStorePath(rootFSPath), "index"))
	require.NoError(t, err)
	require.True(t, empty)
	empty, err = fileutil.DirEmpty(ConfigHistoryDBPath(rootFSPath))
	require.NoError(t, err)
	require.True(t, empty)
	empty, err = fileutil.DirEmpty(HistoryDBPath(rootFSPath))
	require.NoError(t, err)
	require.True(t, empty)
	empty, err = fileutil.DirEmpty(StateDBPath(rootFSPath))
	require.NoError(t, err)
	require.True(t, empty)
	empty, err = fileutil.DirEmpty(BookkeeperDBPath(rootFSPath))
	require.NoError(t, err)
	require.True(t, empty)

	// rebuild again should be successful
	err = RebuildDBs(conf)
	require.NoError(t, err)
}
