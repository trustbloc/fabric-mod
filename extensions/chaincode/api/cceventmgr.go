/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package api

import (
	"github.com/hyperledger/fabric-protos-go/peer"
)

// Definition captures the info about chaincode
type Definition struct {
	Name              string
	Hash              []byte
	Version           string
	CollectionConfigs *peer.CollectionConfigPackage
}

// EventMgr handles chaincode deploy events
type EventMgr interface {
	HandleChaincodeDeploy(channelID string, ccDefs []*Definition) error
	ChaincodeDeployDone(channelID string)
}
