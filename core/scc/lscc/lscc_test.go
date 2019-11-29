/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package lscc

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-chaincode-go/shimtest"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/ledger/queryresult"
	mb "github.com/hyperledger/fabric-protos-go/msp"
	"github.com/hyperledger/fabric-protos-go/peer"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/bccsp/sw"
	"github.com/hyperledger/fabric/common/cauthdsl"
	"github.com/hyperledger/fabric/common/mocks/config"
	mscc "github.com/hyperledger/fabric/common/mocks/scc"
	"github.com/hyperledger/fabric/common/policies"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/core/aclmgmt/mocks"
	"github.com/hyperledger/fabric/core/aclmgmt/resources"
	"github.com/hyperledger/fabric/core/chaincode/lifecycle"
	"github.com/hyperledger/fabric/core/common/ccprovider"
	"github.com/hyperledger/fabric/core/container"
	"github.com/hyperledger/fabric/core/container/externalbuilder"
	"github.com/hyperledger/fabric/core/ledger/ledgermgmt"
	"github.com/hyperledger/fabric/core/ledger/ledgermgmt/ledgermgmttest"
	"github.com/hyperledger/fabric/core/policy"
	policymocks "github.com/hyperledger/fabric/core/policy/mocks"
	"github.com/hyperledger/fabric/core/scc/lscc/mock"
	xtestutil "github.com/hyperledger/fabric/extensions/testutil"
	"github.com/hyperledger/fabric/msp"
	mspmgmt "github.com/hyperledger/fabric/msp/mgmt"
	msptesttools "github.com/hyperledger/fabric/msp/mgmt/testtools"
	mspmocks "github.com/hyperledger/fabric/msp/mocks"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// create a valid SignaturePolicyEnvelope to be used in tests
var testPolicyEnvelope = &common.SignaturePolicyEnvelope{
	Version: 0,
	Rule:    cauthdsl.NOutOf(1, []*common.SignaturePolicy{cauthdsl.SignedBy(0)}),
	Identities: []*mb.MSPPrincipal{
		{
			PrincipalClassification: mb.MSPPrincipal_ORGANIZATION_UNIT,
			Principal:               protoutil.MarshalOrPanic(&mb.OrganizationUnit{MspIdentifier: "Org1"}),
		},
	},
}

func constructDeploymentSpec(name, path, version string, initArgs [][]byte, createInvalidIndex bool, createFS bool, scc *SCC) (*pb.ChaincodeDeploymentSpec, error) {
	spec := &pb.ChaincodeSpec{Type: pb.ChaincodeSpec_GOLANG, ChaincodeId: &pb.ChaincodeID{Name: name, Path: path, Version: version}, Input: &pb.ChaincodeInput{Args: initArgs}}

	codePackageBytes := bytes.NewBuffer(nil)
	gz := gzip.NewWriter(codePackageBytes)
	tw := tar.NewWriter(gz)

	payload := []byte(name + path + version)
	err := tw.WriteHeader(&tar.Header{
		Name: "src/garbage.go",
		Size: int64(len(payload)),
		Mode: 0100644,
	})
	if err != nil {
		return nil, err
	}

	_, err = tw.Write(payload)
	if err != nil {
		return nil, err
	}

	// create an invalid couchdb index definition for negative testing
	if createInvalidIndex {
		payload := []byte("invalid index definition")
		err := tw.WriteHeader(&tar.Header{
			Name: "META-INF/statedb/couchdb/indexes/badIndex.json",
			Size: int64(len(payload)),
			Mode: 0100644,
		})
		if err != nil {
			return nil, err
		}

		_, err = tw.Write(payload)
		if err != nil {
			return nil, err
		}
	}

	tw.Close()
	gz.Close()

	depSpec := &pb.ChaincodeDeploymentSpec{ChaincodeSpec: spec, CodePackage: codePackageBytes.Bytes()}

	if createFS {
		buf, err := proto.Marshal(depSpec)
		if err != nil {
			return nil, err
		}

		cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
		if err != nil {
			return nil, err
		}
		cccdspack := &ccprovider.CDSPackage{GetHasher: cryptoProvider}
		if _, err := cccdspack.InitFromBuffer(buf); err != nil {
			return nil, err
		}

		scc.Support.(*MockSupport).GetChaincodeFromLocalStorageRv = cccdspack
		scc.Support.(*MockSupport).GetChaincodeFromLocalStorageErr = nil
		scc.Support.(*MockSupport).GetChaincodesFromLocalStorageRv = &pb.ChaincodeQueryResponse{Chaincodes: []*pb.ChaincodeInfo{{}}}
		scc.Support.(*MockSupport).GetChaincodesFromLocalStorageErr = nil
	} else {
		scc.Support.(*MockSupport).GetChaincodeFromLocalStorageRv = nil
		scc.Support.(*MockSupport).GetChaincodeFromLocalStorageErr = errors.New("barf")
		scc.Support.(*MockSupport).GetChaincodesFromLocalStorageRv = nil
		scc.Support.(*MockSupport).GetChaincodesFromLocalStorageErr = errors.New("barf")
	}

	return depSpec, nil
}

func getMSPIDs(cid string) []string {
	return nil
}

// TestInstall tests the install function with various inputs
func TestInstall(t *testing.T) {
	// Initialize ledgermgmt that inturn initializes internal components (such as cceventmgmt on which this test depends)
	tempdir, err := ioutil.TempDir("", "lscc-test")
	require.NoError(t, err, "failed to create temporary directory")
	defer os.RemoveAll(tempdir)

	initializer := ledgermgmttest.NewInitializer(tempdir)

	ledgerMgr := ledgermgmt.NewLedgerMgr(initializer)
	defer ledgerMgr.Close()

	chaincodeBuilder := &mock.ChaincodeBuilder{}

	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	scc := &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: chaincodeBuilder,
		EbMetadataProvider: &externalbuilder.MetadataProvider{
			DurablePath: "testdata",
		},
	}
	stub := shimtest.NewMockStub("lscc", scc)
	res := stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)

	res = stub.MockInvokeWithSignedProposal("1", [][]byte{}, nil)
	assert.NotEqual(t, int32(shim.OK), res.Status)
	assert.Equal(t, "invalid number of arguments to lscc: 0", res.Message)

	res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte("install")}, nil)
	assert.NotEqual(t, int32(shim.OK), res.Status)
	assert.Equal(t, "invalid number of arguments to lscc: 1", res.Message)

	res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte("install")}, nil)
	assert.NotEqual(t, int32(shim.OK), res.Status)
	assert.Equal(t, "invalid number of arguments to lscc: 1", res.Message)

	path := "mychaincode"

	testInstall(t, "example02", "0", path, false, "", "Alice", scc, stub, nil)

	assert.Equal(t, 1, chaincodeBuilder.BuildCallCount())
	assert.Equal(t, "example02:0", chaincodeBuilder.BuildArgsForCall(0))

	// Re-install, should not build a second time
	testInstall(t, "example02", "0", path, false, "", "Alice", scc, stub, nil)
	assert.Equal(t, 1, chaincodeBuilder.BuildCallCount())

	chaincodeBuilder.BuildReturns(fmt.Errorf("fake-build-error"))
	testInstall(t, "example02-different", "0", path, false, "could not build chaincode: fake-build-error", "Alice", scc, stub, nil)
	chaincodeBuilder.BuildReturns(nil)

	// This is a bad test, but it does at least exercise the external builder md path
	// The integration tests will ultimately ensure that it actually works.
	testInstall(t, "external-built", "cc", path, false, "", "Alice", scc, stub, nil)

	testInstall(t, "example02-2", "1.0", path, false, "", "Alice", scc, stub, nil)
	testInstall(t, "example02.go", "0", path, false, InvalidChaincodeNameErr("example02.go").Error(), "Alice", scc, stub, nil)
	testInstall(t, "", "0", path, false, InvalidChaincodeNameErr("").Error(), "Alice", scc, stub, nil)
	testInstall(t, "example02", "1{}0", path, false, InvalidVersionErr("1{}0").Error(), "Alice", scc, stub, nil)
	testInstall(t, "example02", "0", path, true, InvalidStatedbArtifactsErr("").Error(), "Alice", scc, stub, nil)
	testInstall(t, "example02", "0", path, false, "access denied for [install]", "Bob", scc, stub, errors.New("authorization error"))
	testInstall(t, "example02-2", "1.0-alpha+001", path, false, "", "Alice", scc, stub, nil)
	testInstall(t, "example02-2", "1.0+sha.c0ffee", path, false, "", "Alice", scc, stub, nil)

	scc.Support.(*MockSupport).PutChaincodeToLocalStorageErr = errors.New("barf")

	testInstall(t, "example02", "0", path, false, "barf", "Alice", scc, stub, nil)
	testInstall(t, "lscc", "0", path, false, "cannot install: lscc is the name of a system chaincode", "Alice", scc, stub, nil)
}

