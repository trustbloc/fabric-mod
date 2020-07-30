/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chaincode

import (
	"testing"

	"github.com/hyperledger/fabric/extensions/chaincode/mock"
	"github.com/stretchr/testify/require"
)

//go:generate counterfeiter -o ./mock/usercc.gen.go -fake-name UserCC ./api UserCC

func TestGetUCC(t *testing.T) {
	cc, ok := GetUCC("", "")
	require.False(t, ok)
	require.Nil(t, cc)
}

func TestChaincodes(t *testing.T) {
	require.Empty(t, Chaincodes())
}

func TestWaitForReady(t *testing.T) {
	require.NotPanics(t, WaitForReady)
}

func TestGetPackageID(t *testing.T) {
	const cc1 = "cc1"
	const v1 = "v1"

	cc := &mock.UserCC{}
	cc.NameReturns(cc1)
	cc.VersionReturns(v1)

	ccid := GetPackageID(cc)
	require.Equal(t, "cc1:v1", ccid)
}
