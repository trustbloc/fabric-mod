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
	path, cleanup := setupPath(t)
	defer cleanup()

	p, err := NewStoreProvider(path)
	require.NoError(t, err)
	require.NotEmpty(t, p)
}

func setupPath(t *testing.T) (string, func()) {
	tempDir, err := ioutil.TempDir("", "transientstore")
	require.NoError(t, err)

	viper.Set("peer.fileSystemPath", tempDir)
	return tempDir, func() { os.RemoveAll(tempDir) }
}
