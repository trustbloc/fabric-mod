/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package service

import (
	"github.com/hyperledger/fabric/protos/gossip"
)

func HandleGossip(handle func(msg *gossip.GossipMessage)) func(msg *gossip.GossipMessage) {
	return handle
}
