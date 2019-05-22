/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package extscc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtSysCC(t *testing.T) {
	require.Empty(t, CreateExtSysCCs())
}
