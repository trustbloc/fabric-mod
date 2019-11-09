/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package resource

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResources(t *testing.T) {
	require.NoError(t, Initialize())
	require.NotPanics(t, func() {
		Close()
	})
	require.NotPanics(t, func() {
		ChannelJoined("channel1")
	})
}
