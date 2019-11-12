/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package retriever

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	p := NewProvider()
	require.NotNil(t, p)
	require.Equal(t, p, p.Initialize())
	assert.PanicsWithValue(t, "not implemented", func() {
		p.RetrieverForChannel("testchannel")
	})
}
