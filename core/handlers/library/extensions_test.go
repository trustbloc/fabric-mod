/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package library

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistry_LoadExtensions(t *testing.T) {
	testReg := registry{}
	require.False(t, testReg.loadExtension("handler1", Auth))
	require.False(t, testReg.loadExtension("handler1", Decoration))
	require.False(t, testReg.loadExtension("handler1", Endorsement))
	require.False(t, testReg.loadExtension("handler1", Validation))
}