func testInstall(t *testing.T, ccname string, version string, path string, createInvalidIndex bool, expectedErrorMsg string, caller string, scc *SCC, stub *shimtest.MockStub, aclErr error) {
	t.Run(ccname+":"+version, func(t *testing.T) {
		identityDeserializer := &policymocks.MockIdentityDeserializer{
			Identity: []byte("Alice"),
			Msg:      []byte("msg1"),
		}
		policyManagerGetter := &policymocks.MockChannelPolicyManagerGetter{
			Managers: map[string]policies.Manager{
				"test": &policymocks.MockChannelPolicyManager{MockPolicy: &policymocks.MockPolicy{Deserializer: identityDeserializer}},
			},
		}
		scc.PolicyChecker = policy.NewPolicyChecker(
			policyManagerGetter,
			identityDeserializer,
			&policymocks.MockMSPPrincipalGetter{Principal: []byte("Alice")},
		)

		cds, err := constructDeploymentSpec(ccname, path, version, [][]byte{[]byte("init"), []byte("a"), []byte("100"), []byte("b"), []byte("200")}, createInvalidIndex, false, scc)
		assert.NoError(t, err)
		cdsBytes := protoutil.MarshalOrPanic(cds)

		// constructDeploymentSpec puts the depspec on the FS. This should succeed
		args := [][]byte{[]byte("install"), cdsBytes}

		sProp, _ := protoutil.MockSignedEndorserProposalOrPanic("", &pb.ChaincodeSpec{}, []byte(caller), []byte("msg1"))
		identityDeserializer.Msg = sProp.ProposalBytes
		sProp.Signature = sProp.ProposalBytes

		mockAclProvider.Reset()
		mockAclProvider.On("CheckACL", resources.Lscc_Install, "", sProp).Return(aclErr)

		if expectedErrorMsg == "" {
			res := stub.MockInvokeWithSignedProposal("1", args, sProp)
			assert.Equal(t, int32(shim.OK), res.Status, res.Message)
		} else {
			res := stub.MockInvokeWithSignedProposal("1", args, sProp)
			assert.True(t, strings.HasPrefix(string(res.Message), expectedErrorMsg), res.Message)
		}
	})
}

func TestNewLifecycleEnabled(t *testing.T) {
	// Enable PrivateChannelData
	mocksccProvider := (&mscc.MocksccProviderFactory{
		ApplicationConfigBool: true,
		ApplicationConfigRv: &config.MockApplication{
			CapabilitiesRv: &config.MockApplicationCapabilities{
				LifecycleV20Rv: true,
			},
		},
	}).NewSystemChaincodeProvider().(*mscc.MocksccProviderImpl)

	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	scc := &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &SupportImpl{GetMSPIDs: getMSPIDs},
		SCCProvider:      mocksccProvider,
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub := shimtest.NewMockStub("lscc", scc)
	res := stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte("deploy"), []byte("test"), nil}, nil)
	assert.NotEqual(t, int32(shim.OK), res.Status)
	assert.Equal(t, "Channel 'test' has been migrated to the new lifecycle, LSCC is now read-only", res.Message)

}

func TestDeploy(t *testing.T) {
	path := "mychaincode"

	testDeploy(t, "example02", "0", path, false, false, true, "", nil, nil, nil)
	testDeploy(t, "example02", "1.0", path, false, false, true, "", nil, nil, nil)
	testDeploy(t, "example02", "1.0", path, false, false, false, "cannot get package for chaincode (example02:1.0)", nil, nil, nil)
	testDeploy(t, "example02", "0", path, true, false, true, InvalidChaincodeNameErr("").Error(), nil, nil, nil)
	testDeploy(t, "example02", "0", path, false, true, true, InvalidVersionErr("").Error(), nil, nil, nil)
	testDeploy(t, "example02.go", "0", path, false, false, true, InvalidChaincodeNameErr("example02.go").Error(), nil, nil, nil)
	testDeploy(t, "example02", "1{}0", path, false, false, true, InvalidVersionErr("1{}0").Error(), nil, nil, nil)
	testDeploy(t, "example02", "0", path, true, true, true, InvalidChaincodeNameErr("").Error(), nil, nil, nil)

	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	scc := &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		SCCProvider:      NewMockProvider(),
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub := shimtest.NewMockStub("lscc", scc)
	res := stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)

	res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte("deploy")}, nil)
	assert.NotEqual(t, int32(shim.OK), res.Status)
	assert.Equal(t, "invalid number of arguments to lscc: 1", res.Message)

	res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte("deploy"), []byte(""), []byte("")}, nil)
	assert.NotEqual(t, int32(shim.OK), res.Status)
	assert.Equal(t, "invalid channel name: ", res.Message)

	res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte("deploy"), []byte("chain"), []byte("barf")}, nil)
	assert.NotEqual(t, int32(shim.OK), res.Status)
	assert.Equal(t, "error unmarshaling ChaincodeDeploymentSpec: unexpected EOF", res.Message)

	testDeploy(t, "example02", "1.0", path, false, false, true, "", scc, stub, nil)
	testDeploy(t, "example02", "1.0", path, false, false, true, "chaincode with name 'example02' already exists", scc, stub, nil)

	scc = &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		SCCProvider:      NewMockProvider(),
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub = shimtest.NewMockStub("lscc", scc)
	res = stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)
	scc.Support.(*MockSupport).GetInstantiationPolicyErr = errors.New("barf")

	testDeploy(t, "example02", "1.0", path, false, false, true, "barf", scc, stub, nil)

	scc = &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		SCCProvider:      NewMockProvider(),
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub = shimtest.NewMockStub("lscc", scc)
	res = stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)
	scc.Support.(*MockSupport).CheckInstantiationPolicyErr = errors.New("barf")

	testDeploy(t, "example02", "1.0", path, false, false, true, "barf", scc, stub, nil)

	scc = &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		SCCProvider:      NewMockProvider(),
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub = shimtest.NewMockStub("lscc", scc)
	res = stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)

	// As the PrivateChannelData is disabled, the following error message is expected due to the presence of
	// collectionConfigBytes in the stub.args
	errMessage := InvalidArgsLenErr(7).Error()
	testDeploy(t, "example02", "1.0", path, false, false, true, PrivateChannelDataNotAvailable("").Error(), scc, stub, []byte("collections"))

	// Enable PrivateChannelData
	mocksccProvider := (&mscc.MocksccProviderFactory{
		ApplicationConfigBool: true,
		ApplicationConfigRv: &config.MockApplication{
			CapabilitiesRv: &config.MockApplicationCapabilities{
				PrivateChannelDataRv: true,
			},
		},
	}).NewSystemChaincodeProvider().(*mscc.MocksccProviderImpl)

	scc = &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		SCCProvider:      mocksccProvider,
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub = shimtest.NewMockStub("lscc", scc)
	res = stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)

	// As the PrivateChannelData is enabled and collectionConfigBytes is invalid, the following error
	// message is expected.
	errMessage = "invalid collection configuration supplied for chaincode example02:1.0"
	testDeploy(t, "example02", "1.0", path, false, false, true, errMessage, scc, stub, []byte("invalid collection"))
	// Should contain an entry for the chaincodeData only
	assert.Equal(t, 1, len(stub.State))
	_, ok := stub.State["example02"]
	assert.Equal(t, true, ok)

	collName1 := "mycollection1"
	policyEnvelope := cauthdsl.SignedByAnyMember([]string{"SampleOrg"})
	var requiredPeerCount, maximumPeerCount int32
	requiredPeerCount = 1
	maximumPeerCount = 2
	coll1 := createCollectionConfig(collName1, policyEnvelope, requiredPeerCount, maximumPeerCount)

	ccp := &peer.CollectionConfigPackage{Config: []*peer.CollectionConfig{coll1}}
	ccpBytes, err := proto.Marshal(ccp)
	assert.NoError(t, err)
	assert.NotNil(t, ccpBytes)

	scc = &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		SCCProvider:      mocksccProvider,
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub = shimtest.NewMockStub("lscc", scc)
	res = stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)

	// As the PrivateChannelData is enabled and collectionConfigBytes is valid, no error is expected
	testDeploy(t, "example02", "1.0", path, false, false, true, "", scc, stub, ccpBytes)
	// Should contain two entries: one for the chaincodeData and another for the collectionConfigBytes
	assert.Equal(t, 2, len(stub.State))
	_, ok = stub.State["example02"]
	assert.Equal(t, true, ok)
	actualccpBytes, ok := stub.State["example02~collection"]
	assert.Equal(t, true, ok)
	assert.Equal(t, ccpBytes, actualccpBytes)

	scc = &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		SCCProvider:      mocksccProvider,
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub = shimtest.NewMockStub("lscc", scc)
	res = stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)

	// As the PrivateChannelData is enabled and collectionConfigBytes is nil, no error is expected
	testDeploy(t, "example02", "1.0", path, false, false, true, "", scc, stub, []byte("nil"))
	// Should contain an entry for the chaincodeData only. As the collectionConfigBytes is nil, it
	// is ignored
	assert.Equal(t, 1, len(stub.State))
	_, ok = stub.State["example02"]
	assert.Equal(t, true, ok)
}

