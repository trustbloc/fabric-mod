/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cscc

import (
	"github.com/hyperledger/fabric/bccsp"
	"github.com/hyperledger/fabric/core/aclmgmt"
	"github.com/hyperledger/fabric/core/committer/txvalidator/v20/plugindispatcher"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/peer"
	"github.com/hyperledger/fabric/core/policy"
	"github.com/hyperledger/fabric/core/scc/cscc"
)

// New creates a new instance of the PeerConfiger.
func New(
	aclProvider aclmgmt.ACLProvider,
	deployedCCInfoProvider ledger.DeployedChaincodeInfoProvider,
	lr plugindispatcher.LifecycleResources,
	nr plugindispatcher.CollectionAndLifecycleResources,
	policyChecker policy.PolicyChecker,
	p *peer.Peer,
	bccsp bccsp.BCCSP,
) *cscc.PeerConfiger {
	return cscc.New(aclProvider, deployedCCInfoProvider, lr, nr, policyChecker, p, bccsp, nil)
}
