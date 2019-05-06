/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package dissemination

import (
	"github.com/hyperledger/fabric/gossip/gossip"
	"github.com/hyperledger/fabric/gossip/protoext"
)

// Plan contains the dissemination plan for extensions private data types
type Plan struct {
	Msg      *protoext.SignedGossipMessage
	Criteria gossip.SendCriteria
}