func createCollectionConfig(collectionName string, signaturePolicyEnvelope *common.SignaturePolicyEnvelope,
	requiredPeerCount int32, maximumPeerCount int32,
) *peer.CollectionConfig {
	signaturePolicy := &peer.CollectionPolicyConfig_SignaturePolicy{
		SignaturePolicy: signaturePolicyEnvelope,
	}
	accessPolicy := &peer.CollectionPolicyConfig{
		Payload: signaturePolicy,
	}

	return &peer.CollectionConfig{
		Payload: &peer.CollectionConfig_StaticCollectionConfig{
			StaticCollectionConfig: &peer.StaticCollectionConfig{
				Name:              collectionName,
				MemberOrgsPolicy:  accessPolicy,
				RequiredPeerCount: requiredPeerCount,
				MaximumPeerCount:  maximumPeerCount,
			},
		},
	}
}

func testDeploy(t *testing.T, ccname string, version string, path string, forceBlankCCName bool, forceBlankVersion bool, install bool, expectedErrorMsg string, scc *SCC, stub *shimtest.MockStub, collectionConfigBytes []byte) {
	if scc == nil {
		cryptoProvider, _ := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
		scc = &SCC{
			BuiltinSCCs:      map[string]struct{}{"lscc": {}},
			Support:          &MockSupport{},
			SCCProvider:      NewMockProvider(),
			ACLProvider:      mockAclProvider,
			GetMSPIDs:        getMSPIDs,
			BCCSP:            cryptoProvider,
			BuildRegistry:    &container.BuildRegistry{},
			ChaincodeBuilder: &mock.ChaincodeBuilder{},
		}
		stub = shimtest.NewMockStub("lscc", scc)
		res := stub.MockInit("1", nil)
		assert.Equal(t, int32(shim.OK), res.Status, res.Message)
	}
	stub.ChannelID = channelID

	identityDeserializer := &policymocks.MockIdentityDeserializer{Identity: []byte("Alice"), Msg: []byte("msg1")}
	policyManagerGetter := &policymocks.MockChannelPolicyManagerGetter{
		Managers: map[string]policies.Manager{
			"test": &policymocks.MockChannelPolicyManager{MockPolicy: &policymocks.MockPolicy{Deserializer: identityDeserializer}},
		},
	}
	scc.PolicyChecker = policy.NewPolicyChecker(
		policyManagerGetter,
		identityDeserializer,
		&policymocks.MockMSPPrincipalGetter{Principal: []byte("Alice")},
	)
	sProp, _ := protoutil.MockSignedEndorserProposalOrPanic(channelID, &pb.ChaincodeSpec{}, []byte("Alice"), []byte("msg1"))
	identityDeserializer.Msg = sProp.ProposalBytes
	sProp.Signature = sProp.ProposalBytes

	cds, err := constructDeploymentSpec(ccname, path, version, [][]byte{[]byte("init"), []byte("a"), []byte("100"), []byte("b"), []byte("200")}, false, install, scc)
	assert.NoError(t, err)

	if forceBlankCCName {
		cds.ChaincodeSpec.ChaincodeId.Name = ""
	}
	if forceBlankVersion {
		cds.ChaincodeSpec.ChaincodeId.Version = ""
	}
	cdsBytes := protoutil.MarshalOrPanic(cds)

	sProp2, _ := protoutil.MockSignedEndorserProposal2OrPanic(channelID, &pb.ChaincodeSpec{}, id)
	var args [][]byte
	if len(collectionConfigBytes) > 0 {
		if bytes.Equal(collectionConfigBytes, []byte("nil")) {
			args = [][]byte{[]byte("deploy"), []byte("test"), cdsBytes, nil, []byte("escc"), []byte("vscc"), nil}
		} else {
			args = [][]byte{[]byte("deploy"), []byte("test"), cdsBytes, nil, []byte("escc"), []byte("vscc"), collectionConfigBytes}
		}
	} else {
		args = [][]byte{[]byte("deploy"), []byte("test"), cdsBytes}
	}
	res := stub.MockInvokeWithSignedProposal("1", args, sProp2)

	if expectedErrorMsg == "" {
		assert.Equal(t, int32(shim.OK), res.Status, res.Message)

		for _, function := range []string{"getchaincodes", "GetChaincodes"} {
			t.Run(function, func(t *testing.T) {
				mockAclProvider.Reset()
				mockAclProvider.On("CheckACL", resources.Lscc_GetInstantiatedChaincodes, channelID, sProp).Return(nil)
				args = [][]byte{[]byte(function)}
				res = stub.MockInvokeWithSignedProposal("1", args, sProp)
				assert.Equal(t, int32(shim.OK), res.Status, res.Message)
			})
		}
		for _, function := range []string{"getid", "ChaincodeExists"} {
			t.Run(function, func(t *testing.T) {
				mockAclProvider.Reset()
				mockAclProvider.On("CheckACL", resources.Lscc_ChaincodeExists, "test", sProp).Return(nil)
				args = [][]byte{[]byte(function), []byte("test"), []byte(cds.ChaincodeSpec.ChaincodeId.Name)}
				res = stub.MockInvokeWithSignedProposal("1", args, sProp)
				assert.Equal(t, int32(shim.OK), res.Status, res.Message)
			})
		}
		for _, function := range []string{"getdepspec", "GetDeploymentSpec"} {
			t.Run(function, func(t *testing.T) {
				mockAclProvider.Reset()
				mockAclProvider.On("CheckACL", resources.Lscc_GetDeploymentSpec, "test", sProp).Return(nil)
				args = [][]byte{[]byte(function), []byte("test"), []byte(cds.ChaincodeSpec.ChaincodeId.Name)}
				res = stub.MockInvokeWithSignedProposal("1", args, sProp)
				assert.Equal(t, int32(shim.OK), res.Status, res.Message)
				scc.Support.(*MockSupport).GetChaincodeFromLocalStorageErr = errors.New("barf")
				res = stub.MockInvokeWithSignedProposal("1", args, sProp)
				assert.NotEqual(t, int32(shim.OK), res.Status)
				assert.Equal(t, "invalid deployment spec: barf", res.Message)
				scc.Support.(*MockSupport).GetChaincodeFromLocalStorageErr = nil
				bkpCCFromLSRv := scc.Support.(*MockSupport).GetChaincodeFromLocalStorageRv
				cryptoProvider, _ := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
				scc.Support.(*MockSupport).GetChaincodeFromLocalStorageRv = &ccprovider.CDSPackage{GetHasher: cryptoProvider}
				res = stub.MockInvokeWithSignedProposal("1", args, sProp)
				assert.NotEqual(t, int32(shim.OK), res.Status)
				assert.Contains(t, res.Message, "chaincode fingerprint mismatch")
				scc.Support.(*MockSupport).GetChaincodeFromLocalStorageRv = bkpCCFromLSRv
			})
		}

		for _, function := range []string{"getccdata", "GetChaincodeData"} {
			t.Run(function, func(t *testing.T) {
				mockAclProvider.Reset()
				mockAclProvider.On("CheckACL", resources.Lscc_GetChaincodeData, "test", sProp).Return(nil)
				args = [][]byte{[]byte(function), []byte("test"), []byte(cds.ChaincodeSpec.ChaincodeId.Name)}
				res = stub.MockInvokeWithSignedProposal("1", args, sProp)
				assert.Equal(t, int32(shim.OK), res.Status, res.Message)
			})
		}
	} else {
		assert.Equal(t, expectedErrorMsg, string(res.Message))
	}
}

