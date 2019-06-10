/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package recover

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecoverDBHandler(t *testing.T) {
	sampleError := errors.New("sample-error")
	handle := func() error {
		return sampleError
	}
	err := RecoverDBHandler(handle)()
	require.Equal(t, sampleError, err)
}
