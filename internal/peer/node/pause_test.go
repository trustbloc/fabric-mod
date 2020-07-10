/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"os"
	"testing"

	xtestutil "github.com/hyperledger/fabric/extensions/testutil"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestPauseCmd(t *testing.T) {
	addr, _, destroy := xtestutil.SetupExtTestEnv()
	defer destroy()

	viper.Set("ledger.state.couchDBConfig.couchDBAddress", addr)
	viper.Set("ledger.state.couchDBConfig.username", "admin")
	viper.Set("ledger.state.couchDBConfig.password", "adminpw")
	viper.Set("ledger.state.couchDBConfig.maxRetries", 3)
	viper.Set("ledger.state.couchDBConfig.maxRetriesOnStartup", 3)

	t.Run("when the channelID is not supplied", func(t *testing.T) {
		t.Skip("This test doesn't work with fabric-peer-ext. Must fix and re-enable.")

		cmd := pauseCmd()
		args := []string{}
		cmd.SetArgs(args)
		err := cmd.Execute()
		require.EqualError(t, err, "Must supply channel ID")
	})

	t.Run("when the specified channelID does not exist", func(t *testing.T) {
		t.Skip("This test doesn't work with fabric-peer-ext. Must fix and re-enable.")

		testPath := "/tmp/hyperledger/test"
		os.RemoveAll(testPath)
		viper.Set("peer.fileSystemPath", testPath)
		defer os.RemoveAll(testPath)

		cmd := pauseCmd()
		args := []string{"-c", "ch_p"}
		cmd.SetArgs(args)
		err := cmd.Execute()
		require.EqualError(t, err, "LedgerID does not exist")
	})
}