// TestUpgrade tests the upgrade function with various inputs for basic use cases
func TestUpgrade(t *testing.T) {
	path := "mychaincode"

	testUpgrade(t, "example02", "0", "example02", "1", path, "", nil, nil, nil)
	testUpgrade(t, "example02", "0", "example02", "", path, InvalidVersionErr("").Error(), nil, nil, nil)
	testUpgrade(t, "example02", "0", "example02", "0", path, IdenticalVersionErr("example02").Error(), nil, nil, nil)
	testUpgrade(t, "example02", "0", "example03", "1", path, NotFoundErr("example03").Error(), nil, nil, nil)
	testUpgrade(t, "example02", "0", "example02", "1{}0", path, InvalidVersionErr("1{}0").Error(), nil, nil, nil)
	testUpgrade(t, "example02", "0", "example*02", "1{}0", path, InvalidChaincodeNameErr("example*02").Error(), nil, nil, nil)
	testUpgrade(t, "example02", "0", "", "1", path, InvalidChaincodeNameErr("").Error(), nil, nil, nil)

	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	scc := &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		SCCProvider:      NewMockProvider(),
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub := shimtest.NewMockStub("lscc", scc)
	res := stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)
	scc.Support.(*MockSupport).GetInstantiationPolicyRv = []byte("instantiation policy")
	scc.Support.(*MockSupport).GetInstantiationPolicyErr = errors.New("barf")

	testUpgrade(t, "example02", "0", "example02", "1", path, "barf", scc, stub, nil)

	scc = &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		SCCProvider:      NewMockProvider(),
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub = shimtest.NewMockStub("lscc", scc)
	res = stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)

	testUpgrade(t, "example02", "0", "example02", "1", path, "instantiation policy missing", scc, stub, nil)

	scc = &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		SCCProvider:      NewMockProvider(),
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub = shimtest.NewMockStub("lscc", scc)
	res = stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)
	scc.Support.(*MockSupport).GetInstantiationPolicyRv = []byte("instantiation policy")
	scc.Support.(*MockSupport).GetInstantiationPolicyMap = map[string][]byte{}
	scc.Support.(*MockSupport).CheckInstantiationPolicyMap = map[string]error{"example020": errors.New("barf")}

	testUpgrade(t, "example02", "0", "example02", "1", path, "barf", scc, stub, nil)

	scc = &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		SCCProvider:      NewMockProvider(),
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub = shimtest.NewMockStub("lscc", scc)
	res = stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)
	scc.Support.(*MockSupport).GetInstantiationPolicyRv = []byte("instantiation policy")
	scc.Support.(*MockSupport).GetInstantiationPolicyMap = map[string][]byte{}
	scc.Support.(*MockSupport).CheckInstantiationPolicyMap = map[string]error{"example021": errors.New("barf")}

	testUpgrade(t, "example02", "0", "example02", "1", path, "barf", scc, stub, nil)

	// Enable PrivateChannelData
	mocksccProvider := (&mscc.MocksccProviderFactory{
		ApplicationConfigBool: true,
		ApplicationConfigRv: &config.MockApplication{
			CapabilitiesRv: &config.MockApplicationCapabilities{
				PrivateChannelDataRv: true,
			},
		},
	}).NewSystemChaincodeProvider().(*mscc.MocksccProviderImpl)

	scc = &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		SCCProvider:      mocksccProvider,
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub = shimtest.NewMockStub("lscc", scc)
	res = stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)
	scc.Support.(*MockSupport).GetInstantiationPolicyRv = []byte("instantiation policy")

	collName1 := "mycollection1"
	var requiredPeerCount, maximumPeerCount int32
	requiredPeerCount = 1
	maximumPeerCount = 2
	coll1 := createCollectionConfig(collName1, testPolicyEnvelope, requiredPeerCount, maximumPeerCount)

	ccp := &peer.CollectionConfigPackage{Config: []*peer.CollectionConfig{coll1}}
	ccpBytes, err := proto.Marshal(ccp)
	assert.NoError(t, err)
	assert.NotNil(t, ccpBytes)

	// As v12 capability is not enabled (which is required for the collection upgrade), an error is expected
	expectedErrorMsg := "as V1_2 capability is not enabled, collection upgrades are not allowed"
	testUpgrade(t, "example02", "0", "example02", "1", path, expectedErrorMsg, scc, stub, ccpBytes)

	// Enable PrivateChannelData and V1_2Validation
	mocksccProvider = (&mscc.MocksccProviderFactory{
		ApplicationConfigBool: true,
		ApplicationConfigRv: &config.MockApplication{
			CapabilitiesRv: &config.MockApplicationCapabilities{
				PrivateChannelDataRv: true,
				CollectionUpgradeRv:  true,
			},
		},
	}).NewSystemChaincodeProvider().(*mscc.MocksccProviderImpl)

	scc = &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		SCCProvider:      mocksccProvider,
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub = shimtest.NewMockStub("lscc", scc)
	res = stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)
	scc.Support.(*MockSupport).GetInstantiationPolicyRv = []byte("instantiation policy")

	// As the PrivateChannelData is enabled and collectionConfigBytes is valid, no error is expected
	testUpgrade(t, "example02", "0", "example02", "1", path, "", scc, stub, []byte("nil"))
	// Should contain an entry for the chaincodeData only as the collectionConfigBytes is nil
	assert.Equal(t, 1, len(stub.State))
	_, ok := stub.State["example02"]
	assert.Equal(t, true, ok)

	scc = &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		SCCProvider:      mocksccProvider,
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub = shimtest.NewMockStub("lscc", scc)
	res = stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)
	scc.Support.(*MockSupport).GetInstantiationPolicyRv = []byte("instantiation policy")

	// As the PrivateChannelData is enabled and collectionConfigBytes is valid, no error is expected
	testUpgrade(t, "example02", "0", "example02", "1", path, "", scc, stub, ccpBytes)
	// Should contain two entries: one for the chaincodeData and another for the collectionConfigBytes
	// as the V1_2Validation is enabled. Only in V1_2Validation, collection upgrades are allowed.
	// Note that V1_2Validation would be replaced with CollectionUpgrade capability.
	assert.Equal(t, 2, len(stub.State))
	_, ok = stub.State["example02"]
	assert.Equal(t, true, ok)
	actualccpBytes, ok := stub.State["example02~collection"]
	assert.Equal(t, true, ok)
	assert.Equal(t, ccpBytes, actualccpBytes)
}

