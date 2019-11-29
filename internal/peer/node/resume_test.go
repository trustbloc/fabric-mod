/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestResumeCmd(t *testing.T) {
	t.Run("when the channelID is not supplied", func(t *testing.T) {
		cmd := resumeCmd()
		args := []string{}
		cmd.SetArgs(args)
		err := cmd.Execute()
		require.EqualError(t, err, "Must supply channel ID")
	})

	t.Run("when the specified channelID does not exist", func(t *testing.T) {
		testPath := "/tmp/hyperledger/test"
		os.RemoveAll(testPath)
		viper.Set("peer.fileSystemPath", testPath)
		defer os.RemoveAll(testPath)

		cmd := resumeCmd()
		args := []string{"-c", "ch_r"}
		cmd.SetArgs(args)
		err := cmd.Execute()
		require.EqualError(t, err, "LedgerID does not exist")
	})
}
