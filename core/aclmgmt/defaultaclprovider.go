/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package aclmgmt

import (
	"fmt"

	"github.com/hyperledger/fabric-protos-go/common"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/common/policies"
	"github.com/hyperledger/fabric/core/aclmgmt/resources"
	"github.com/hyperledger/fabric/core/policy"
	"github.com/hyperledger/fabric/msp/mgmt"
	"github.com/hyperledger/fabric/protoutil"
)

const (
	CHANNELREADERS = policies.ChannelApplicationReaders
	CHANNELWRITERS = policies.ChannelApplicationWriters
)

type defaultACLProvider interface {
	ACLProvider
	IsPtypePolicy(resName string) bool
}

//defaultACLProvider used if resource-based ACL Provider is not provided or
//if it does not contain a policy for the named resource
type defaultACLProviderImpl struct {
	policyChecker policy.PolicyChecker

	//peer wide policy (currently not used)
	pResourcePolicyMap map[string]string

	//channel specific policy
	cResourcePolicyMap map[string]string
}

func newDefaultACLProvider(policyChecker policy.PolicyChecker) defaultACLProvider {
	d := &defaultACLProviderImpl{
		policyChecker:      policyChecker,
		pResourcePolicyMap: map[string]string{},
		cResourcePolicyMap: map[string]string{},
	}

	//-------------- _lifecycle --------------
	d.pResourcePolicyMap[resources.Lifecycle_InstallChaincode] = mgmt.Admins
	d.pResourcePolicyMap[resources.Lifecycle_QueryInstalledChaincode] = mgmt.Admins
	d.pResourcePolicyMap[resources.Lifecycle_GetInstalledChaincodePackage] = mgmt.Admins
	d.pResourcePolicyMap[resources.Lifecycle_QueryInstalledChaincodes] = mgmt.Admins
	d.pResourcePolicyMap[resources.Lifecycle_ApproveChaincodeDefinitionForMyOrg] = mgmt.Admins

	d.cResourcePolicyMap[resources.Lifecycle_CommitChaincodeDefinition] = CHANNELWRITERS
	d.cResourcePolicyMap[resources.Lifecycle_QueryChaincodeDefinition] = CHANNELWRITERS
	d.cResourcePolicyMap[resources.Lifecycle_QueryChaincodeDefinitions] = CHANNELWRITERS
	d.cResourcePolicyMap[resources.Lifecycle_CheckCommitReadiness] = CHANNELWRITERS

	//-------------- LSCC --------------
	//p resources (implemented by the chaincode currently)
	d.pResourcePolicyMap[resources.Lscc_Install] = mgmt.Admins
	d.pResourcePolicyMap[resources.Lscc_GetInstalledChaincodes] = mgmt.Admins

	//c resources
	d.cResourcePolicyMap[resources.Lscc_Deploy] = ""  //ACL check covered by PROPOSAL
	d.cResourcePolicyMap[resources.Lscc_Upgrade] = "" //ACL check covered by PROPOSAL
	d.cResourcePolicyMap[resources.Lscc_ChaincodeExists] = CHANNELREADERS
	d.cResourcePolicyMap[resources.Lscc_GetDeploymentSpec] = CHANNELREADERS
	d.cResourcePolicyMap[resources.Lscc_GetChaincodeData] = CHANNELREADERS
	d.cResourcePolicyMap[resources.Lscc_GetInstantiatedChaincodes] = CHANNELREADERS
	d.cResourcePolicyMap[resources.Lscc_GetCollectionsConfig] = CHANNELREADERS

	//-------------- QSCC --------------
	//p resources (none)

	//c resources
	d.cResourcePolicyMap[resources.Qscc_GetChainInfo] = CHANNELREADERS
	d.cResourcePolicyMap[resources.Qscc_GetBlockByNumber] = CHANNELREADERS
	d.cResourcePolicyMap[resources.Qscc_GetBlockByHash] = CHANNELREADERS
	d.cResourcePolicyMap[resources.Qscc_GetTransactionByID] = CHANNELREADERS
	d.cResourcePolicyMap[resources.Qscc_GetBlockByTxID] = CHANNELREADERS

	//--------------- CSCC resources -----------
	//p resources (implemented by the chaincode currently)
	d.pResourcePolicyMap[resources.Cscc_JoinChain] = mgmt.Admins
	d.pResourcePolicyMap[resources.Cscc_GetChannels] = mgmt.Members

	//c resources
	d.cResourcePolicyMap[resources.Cscc_GetConfigBlock] = CHANNELREADERS

	//---------------- non-scc resources ------------
	//Peer resources
	d.cResourcePolicyMap[resources.Peer_Propose] = CHANNELWRITERS
	d.cResourcePolicyMap[resources.Peer_ChaincodeToChaincode] = CHANNELWRITERS

	//Event resources
	d.cResourcePolicyMap[resources.Event_Block] = CHANNELREADERS
	d.cResourcePolicyMap[resources.Event_FilteredBlock] = CHANNELREADERS

	return d
}

func (d *defaultACLProviderImpl) IsPtypePolicy(resName string) bool {
	_, ok := d.pResourcePolicyMap[resName]
	return ok
}

// CheckACL provides default (v 1.0) behavior by mapping resources to their ACL for a channel.
func (d *defaultACLProviderImpl) CheckACL(resName string, channelID string, idinfo interface{}) error {
	//the default behavior is to use p type if defined and use channeless policy checks
	policy := d.pResourcePolicyMap[resName]
	if policy != "" {
		channelID = ""
	} else {
		policy = d.cResourcePolicyMap[resName]
		if policy == "" {
			aclLogger.Errorf("Unmapped policy for %s", resName)
			return fmt.Errorf("Unmapped policy for %s", resName)
		}
	}

	switch typedData := idinfo.(type) {
	case *pb.SignedProposal:
		return d.policyChecker.CheckPolicy(channelID, policy, typedData)
	case *common.Envelope:
		sd, err := protoutil.EnvelopeAsSignedData(typedData)
		if err != nil {
			return err
		}
		return d.policyChecker.CheckPolicyBySignedData(channelID, policy, sd)
	case []*protoutil.SignedData:
		return d.policyChecker.CheckPolicyBySignedData(channelID, policy, typedData)
	default:
		aclLogger.Errorf("Unmapped id on checkACL %s", resName)
		return fmt.Errorf("Unknown id on checkACL %s", resName)
	}
}