func testUpgrade(t *testing.T, ccname string, version string, newccname string, newversion string, path string, expectedErrorMsg string, scc *SCC, stub *shimtest.MockStub, collectionConfigBytes []byte) {
	t.Run(ccname+":"+version+"->"+newccname+":"+newversion, func(t *testing.T) {
		cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
		assert.NoError(t, err)
		if scc == nil {
			scc = &SCC{
				BuiltinSCCs:      map[string]struct{}{"lscc": {}},
				Support:          &MockSupport{},
				SCCProvider:      NewMockProvider(),
				ACLProvider:      mockAclProvider,
				GetMSPIDs:        getMSPIDs,
				BCCSP:            cryptoProvider,
				BuildRegistry:    &container.BuildRegistry{},
				ChaincodeBuilder: &mock.ChaincodeBuilder{},
			}
			stub = shimtest.NewMockStub("lscc", scc)
			res := stub.MockInit("1", nil)
			assert.Equal(t, int32(shim.OK), res.Status, res.Message)
			scc.Support.(*MockSupport).GetInstantiationPolicyRv = []byte("instantiation policy")
		}

		cds, err := constructDeploymentSpec(ccname, path, version, [][]byte{[]byte("init"), []byte("a"), []byte("100"), []byte("b"), []byte("200")}, false, true, scc)
		assert.NoError(t, err)
		cdsBytes := protoutil.MarshalOrPanic(cds)

		sProp, _ := protoutil.MockSignedEndorserProposal2OrPanic(channelID, &pb.ChaincodeSpec{}, id)
		args := [][]byte{[]byte("deploy"), []byte("test"), cdsBytes}
		saved1 := scc.Support.(*MockSupport).GetInstantiationPolicyErr
		saved2 := scc.Support.(*MockSupport).CheckInstantiationPolicyMap
		scc.Support.(*MockSupport).GetInstantiationPolicyErr = nil
		scc.Support.(*MockSupport).CheckInstantiationPolicyMap = nil
		res := stub.MockInvokeWithSignedProposal("1", args, sProp)
		assert.Equal(t, int32(shim.OK), res.Status, res.Message)
		scc.Support.(*MockSupport).GetInstantiationPolicyErr = saved1
		scc.Support.(*MockSupport).CheckInstantiationPolicyMap = saved2

		newCds, err := constructDeploymentSpec(newccname, path, newversion, [][]byte{[]byte("init"), []byte("a"), []byte("100"), []byte("b"), []byte("200")}, false, true, scc)
		assert.NoError(t, err)
		newCdsBytes := protoutil.MarshalOrPanic(newCds)

		if len(collectionConfigBytes) > 0 {
			if bytes.Equal(collectionConfigBytes, []byte("nil")) {
				args = [][]byte{[]byte("upgrade"), []byte("test"), newCdsBytes, nil, []byte("escc"), []byte("vscc"), nil}
			} else {
				args = [][]byte{[]byte("upgrade"), []byte("test"), newCdsBytes, nil, []byte("escc"), []byte("vscc"), collectionConfigBytes}
			}
		} else {
			args = [][]byte{[]byte("upgrade"), []byte("test"), newCdsBytes}
		}

		res = stub.MockInvokeWithSignedProposal("1", args, sProp)
		if expectedErrorMsg == "" {
			assert.Equal(t, int32(shim.OK), res.Status, res.Message)

			cd := &ccprovider.ChaincodeData{}
			err = proto.Unmarshal(res.Payload, cd)
			assert.NoError(t, err)

			newVer := cd.Version

			expectVer := "1"
			assert.Equal(t, newVer, expectVer, fmt.Sprintf("Upgrade chaincode version error, expected %s, got %s", expectVer, newVer))

			chaincodeEvent := <-stub.ChaincodeEventsChannel
			assert.Equal(t, "upgrade", chaincodeEvent.EventName)
			lifecycleEvent := &pb.LifecycleEvent{}
			err = proto.Unmarshal(chaincodeEvent.Payload, lifecycleEvent)
			assert.NoError(t, err)
			assert.Equal(t, newccname, lifecycleEvent.ChaincodeName)
		} else {
			assert.Equal(t, expectedErrorMsg, string(res.Message))
		}
	})
}

func TestFunctionsWithAliases(t *testing.T) {
	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	scc := &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub := shimtest.NewMockStub("lscc", scc)
	res := stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)

	identityDeserializer := &policymocks.MockIdentityDeserializer{Identity: []byte("Alice"), Msg: []byte("msg1")}
	policyManagerGetter := &policymocks.MockChannelPolicyManagerGetter{
		Managers: map[string]policies.Manager{
			"test": &policymocks.MockChannelPolicyManager{MockPolicy: &policymocks.MockPolicy{Deserializer: identityDeserializer}},
		},
	}
	scc.PolicyChecker = policy.NewPolicyChecker(
		policyManagerGetter,
		identityDeserializer,
		&policymocks.MockMSPPrincipalGetter{Principal: []byte("Alice")},
	)
	sProp, _ := protoutil.MockSignedEndorserProposalOrPanic("", &pb.ChaincodeSpec{}, []byte("Alice"), []byte("msg1"))
	identityDeserializer.Msg = sProp.ProposalBytes
	sProp.Signature = sProp.ProposalBytes

	testInvoke := func(function, resource string) {
		t.Run(function, func(t *testing.T) {
			res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte(function), []byte("testchannel1")}, nil)
			assert.NotEqual(t, int32(shim.OK), res.Status)
			assert.Equal(t, "invalid number of arguments to lscc: 2", res.Message)

			mockAclProvider.Reset()
			mockAclProvider.On("CheckACL", resource, "testchannel1", sProp).Return(errors.New("bonanza"))
			res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte(function), []byte("testchannel1"), []byte("chaincode")}, sProp)
			assert.NotEqual(t, int32(shim.OK), res.Status, res.Message)
			assert.Equal(t, fmt.Sprintf("access denied for [%s][testchannel1]: bonanza", function), res.Message)

			mockAclProvider.Reset()
			mockAclProvider.On("CheckACL", resource, "testchannel1", sProp).Return(nil)
			res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte(function), []byte("testchannel1"), []byte("nonexistentchaincode")}, sProp)
			assert.NotEqual(t, int32(shim.OK), res.Status, res.Message)
			assert.Equal(t, res.Message, "could not find chaincode with name 'nonexistentchaincode'")
		})
	}

	testInvoke("getid", "lscc/ChaincodeExists")
	testInvoke("ChaincodeExists", "lscc/ChaincodeExists")
	testInvoke("getdepspec", "lscc/GetDeploymentSpec")
	testInvoke("GetDeploymentSpec", "lscc/GetDeploymentSpec")
	testInvoke("getccdata", "lscc/GetChaincodeData")
	testInvoke("GetChaincodeData", "lscc/GetChaincodeData")
}

func TestGetChaincodes(t *testing.T) {
	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	scc := &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub := shimtest.NewMockStub("lscc", scc)
	stub.ChannelID = "test"
	res := stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)

	for _, function := range []string{"getchaincodes", "GetChaincodes"} {
		t.Run(function, func(t *testing.T) {
			res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte(function), []byte("barf")}, nil)
			assert.NotEqual(t, int32(shim.OK), res.Status)
			assert.Equal(t, "invalid number of arguments to lscc: 2", res.Message)

			sProp, _ := protoutil.MockSignedEndorserProposalOrPanic("test", &pb.ChaincodeSpec{}, []byte("Bob"), []byte("msg1"))
			sProp.Signature = sProp.ProposalBytes

			mockAclProvider.Reset()
			mockAclProvider.On("CheckACL", resources.Lscc_GetInstantiatedChaincodes, "test", sProp).Return(errors.New("coyote"))
			res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte(function)}, sProp)
			assert.NotEqual(t, int32(shim.OK), res.Status)
			assert.Regexp(t, `access denied for \[`+function+`\]\[test\](.*)coyote`, res.Message)

			mockAclProvider.Reset()
			mockAclProvider.On("CheckACL", resources.Lscc_GetInstantiatedChaincodes, "test", sProp).Return(nil)
			res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte(function)}, sProp)
			assert.Equal(t, int32(shim.OK), res.Status, res.Message)
		})
	}
}

func TestGetChaincodesFilter(t *testing.T) {
	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	scc := &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{GetChaincodeFromLocalStorageErr: errors.New("banana")},
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}

	sqi := &mock.StateQueryIterator{}
	results := []*queryresult.KV{
		{Key: "one", Value: protoutil.MarshalOrPanic(&ccprovider.ChaincodeData{Name: "name-one", Version: "1.0", Escc: "escc", Vscc: "vscc"})},
		{Key: "something~collections", Value: []byte("completely-ignored")},
		{Key: "two", Value: protoutil.MarshalOrPanic(&ccprovider.ChaincodeData{Name: "name-two", Version: "2.0", Escc: "escc-2", Vscc: "vscc-2"})},
	}
	for i, r := range results {
		sqi.NextReturnsOnCall(i, r, nil)
		sqi.HasNextReturnsOnCall(i, true)
	}

	stub := &mock.ChaincodeStub{}
	stub.GetStateByRangeReturns(sqi, nil)

	resp := scc.getChaincodes(stub)
	assert.Equal(t, resp.Status, int32(shim.OK))

	cqr := &pb.ChaincodeQueryResponse{}
	err = proto.Unmarshal(resp.GetPayload(), cqr)
	assert.NoError(t, err)

	assert.Equal(t, cqr.Chaincodes, []*pb.ChaincodeInfo{
		{Name: "name-one", Version: "1.0", Escc: "escc", Vscc: "vscc"},
		{Name: "name-two", Version: "2.0", Escc: "escc-2", Vscc: "vscc-2"},
	})
}

