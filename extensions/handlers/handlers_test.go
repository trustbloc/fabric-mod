/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package handlers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAuthFilter(t *testing.T) {
	require.Nil(t, GetAuthFilter("handler"))
}

func TestGetDecorator(t *testing.T) {
	require.Nil(t, GetDecorator("handler"))
}
