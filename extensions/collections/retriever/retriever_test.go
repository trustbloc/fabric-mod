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
	p := NewProvider(nil, nil, nil, nil)
	require.NotNil(t, p)

	assert.PanicsWithValue(t, "not implemented", func() {
		p.RetrieverForChannel("testchannel")
	})
}
