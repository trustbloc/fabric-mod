/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package dispatcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider(t *testing.T) {
	p := NewProvider()
	require.NotNil(t, p)
	require.Equal(t, p, p.Initialize(nil, nil))
	d := p.ForChannel("testchannel", nil)
	require.NotNil(t, d)
	assert.Falsef(t, d.Dispatch(nil), "should always return false since this is a noop dispatcher")
}
