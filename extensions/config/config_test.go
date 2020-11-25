/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsSkipCheckForDupTxnID(t *testing.T) {
	require.False(t, IsSkipCheckForDupTxnID())
}
