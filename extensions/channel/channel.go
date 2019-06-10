/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package peer

import (
	"github.com/hyperledger/fabric/core/committer/txvalidator/plugin"
	"github.com/hyperledger/fabric/core/committer/txvalidator/v20/plugindispatcher"
	"github.com/hyperledger/fabric/core/common/sysccprovider"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/protos/common"
	pb "github.com/hyperledger/fabric/protos/peer"
)

type JoinChain func(string, *common.Block, sysccprovider.SystemChaincodeProvider, ledger.DeployedChaincodeInfoProvider, plugindispatcher.LifecycleResources, plugindispatcher.CollectionAndLifecycleResources) pb.Response

type CreateChain func(string, ledger.PeerLedger, *common.Block, sysccprovider.SystemChaincodeProvider, plugin.Mapper,
	ledger.DeployedChaincodeInfoProvider, plugindispatcher.LifecycleResources, plugindispatcher.CollectionAndLifecycleResources) error

//JoinChainHandler can be used to provide extended features to CSCC join channel
func JoinChainHandler(handle JoinChain) JoinChain {
	return handle
}

//RegisterChannelInitializer registers channel initializer using given plugin mapper and create chain handle
func RegisterChannelInitializer(plugin.Mapper, CreateChain) {
	//do nothing
}
