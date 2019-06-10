/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package ledgerstorage

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncPvtdataStoreWithBlockStoreHandler(t *testing.T) {
	sampleError := errors.New("sample-error")
	handle := func() error {
		return sampleError
	}
	err := SyncPvtdataStoreWithBlockStoreHandler(handle)()
	require.Equal(t, sampleError, err)
}
