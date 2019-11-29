/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package idstore

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/hyperledger/fabric/extensions/testutil"

	"github.com/spf13/viper"

	"github.com/stretchr/testify/require"
)

func TestOpenIDStore(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "idstore")
	require.NoError(t, err)
	viper.Set("peer.fileSystemPath", tempDir)
	defer os.RemoveAll(tempDir)

	s, err := OpenIDStore(tempDir, testutil.TestLedgerConf())
	require.NoError(t, err)
	require.NotEmpty(t, s)
}
