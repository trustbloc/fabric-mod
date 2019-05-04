/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package transientstore

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/spf13/viper"

	"github.com/stretchr/testify/require"
)

func TestNewStoreProvider(t *testing.T) {
	cleanup := setupPath(t)
	defer cleanup()
	require.NotEmpty(t, NewStoreProvider())
}

func setupPath(t *testing.T) (cleanup func()) {
	tempDir, err := ioutil.TempDir("", "transientstore")
	require.NoError(t, err)

	viper.Set("peer.fileSystemPath", tempDir)
	return func() { os.RemoveAll(tempDir) }
}
