/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package roles

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsCommitter(t *testing.T) {
	require.True(t, IsCommitter())
}

func TestIsEndorser(t *testing.T) {
	require.True(t, IsEndorser())
}

func TestIsValidator(t *testing.T) {
	require.True(t, IsValidator())
}

func TestRolesAsString(t *testing.T) {
	require.Empty(t, RolesAsString())
}
