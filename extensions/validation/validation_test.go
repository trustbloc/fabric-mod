/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package validation

import (
	"testing"

	"github.com/hyperledger/fabric/extensions/validation/mocks"
	"github.com/stretchr/testify/require"
)

func TestNewTxValidator(t *testing.T) {
	v := NewTxValidator("channel1", nil, &mocks.ChannelResources{}, nil, nil, nil, nil, nil, nil)
	require.NotNil(t, v)
}