func TestGetInstalledChaincodes(t *testing.T) {
	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	scc := &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &MockSupport{},
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub := shimtest.NewMockStub("lscc", scc)
	res := stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)

	for _, function := range []string{"getinstalledchaincodes", "GetInstalledChaincodes"} {
		t.Run(function, func(t *testing.T) {
			res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte(function), []byte("barf")}, nil)
			assert.NotEqual(t, int32(shim.OK), res.Status)
			assert.Equal(t, "invalid number of arguments to lscc: 2", res.Message)

			identityDeserializer := &policymocks.MockIdentityDeserializer{Identity: []byte("Alice"), Msg: []byte("msg1")}
			policyManagerGetter := &policymocks.MockChannelPolicyManagerGetter{
				Managers: map[string]policies.Manager{
					"test": &policymocks.MockChannelPolicyManager{MockPolicy: &policymocks.MockPolicy{Deserializer: identityDeserializer}},
				},
			}
			scc.PolicyChecker = policy.NewPolicyChecker(
				policyManagerGetter,
				identityDeserializer,
				&policymocks.MockMSPPrincipalGetter{Principal: []byte("Alice")},
			)
			sProp, _ := protoutil.MockSignedEndorserProposalOrPanic("", &pb.ChaincodeSpec{}, []byte("Bob"), []byte("msg1"))
			identityDeserializer.Msg = sProp.ProposalBytes
			sProp.Signature = sProp.ProposalBytes

			mockAclProvider.Reset()
			mockAclProvider.On("CheckACL", resources.Lscc_GetInstalledChaincodes, "", sProp).Return(errors.New("authorization failure"))
			res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte(function)}, sProp)
			assert.NotEqual(t, int32(shim.OK), res.Status)
			assert.Contains(t, res.Message, "access denied for ["+function+"]")

			sProp, _ = protoutil.MockSignedEndorserProposalOrPanic("", &pb.ChaincodeSpec{}, []byte("Alice"), []byte("msg1"))
			identityDeserializer.Msg = sProp.ProposalBytes
			sProp.Signature = sProp.ProposalBytes

			mockAclProvider.Reset()
			mockAclProvider.On("CheckACL", resources.Lscc_GetInstalledChaincodes, "", sProp).Return(nil)
			res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte(function)}, sProp)
			assert.NotEqual(t, int32(shim.OK), res.Status)
			assert.Equal(t, "proto: Marshal called with nil", res.Message)

			_, err := constructDeploymentSpec("ccname-"+function, "path", "version", [][]byte{[]byte("init"), []byte("a"), []byte("100"), []byte("b"), []byte("200")}, false, false, scc)
			assert.NoError(t, err)

			res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte(function)}, sProp)
			assert.NotEqual(t, int32(shim.OK), res.Status)
			assert.Equal(t, "barf", res.Message)

			_, err = constructDeploymentSpec("ccname-"+function, "path", "version", [][]byte{[]byte("init"), []byte("a"), []byte("100"), []byte("b"), []byte("200")}, false, true, scc)
			assert.NoError(t, err)

			res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte(function)}, sProp)
			assert.Equal(t, int32(shim.OK), res.Status, res.Message)

			scc.Support = &MockSupport{}
		})
	}
}

func TestNewLifeCycleSysCC(t *testing.T) {
	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	scc := &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &SupportImpl{GetMSPIDs: getMSPIDs},
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	assert.NotNil(t, scc)
	stub := shimtest.NewMockStub("lscc", scc)
	res := stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)

	res = stub.MockInvokeWithSignedProposal("1", [][]byte{[]byte("barf")}, nil)
	assert.NotEqual(t, int32(shim.OK), res.Status)
	assert.Equal(t, "invalid function to lscc: barf", res.Message)
}

func TestGetChaincodeData(t *testing.T) {
	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	scc := &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &SupportImpl{GetMSPIDs: getMSPIDs},
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	assert.NotNil(t, scc)
	stub := shimtest.NewMockStub("lscc", scc)
	res := stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)

	_, err = scc.getChaincodeData("barf", []byte("barf"))
	assert.Error(t, err)

	_, err = scc.getChaincodeData("barf", protoutil.MarshalOrPanic(&ccprovider.ChaincodeData{Name: "barf s'more"}))
	assert.Error(t, err)
	assert.True(t, len(err.Error()) > 0)
}

func TestExecuteInstall(t *testing.T) {
	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	scc := &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &SupportImpl{GetMSPIDs: getMSPIDs},
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	assert.NotNil(t, scc)
	stub := shimtest.NewMockStub("lscc", scc)
	res := stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)

	err = scc.executeInstall(stub, []byte("barf"))
	assert.Error(t, err)
}

func TestErrors(t *testing.T) {
	// these errors are really hard (if
	// outright impossible without writing
	// tons of lines of mocking code) to
	// get in testing
	err1 := TXNotFoundErr("")
	assert.True(t, len(err1.Error()) > 0)

	err3 := MarshallErr("")
	assert.True(t, len(err3.Error()) > 0)
}

func TestPutChaincodeCollectionData(t *testing.T) {
	scc := &SCC{
		Support: &MockSupport{},
	}
	stub := shimtest.NewMockStub("lscc", scc)

	if res := stub.MockInit("1", nil); res.Status != shim.OK {
		fmt.Println("Init failed", string(res.Message))
		t.FailNow()
	}

	err := scc.putChaincodeCollectionData(stub, nil, nil)
	assert.Error(t, err)

	cd := &ccprovider.ChaincodeData{Name: "foo"}

	err = scc.putChaincodeCollectionData(stub, cd, nil)
	assert.NoError(t, err)

	collName1 := "mycollection1"
	coll1 := createCollectionConfig(collName1, testPolicyEnvelope, 1, 2)
	ccp := &peer.CollectionConfigPackage{Config: []*peer.CollectionConfig{coll1}}
	ccpBytes, err := proto.Marshal(ccp)
	assert.NoError(t, err)
	assert.NotNil(t, ccpBytes)

	stub.MockTransactionStart("foo")
	err = scc.putChaincodeCollectionData(stub, cd, []byte("barf"))
	assert.Error(t, err)
	stub.MockTransactionEnd("foo")

	stub.MockTransactionStart("foo")
	err = scc.putChaincodeCollectionData(stub, cd, ccpBytes)
	assert.NoError(t, err)
	stub.MockTransactionEnd("foo")
}

func TestGetChaincodeCollectionData(t *testing.T) {
	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	scc := &SCC{
		BuiltinSCCs:      map[string]struct{}{"lscc": {}},
		Support:          &SupportImpl{GetMSPIDs: getMSPIDs},
		ACLProvider:      mockAclProvider,
		GetMSPIDs:        getMSPIDs,
		BCCSP:            cryptoProvider,
		BuildRegistry:    &container.BuildRegistry{},
		ChaincodeBuilder: &mock.ChaincodeBuilder{},
	}
	stub := shimtest.NewMockStub("lscc", scc)
	stub.ChannelID = "test"
	scc.Support = &MockSupport{}

	cd := &ccprovider.ChaincodeData{Name: "foo"}

	collName1 := "mycollection1"
	coll1 := createCollectionConfig(collName1, testPolicyEnvelope, 1, 2)
	ccp := &peer.CollectionConfigPackage{Config: []*peer.CollectionConfig{coll1}}
	ccpBytes, err := proto.Marshal(ccp)
	assert.NoError(t, err)
	assert.NotNil(t, ccpBytes)

	stub.MockTransactionStart("foo")
	err = scc.putChaincodeCollectionData(stub, cd, ccpBytes)
	assert.NoError(t, err)
	stub.MockTransactionEnd("foo")

	res := stub.MockInit("1", nil)
	assert.Equal(t, int32(shim.OK), res.Status, res.Message)

	for _, function := range []string{"GetCollectionsConfig", "getcollectionsconfig"} {
		sProp, _ := protoutil.MockSignedEndorserProposalOrPanic("test", &pb.ChaincodeSpec{}, []byte("Bob"), []byte("msg1"))
		sProp.Signature = sProp.ProposalBytes

		t.Run("invalid number of arguments", func(t *testing.T) {
			res = stub.MockInvokeWithSignedProposal("1", util.ToChaincodeArgs(function, "foo", "bar"), nil)
			assert.NotEqual(t, int32(shim.OK), res.Status)
			assert.Equal(t, "invalid number of arguments to lscc: 3", res.Message)
		})
		t.Run("invalid identity", func(t *testing.T) {
			mockAclProvider.Reset()
			mockAclProvider.On("CheckACL", resources.Lscc_GetCollectionsConfig, "test", sProp).Return(errors.New("acl check failed"))
			res = stub.MockInvokeWithSignedProposal("1", util.ToChaincodeArgs(function, "foo"), sProp)
			assert.NotEqual(t, int32(shim.OK), res.Status)
			assert.Contains(t, res.Message, "access denied for ["+function+"]")
		})
		t.Run("non-exists collections config", func(t *testing.T) {
			mockAclProvider.Reset()
			mockAclProvider.On("CheckACL", resources.Lscc_GetCollectionsConfig, "test", sProp).Return(nil)
			res = stub.MockInvokeWithSignedProposal("1", util.ToChaincodeArgs(function, "bar"), sProp)
			assert.NotEqual(t, int32(shim.OK), res.Status)
			assert.Equal(t, res.Message, "collections config not defined for chaincode bar")
		})
		t.Run("Success", func(t *testing.T) {
			res = stub.MockInvokeWithSignedProposal("1", util.ToChaincodeArgs(function, "foo"), sProp)
			assert.Equal(t, int32(shim.OK), res.Status)
			assert.NotNil(t, res.Payload)
		})
	}
}

