/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package peer

import (
	"testing"

	"github.com/hyperledger/fabric/core/committer/txvalidator/v20/plugindispatcher"
	"github.com/hyperledger/fabric/core/common/sysccprovider"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/protos/common"

	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/stretchr/testify/require"
)

func TestJoinChainHandler(t *testing.T) {

	sampelResponse := pb.Response{Message: "sample-test-msg"}
	handle := func(string, *common.Block, sysccprovider.SystemChaincodeProvider,
		ledger.DeployedChaincodeInfoProvider, plugindispatcher.LifecycleResources, plugindispatcher.CollectionAndLifecycleResources) pb.Response {
		return sampelResponse
	}

	response := JoinChainHandler(handle)("", nil, nil, nil, nil, nil)
	require.Equal(t, sampelResponse, response)

}
