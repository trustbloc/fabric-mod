/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package service

import (
	"github.com/hyperledger/fabric/protos/gossip"
)

//HandleGossip can be used to extend ossipServiceAdapter.Gossip feature
func HandleGossip(handle func(msg *gossip.GossipMessage)) func(msg *gossip.GossipMessage) {
	return handle
}

//IsPvtDataReconcilerEnabled can be used to override private data reconciler enable/disable
func IsPvtDataReconcilerEnabled(isEnabled bool) bool {
	return isEnabled
}