func TestCheckCollectionMemberPolicy(t *testing.T) {
	// error case: no msp manager set, no collection config set
	err := checkCollectionMemberPolicy(nil, nil)
	assert.Error(t, err)

	mockmsp := new(mspmocks.MockMSP)
	mockmsp.On("DeserializeIdentity", []byte("signer0")).Return(&mspmocks.MockIdentity{}, nil)
	mockmsp.On("DeserializeIdentity", []byte("signer1")).Return(&mspmocks.MockIdentity{}, nil)
	mockmsp.On("GetIdentifier").Return("Org1", nil)
	mockmsp.On("GetType").Return(msp.FABRIC)
	mspmgmt.GetManagerForChain("foochannel")
	mgr := mspmgmt.GetManagerForChain("foochannel")

	// error case: msp manager not set up, no collection config set
	err = checkCollectionMemberPolicy(nil, nil)
	assert.EqualError(t, err, "msp manager not set")

	// set up msp manager
	mgr.Setup([]msp.MSP{mockmsp})

	// error case: no collection config set
	err = checkCollectionMemberPolicy(nil, mgr)
	assert.EqualError(t, err, "collection configuration is not set")

	// error case: empty collection config
	cc := &peer.CollectionConfig{}
	err = checkCollectionMemberPolicy(cc, mgr)
	assert.EqualError(t, err, "collection configuration is empty")

	// error case: no static collection config
	cc = &peer.CollectionConfig{Payload: &peer.CollectionConfig_StaticCollectionConfig{}}
	err = checkCollectionMemberPolicy(cc, mgr)
	assert.EqualError(t, err, "collection configuration is empty")

	// error case: member org policy not set
	cc = &peer.CollectionConfig{
		Payload: &peer.CollectionConfig_StaticCollectionConfig{
			StaticCollectionConfig: &peer.StaticCollectionConfig{},
		},
	}
	err = checkCollectionMemberPolicy(cc, mgr)
	assert.EqualError(t, err, "collection member policy is not set")

	// error case: member org policy config empty
	cc = &peer.CollectionConfig{
		Payload: &peer.CollectionConfig_StaticCollectionConfig{
			StaticCollectionConfig: &peer.StaticCollectionConfig{
				Name: "mycollection",
				MemberOrgsPolicy: &peer.CollectionPolicyConfig{
					Payload: &peer.CollectionPolicyConfig_SignaturePolicy{},
				},
			},
		},
	}
	err = checkCollectionMemberPolicy(cc, mgr)
	assert.EqualError(t, err, "collection member org policy is empty")

	// error case: signd-by index is out of range of signers
	cc = &peer.CollectionConfig{
		Payload: &peer.CollectionConfig_StaticCollectionConfig{
			StaticCollectionConfig: &peer.StaticCollectionConfig{
				Name:             "mycollection",
				MemberOrgsPolicy: getBadAccessPolicy([]string{"signer0"}, 1),
			},
		},
	}
	err = checkCollectionMemberPolicy(cc, mgr)
	assert.EqualError(t, err, "invalid member org policy for collection 'mycollection': identity index out of range, requested 1, but identities length is 1")

	// valid case: well-formed collection policy config
	cc = &peer.CollectionConfig{
		Payload: &peer.CollectionConfig_StaticCollectionConfig{
			StaticCollectionConfig: &peer.StaticCollectionConfig{
				Name: "mycollection",
				MemberOrgsPolicy: &peer.CollectionPolicyConfig{
					Payload: &peer.CollectionPolicyConfig_SignaturePolicy{
						SignaturePolicy: testPolicyEnvelope,
					},
				},
			},
		},
	}
	err = checkCollectionMemberPolicy(cc, mgr)
	assert.NoError(t, err)

	// check MSPPrincipal_IDENTITY type
	var signers = [][]byte{[]byte("signer0"), []byte("signer1")}
	signaturePolicyEnvelope := cauthdsl.Envelope(cauthdsl.Or(cauthdsl.SignedBy(0), cauthdsl.SignedBy(1)), signers)
	signaturePolicy := &peer.CollectionPolicyConfig_SignaturePolicy{
		SignaturePolicy: signaturePolicyEnvelope,
	}
	accessPolicy := &peer.CollectionPolicyConfig{
		Payload: signaturePolicy,
	}
	cc = &peer.CollectionConfig{
		Payload: &peer.CollectionConfig_StaticCollectionConfig{
			StaticCollectionConfig: &peer.StaticCollectionConfig{
				Name:             "mycollection",
				MemberOrgsPolicy: accessPolicy,
			},
		},
	}
	err = checkCollectionMemberPolicy(cc, mgr)
	assert.NoError(t, err)
	mockmsp.AssertNumberOfCalls(t, "DeserializeIdentity", 3)

	// check MSPPrincipal_ROLE type
	signaturePolicyEnvelope = cauthdsl.SignedByAnyMember([]string{"Org1"})
	signaturePolicy.SignaturePolicy = signaturePolicyEnvelope
	accessPolicy.Payload = signaturePolicy
	cc = &peer.CollectionConfig{
		Payload: &peer.CollectionConfig_StaticCollectionConfig{
			StaticCollectionConfig: &peer.StaticCollectionConfig{
				Name:             "mycollection",
				MemberOrgsPolicy: accessPolicy,
			},
		},
	}
	err = checkCollectionMemberPolicy(cc, mgr)
	assert.NoError(t, err)

	// check MSPPrincipal_ROLE type for unknown org
	signaturePolicyEnvelope = cauthdsl.SignedByAnyMember([]string{"Org2"})
	signaturePolicy.SignaturePolicy = signaturePolicyEnvelope
	accessPolicy.Payload = signaturePolicy
	cc = &peer.CollectionConfig{
		Payload: &peer.CollectionConfig_StaticCollectionConfig{
			StaticCollectionConfig: &peer.StaticCollectionConfig{
				Name:             "mycollection",
				MemberOrgsPolicy: accessPolicy,
			},
		},
	}
	err = checkCollectionMemberPolicy(cc, mgr)
	// this does not raise an error but prints a warning logging message instead
	assert.NoError(t, err)

	// check MSPPrincipal_ORGANIZATION_UNIT type
	principal := &mb.MSPPrincipal{
		PrincipalClassification: mb.MSPPrincipal_ORGANIZATION_UNIT,
		Principal:               protoutil.MarshalOrPanic(&mb.OrganizationUnit{MspIdentifier: "Org1"}),
	}
	// create the policy: it requires exactly 1 signature from the first (and only) principal
	signaturePolicy.SignaturePolicy = &common.SignaturePolicyEnvelope{
		Version:    0,
		Rule:       cauthdsl.NOutOf(1, []*common.SignaturePolicy{cauthdsl.SignedBy(0)}),
		Identities: []*mb.MSPPrincipal{principal},
	}
	accessPolicy.Payload = signaturePolicy
	cc = &peer.CollectionConfig{
		Payload: &peer.CollectionConfig_StaticCollectionConfig{
			StaticCollectionConfig: &peer.StaticCollectionConfig{
				Name:             "mycollection",
				MemberOrgsPolicy: accessPolicy,
			},
		},
	}
	err = checkCollectionMemberPolicy(cc, mgr)
	assert.NoError(t, err)
}

