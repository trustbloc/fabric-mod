/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package storeprovider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollDataStore(t *testing.T) {
	f := NewProviderFactory()
	require.NotNil(t, f)

	s, err := f.OpenStore("testchannel")
	assert.NoError(t, err)
	require.NotNil(t, s)

	s = f.StoreForChannel("testchannel")
	require.NotNil(t, s)

	err = s.Persist("tx1", nil)
	assert.NoError(t, err)

	assert.NotPanics(t, func() { s.Close() })
	assert.NotPanics(t, func() { f.Close() })
}
