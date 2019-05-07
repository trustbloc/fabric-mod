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
	p := New("testchannel", nil, nil, nil, nil)
	require.NotNil(t, p)
	assert.Falsef(t, p.Dispatch(nil), "should always return false since this is a noop dispatcher")
}