func TestCheckChaincodeName(t *testing.T) {
	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)

	lscc := &SCC{BCCSP: cryptoProvider}

	/*allowed naming*/
	err = lscc.isValidChaincodeName("a-b")
	assert.NoError(t, err)
	err = lscc.isValidChaincodeName("a_b")
	assert.NoError(t, err)
	err = lscc.isValidChaincodeName("a_b-c")
	assert.NoError(t, err)
	err = lscc.isValidChaincodeName("a-b_c")
	assert.NoError(t, err)

	/*invalid naming*/
	err = lscc.isValidChaincodeName("")
	assert.EqualError(t, err, "invalid chaincode name ''. Names must start with an alphanumeric character and can only consist of alphanumerics, '_', and '-'")
	err = lscc.isValidChaincodeName("-ab")
	assert.EqualError(t, err, "invalid chaincode name '-ab'. Names must start with an alphanumeric character and can only consist of alphanumerics, '_', and '-'")
	err = lscc.isValidChaincodeName("_ab")
	assert.EqualError(t, err, "invalid chaincode name '_ab'. Names must start with an alphanumeric character and can only consist of alphanumerics, '_', and '-'")
	err = lscc.isValidChaincodeName("ab-")
	assert.EqualError(t, err, "invalid chaincode name 'ab-'. Names must start with an alphanumeric character and can only consist of alphanumerics, '_', and '-'")
	err = lscc.isValidChaincodeName("ab_")
	assert.EqualError(t, err, "invalid chaincode name 'ab_'. Names must start with an alphanumeric character and can only consist of alphanumerics, '_', and '-'")
	err = lscc.isValidChaincodeName("a__b")
	assert.EqualError(t, err, "invalid chaincode name 'a__b'. Names must start with an alphanumeric character and can only consist of alphanumerics, '_', and '-'")
	err = lscc.isValidChaincodeName("a--b")
	assert.EqualError(t, err, "invalid chaincode name 'a--b'. Names must start with an alphanumeric character and can only consist of alphanumerics, '_', and '-'")
	err = lscc.isValidChaincodeName("a-_b")
	assert.EqualError(t, err, "invalid chaincode name 'a-_b'. Names must start with an alphanumeric character and can only consist of alphanumerics, '_', and '-'")
}

func TestCheckChaincodeVersion(t *testing.T) {
	lscc := &SCC{}

	validCCName := "ccname"
	/*allowed versions*/
	err := lscc.isValidChaincodeVersion(validCCName, "a_b")
	assert.NoError(t, err)
	err = lscc.isValidChaincodeVersion(validCCName, "a.b")
	assert.NoError(t, err)
	err = lscc.isValidChaincodeVersion(validCCName, "a+b")
	assert.NoError(t, err)
	err = lscc.isValidChaincodeVersion(validCCName, "a-b")
	assert.NoError(t, err)
	err = lscc.isValidChaincodeVersion(validCCName, "-ab")
	assert.NoError(t, err)
	err = lscc.isValidChaincodeVersion(validCCName, "a.0")
	assert.NoError(t, err)
	err = lscc.isValidChaincodeVersion(validCCName, "a_b.c+d-e")
	assert.NoError(t, err)
	err = lscc.isValidChaincodeVersion(validCCName, "0")
	assert.NoError(t, err)

	/*invalid versions*/
	err = lscc.isValidChaincodeVersion(validCCName, "")
	assert.EqualError(t, err, fmt.Sprintf("invalid chaincode version ''. Versions must not be empty and can only consist of alphanumerics, '_',  '-', '+', and '.'"))
	err = lscc.isValidChaincodeVersion(validCCName, "$badversion")
	assert.EqualError(t, err, "invalid chaincode version '$badversion'. Versions must not be empty and can only consist of alphanumerics, '_',  '-', '+', and '.'")
}

func TestLifecycleChaincodeRegularExpressionsMatch(t *testing.T) {
	assert.Equal(t, ChaincodeNameRegExp.String(), lifecycle.ChaincodeNameRegExp.String())
	assert.Equal(t, ChaincodeVersionRegExp.String(), lifecycle.ChaincodeVersionRegExp.String())
}

var id msp.SigningIdentity
var channelID = "testchannelid"
var mockAclProvider *mocks.MockACLProvider

func NewMockProvider() *mscc.MocksccProviderImpl {
	return (&mscc.MocksccProviderFactory{
		ApplicationConfigBool: true,
		ApplicationConfigRv: &config.MockApplication{
			CapabilitiesRv: &config.MockApplicationCapabilities{},
		},
	}).NewSystemChaincodeProvider().(*mscc.MocksccProviderImpl)
}

func TestMain(m *testing.M) {
	//setup extension test environment
	_, _, destroy := xtestutil.SetupExtTestEnv()
	viper.Set("peer.fileSystemPath", "/tmp/fabric/core/ledger")

	var err error
	msptesttools.LoadMSPSetupForTesting()

	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	if err != nil {
		fmt.Printf("Initialize cryptoProvider bccsp failed: %s", err)
		os.Exit(-1)
	}

	id, err = mspmgmt.GetLocalMSP(cryptoProvider).GetDefaultSigningIdentity()
	if err != nil {
		fmt.Printf("GetSigningIdentity failed with err %s", err)
		os.Exit(-1)
	}

	mockAclProvider = &mocks.MockACLProvider{}
	mockAclProvider.Reset()

	code := m.Run()
	destroy()
	os.Exit(code)
}

type MockSupport struct {
	PutChaincodeToLocalStorageErr    error
	GetChaincodeFromLocalStorageRv   ccprovider.CCPackage
	GetChaincodeFromLocalStorageErr  error
	GetChaincodesFromLocalStorageRv  *peer.ChaincodeQueryResponse
	GetChaincodesFromLocalStorageErr error
	GetInstantiationPolicyRv         []byte
	GetInstantiationPolicyErr        error
	CheckInstantiationPolicyErr      error
	GetInstantiationPolicyMap        map[string][]byte
	CheckInstantiationPolicyMap      map[string]error
	CheckCollectionConfigErr         error
}

func (s *MockSupport) PutChaincodeToLocalStorage(ccpack ccprovider.CCPackage) error {
	return s.PutChaincodeToLocalStorageErr
}

func (s *MockSupport) GetChaincodeFromLocalStorage(ccNameVersion string) (ccprovider.CCPackage, error) {
	return s.GetChaincodeFromLocalStorageRv, s.GetChaincodeFromLocalStorageErr
}

func (s *MockSupport) GetChaincodesFromLocalStorage() (*peer.ChaincodeQueryResponse, error) {
	return s.GetChaincodesFromLocalStorageRv, s.GetChaincodesFromLocalStorageErr
}

func (s *MockSupport) GetInstantiationPolicy(channel string, ccpack ccprovider.CCPackage) ([]byte, error) {
	if s.GetInstantiationPolicyMap != nil {
		str := ccpack.GetChaincodeData().Name + ccpack.GetChaincodeData().Version
		s.GetInstantiationPolicyMap[str] = []byte(str)
		return []byte(ccpack.GetChaincodeData().Name + ccpack.GetChaincodeData().Version), nil
	}
	return s.GetInstantiationPolicyRv, s.GetInstantiationPolicyErr
}

func (s *MockSupport) CheckInstantiationPolicy(signedProp *peer.SignedProposal, chainName string, instantiationPolicy []byte) error {
	if s.CheckInstantiationPolicyMap != nil {
		return s.CheckInstantiationPolicyMap[string(instantiationPolicy)]
	}
	return s.CheckInstantiationPolicyErr
}

func (s *MockSupport) CheckCollectionConfig(collectionConfig *peer.CollectionConfig, channelName string) error {
	return s.CheckCollectionConfigErr
}

// getBadAccessPolicy creates a bad CollectionPolicyConfig with signedby index out of range of signers
func getBadAccessPolicy(signers []string, badIndex int32) *peer.CollectionPolicyConfig {
	var data [][]byte
	for _, signer := range signers {
		data = append(data, []byte(signer))
	}
	// use a out of range index to trigger error
	policyEnvelope := cauthdsl.Envelope(cauthdsl.Or(cauthdsl.SignedBy(0), cauthdsl.SignedBy(badIndex)), data)
	return &peer.CollectionPolicyConfig{
		Payload: &peer.CollectionPolicyConfig_SignaturePolicy{
			SignaturePolicy: policyEnvelope,
		},
	}
}
