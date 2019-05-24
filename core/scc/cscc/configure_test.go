/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package cscc

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/config"
	"github.com/hyperledger/fabric/common/configtx"
	configtxtest "github.com/hyperledger/fabric/common/configtx/test"
	"github.com/hyperledger/fabric/common/genesis"
	"github.com/hyperledger/fabric/common/metrics/disabled"
	"github.com/hyperledger/fabric/common/mocks/scc"
	"github.com/hyperledger/fabric/common/policies"
	"github.com/hyperledger/fabric/core/aclmgmt"
	aclmocks "github.com/hyperledger/fabric/core/aclmgmt/mocks"
	"github.com/hyperledger/fabric/core/aclmgmt/resources"
	"github.com/hyperledger/fabric/core/chaincode"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/core/common/ccprovider"
	"github.com/hyperledger/fabric/core/deliverservice"
	"github.com/hyperledger/fabric/core/deliverservice/blocksprovider"
	"github.com/hyperledger/fabric/core/peer"
	"github.com/hyperledger/fabric/core/policy"
	policymocks "github.com/hyperledger/fabric/core/policy/mocks"
	"github.com/hyperledger/fabric/core/scc/cscc/mock"
	xtestutil "github.com/hyperledger/fabric/extensions/testutil"
	"github.com/hyperledger/fabric/gossip/api"
	"github.com/hyperledger/fabric/gossip/service"
	"github.com/hyperledger/fabric/internal/configtxgen/configtxgentest"
	"github.com/hyperledger/fabric/internal/configtxgen/encoder"
	genesisconfig "github.com/hyperledger/fabric/internal/configtxgen/localconfig"
	peergossip "github.com/hyperledger/fabric/internal/peer/gossip"
	"github.com/hyperledger/fabric/internal/peer/gossip/mocks"
	"github.com/hyperledger/fabric/msp/mgmt"
	msptesttools "github.com/hyperledger/fabric/msp/mgmt/testtools"
	cb "github.com/hyperledger/fabric/protos/common"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

//go:generate counterfeiter -o mock/config_manager.go --fake-name ConfigManager . configManager

type configManager interface {
	config.Manager
}

//go:generate counterfeiter -o mock/acl_provider.go --fake-name ACLProvider . aclProvider

type aclProvider interface {
	aclmgmt.ACLProvider
}

//go:generate counterfeiter -o mock/configtx_validator.go --fake-name ConfigtxValidator . configtxValidator

type configtxValidator interface {
	configtx.Validator
}

type mockDeliveryClient struct {
}

func (ds *mockDeliveryClient) UpdateEndpoints(chainID string, endpoints []string) error {
	return nil
}

// StartDeliverForChannel dynamically starts delivery of new blocks from ordering service
// to channel peers.
func (ds *mockDeliveryClient) StartDeliverForChannel(chainID string, ledgerInfo blocksprovider.LedgerInfo, f func()) error {
	return nil
}

// StopDeliverForChannel dynamically stops delivery of new blocks from ordering service
// to channel peers.
func (ds *mockDeliveryClient) StopDeliverForChannel(chainID string) error {
	return nil
}

// Stop terminates delivery service and closes the connection
func (*mockDeliveryClient) Stop() {

}

type mockDeliveryClientFactory struct {
}

func (*mockDeliveryClientFactory) Service(g service.GossipService, endpoints []string, mcs api.MessageCryptoService) (deliverservice.DeliverService, error) {
	return &mockDeliveryClient{}, nil
}

var mockAclProvider *aclmocks.MockACLProvider

func TestMain(m *testing.M) {
	//setup extension test environment
	_, _, destroy := xtestutil.SetupExtTestEnv()

	msptesttools.LoadMSPSetupForTesting()

	mockAclProvider = &aclmocks.MockACLProvider{}
	mockAclProvider.Reset()

	code := m.Run()
	destroy()
	os.Exit(code)
}

func TestConfigerInit(t *testing.T) {
	e := New(nil, mockAclProvider, nil, nil, nil)
	stub := shim.NewMockStub("PeerConfiger", e)

	if res := stub.MockInit("1", nil); res.Status != shim.OK {
		fmt.Println("Init failed", string(res.Message))
		t.FailNow()
	}
}

func TestConfigerInvokeInvalidParameters(t *testing.T) {
	e := New(nil, mockAclProvider, nil, nil, nil)
	stub := shim.NewMockStub("PeerConfiger", e)

	res := stub.MockInit("1", nil)
	assert.Equal(t, res.Status, int32(shim.OK), "Init failed")

	res = stub.MockInvoke("2", nil)
	assert.Equal(t, res.Status, int32(shim.ERROR), "CSCC invoke expected to fail having zero arguments")
	assert.Equal(t, res.Message, "Incorrect number of arguments, 0")

	mockAclProvider.Reset()
	mockAclProvider.On("CheckACL", resources.Cscc_GetChannels, "", (*pb.SignedProposal)(nil)).Return(errors.New("Failed authorization"))
	args := [][]byte{[]byte("GetChannels")}
	res = stub.MockInvokeWithSignedProposal("3", args, nil)
	assert.Equal(t, res.Status, int32(shim.ERROR), "CSCC invoke expected to fail no signed proposal provided")
	assert.Contains(t, res.Message, "access denied for [GetChannels]")

	args = [][]byte{[]byte("fooFunction"), []byte("testChainID")}
	res = stub.MockInvoke("5", args)
	assert.Equal(t, res.Status, int32(shim.ERROR), "CSCC invoke expected wrong function name provided")
	assert.Equal(t, res.Message, "Requested function fooFunction not found.")

	mockAclProvider.Reset()
	mockAclProvider.On("CheckACL", resources.Cscc_GetConfigBlock, "testChainID", (*pb.SignedProposal)(nil)).Return(errors.New("Nil SignedProposal"))
	args = [][]byte{[]byte("GetConfigBlock"), []byte("testChainID")}
	res = stub.MockInvokeWithSignedProposal("4", args, nil)
	assert.Equal(t, res.Status, int32(shim.ERROR), "CSCC invoke expected to fail no signed proposal provided")
	assert.Contains(t, res.Message, "Nil SignedProposal")
	mockAclProvider.AssertExpectations(t)
}

func TestConfigerInvokeJoinChainMissingParams(t *testing.T) {
	viper.Set("peer.fileSystemPath", "/tmp/hyperledgertest/")
	os.Mkdir("/tmp/hyperledgertest", 0755)
	defer os.RemoveAll("/tmp/hyperledgertest/")

	e := New(nil, mockAclProvider, nil, nil, nil)
	stub := shim.NewMockStub("PeerConfiger", e)

	if res := stub.MockInit("1", nil); res.Status != shim.OK {
		fmt.Println("Init failed", string(res.Message))
		t.FailNow()
	}

	// Failed path: expected to have at least one argument
	mockAclProvider.Reset()
	mockAclProvider.On("CheckACL", resources.Cscc_JoinChain, "", (*pb.SignedProposal)(nil)).Return(nil)
	args := [][]byte{[]byte("JoinChain")}
	if res := stub.MockInvoke("2", args); res.Status == shim.OK {
		t.Fatalf("cscc invoke JoinChain should have failed with invalid number of args: %v", args)
	}
}

func TestConfigerInvokeJoinChainWrongParams(t *testing.T) {

	viper.Set("peer.fileSystemPath", "/tmp/hyperledgertest/")
	os.Mkdir("/tmp/hyperledgertest", 0755)
	defer os.RemoveAll("/tmp/hyperledgertest/")

	e := New(nil, mockAclProvider, nil, nil, nil)
	stub := shim.NewMockStub("PeerConfiger", e)

	if res := stub.MockInit("1", nil); res.Status != shim.OK {
		fmt.Println("Init failed", string(res.Message))
		t.FailNow()
	}

	// Failed path: wrong parameter type
	mockAclProvider.Reset()
	mockAclProvider.On("CheckACL", resources.Cscc_JoinChain, "", (*pb.SignedProposal)(nil)).Return(nil)
	args := [][]byte{[]byte("JoinChain"), []byte("action")}
	if res := stub.MockInvoke("2", args); res.Status == shim.OK {
		t.Fatalf("cscc invoke JoinChain should have failed with null genesis block.  args: %v", args)
	}
}

type PackageProviderWrapper struct {
	FS *ccprovider.CCInfoFSImpl
}

func (p *PackageProviderWrapper) GetChaincodeCodePackage(ccci *ccprovider.ChaincodeContainerInfo) ([]byte, error) {
	return p.FS.GetChaincodeCodePackage(ccci.Name, ccci.Version)
}

func TestConfigerInvokeJoinChainCorrectParams(t *testing.T) {
	mp := (&scc.MocksccProviderFactory{}).NewSystemChaincodeProvider()

	viper.Set("chaincode.executetimeout", "3s")

	cleanup, err := peer.MockInitialize()
	if err != nil {
		t.Fatalf("Failed to initialize peer: %s", err)
	}
	defer cleanup()

	e := New(mp, mockAclProvider, nil, nil, nil)
	stub := shim.NewMockStub("PeerConfiger", e)

	peerEndpoint := "127.0.0.1:13611"

	config := chaincode.GlobalConfig()
	config.StartupTimeout = 30 * time.Second

	// Init the policy checker
	policyManagerGetter := &policymocks.MockChannelPolicyManagerGetter{
		Managers: map[string]policies.Manager{
			"mytestchainid": &policymocks.MockChannelPolicyManager{
				MockPolicy: &policymocks.MockPolicy{
					Deserializer: &policymocks.MockIdentityDeserializer{
						Identity: []byte("Alice"),
						Msg:      []byte("msg1"),
					},
				},
			},
		},
	}

	identityDeserializer := &policymocks.MockIdentityDeserializer{
		Identity: []byte("Alice"),
		Msg:      []byte("msg1"),
	}

	e.policyChecker = policy.NewPolicyChecker(
		policyManagerGetter,
		identityDeserializer,
		&policymocks.MockMSPPrincipalGetter{Principal: []byte("Alice")},
	)

	grpcServer := grpc.NewServer()
	socket, err := net.Listen("tcp", peerEndpoint)
	require.NoError(t, err)

	signer := mgmt.GetLocalSigningIdentityOrPanic()
	messageCryptoService := peergossip.NewMCS(&mocks.ChannelPolicyManagerGetter{}, signer, mgmt.NewDeserializersManager())
	secAdv := peergossip.NewSecurityAdvisor(mgmt.NewDeserializersManager())
	var defaultSecureDialOpts = func() []grpc.DialOption {
		var dialOpts []grpc.DialOption
		dialOpts = append(dialOpts, grpc.WithInsecure())
		return dialOpts
	}
	err = service.InitGossipServiceCustomDeliveryFactory(signer, &disabled.Provider{}, peerEndpoint, grpcServer, nil,
		&mockDeliveryClientFactory{}, messageCryptoService, secAdv, defaultSecureDialOpts)
	assert.NoError(t, err)

	go grpcServer.Serve(socket)
	defer grpcServer.Stop()

	// Successful path for JoinChain
	blockBytes := mockConfigBlock()
	if blockBytes == nil {
		t.Fatalf("cscc invoke JoinChain failed because invalid block")
	}
	args := [][]byte{[]byte("JoinChain"), blockBytes}
	sProp, _ := protoutil.MockSignedEndorserProposalOrPanic("", &pb.ChaincodeSpec{}, []byte("Alice"), []byte("msg1"))
	identityDeserializer.Msg = sProp.ProposalBytes
	sProp.Signature = sProp.ProposalBytes

	// Try fail path with nil block
	mockAclProvider.Reset()
	mockAclProvider.On("CheckACL", resources.Cscc_JoinChain, "", sProp).Return(nil)
	res := stub.MockInvokeWithSignedProposal("2", [][]byte{[]byte("JoinChain"), nil}, sProp)
	assert.Equal(t, res.Status, int32(shim.ERROR))

	// Try fail path with block and nil payload header
	payload, _ := proto.Marshal(&cb.Payload{})
	env, _ := proto.Marshal(&cb.Envelope{
		Payload: payload,
	})
	badBlock := &cb.Block{
		Data: &cb.BlockData{
			Data: [][]byte{env},
		},
	}
	mockAclProvider.Reset()
	mockAclProvider.On("CheckACL", resources.Cscc_JoinChain, "", sProp).Return(nil)
	badBlockBytes := protoutil.MarshalOrPanic(badBlock)
	res = stub.MockInvokeWithSignedProposal("2", [][]byte{[]byte("JoinChain"), badBlockBytes}, sProp)
	assert.Equal(t, res.Status, int32(shim.ERROR))

	// Now, continue with valid execution path
	if res := stub.MockInvokeWithSignedProposal("2", args, sProp); res.Status != shim.OK {
		t.Fatalf("cscc invoke JoinChain failed with: %v", res.Message)
	}

	// This call must fail
	sProp.Signature = nil
	mockAclProvider.Reset()
	mockAclProvider.On("CheckACL", resources.Cscc_JoinChain, "", sProp).Return(errors.New("Failed authorization"))
	res = stub.MockInvokeWithSignedProposal("3", args, sProp)
	if res.Status == shim.OK {
		t.Fatalf("cscc invoke JoinChain must fail : %v", res.Message)
	}
	assert.Contains(t, res.Message, "access denied for [JoinChain][mytestchainid]")
	sProp.Signature = sProp.ProposalBytes

	// Query the configuration block
	//chainID := []byte{143, 222, 22, 192, 73, 145, 76, 110, 167, 154, 118, 66, 132, 204, 113, 168}
	chainID, err := protoutil.GetChainIDFromBlockBytes(blockBytes)
	if err != nil {
		t.Fatalf("cscc invoke JoinChain failed with: %v", err)
	}

	// Test an ACL failure on GetConfigBlock
	mockAclProvider.Reset()
	mockAclProvider.On("CheckACL", resources.Cscc_GetConfigBlock, "mytestchainid", sProp).Return(errors.New("Failed authorization"))
	args = [][]byte{[]byte("GetConfigBlock"), []byte(chainID)}
	res = stub.MockInvokeWithSignedProposal("2", args, sProp)
	if res.Status == shim.OK {
		t.Fatalf("cscc invoke GetConfigBlock should have failed: %v", res.Message)
	}
	assert.Contains(t, res.Message, "Failed authorization")
	mockAclProvider.AssertExpectations(t)

	// Test with ACL okay
	mockAclProvider.Reset()
	mockAclProvider.On("CheckACL", resources.Cscc_GetConfigBlock, "mytestchainid", sProp).Return(nil)
	if res := stub.MockInvokeWithSignedProposal("2", args, sProp); res.Status != shim.OK {
		t.Fatalf("cscc invoke GetConfigBlock failed with: %v", res.Message)
	}

	// get channels for the peer
	mockAclProvider.Reset()
	mockAclProvider.On("CheckACL", resources.Cscc_GetChannels, "", sProp).Return(nil)
	args = [][]byte{[]byte(GetChannels)}
	res = stub.MockInvokeWithSignedProposal("2", args, sProp)
	if res.Status != shim.OK {
		t.FailNow()
	}

	cqr := &pb.ChannelQueryResponse{}
	err = proto.Unmarshal(res.Payload, cqr)
	if err != nil {
		t.FailNow()
	}

	// peer joined one channel so query should return an array with one channel
	if len(cqr.GetChannels()) != 1 {
		t.FailNow()
	}
}

func TestGetConfigTree(t *testing.T) {
	aclProvider := &mock.ACLProvider{}
	configMgr := &mock.ConfigManager{}
	pc := &PeerConfiger{
		aclProvider: aclProvider,
		configMgr:   configMgr,
	}

	args := [][]byte{[]byte("GetConfigTree"), []byte("testchan")}

	t.Run("Success", func(t *testing.T) {
		ctxv := &mock.ConfigtxValidator{}
		configMgr.GetChannelConfigReturns(ctxv)
		testConfig := &cb.Config{
			ChannelGroup: &cb.ConfigGroup{
				Values: map[string]*cb.ConfigValue{
					"foo": {
						Value: []byte("bar"),
					},
				},
			},
		}
		ctxv.ConfigProtoReturns(testConfig)
		res := pc.InvokeNoShim(args, nil)
		assert.Equal(t, int32(shim.OK), res.Status)
		checkConfig := &pb.ConfigTree{}
		err := proto.Unmarshal(res.Payload, checkConfig)
		assert.NoError(t, err)
		assert.True(t, proto.Equal(testConfig, checkConfig.ChannelConfig))
	})

	t.Run("MissingConfig", func(t *testing.T) {
		ctxv := &mock.ConfigtxValidator{}
		configMgr.GetChannelConfigReturns(ctxv)
		res := pc.InvokeNoShim(args, nil)
		assert.NotEqual(t, int32(shim.OK), res.Status)
		assert.Equal(t, "Unknown chain ID, testchan", res.Message)
	})

	t.Run("NilChannel", func(t *testing.T) {
		ctxv := &mock.ConfigtxValidator{}
		configMgr.GetChannelConfigReturns(ctxv)
		res := pc.InvokeNoShim([][]byte{[]byte("GetConfigTree"), nil}, nil)
		assert.NotEqual(t, int32(shim.OK), res.Status)
		assert.Equal(t, "Chain ID must not be nil", res.Message)
	})

	t.Run("BadACL", func(t *testing.T) {
		aclProvider.CheckACLReturns(fmt.Errorf("fake-error"))
		res := pc.InvokeNoShim(args, nil)
		assert.NotEqual(t, int32(shim.OK), res.Status)
		assert.Equal(t, "access denied for [GetConfigTree][testchan]: fake-error", res.Message)
	})
}

func TestSimulateConfigTreeUpdate(t *testing.T) {
	aclProvider := &mock.ACLProvider{}
	configMgr := &mock.ConfigManager{}
	pc := &PeerConfiger{
		aclProvider: aclProvider,
		configMgr:   configMgr,
	}

	testUpdate := &cb.Envelope{
		Payload: protoutil.MarshalOrPanic(&cb.Payload{
			Header: &cb.Header{
				ChannelHeader: protoutil.MarshalOrPanic(&cb.ChannelHeader{
					Type: int32(cb.HeaderType_CONFIG_UPDATE),
				}),
			},
		}),
	}

	args := [][]byte{[]byte("SimulateConfigTreeUpdate"), []byte("testchan"), protoutil.MarshalOrPanic(testUpdate)}

	t.Run("Success", func(t *testing.T) {
		ctxv := &mock.ConfigtxValidator{}
		configMgr.GetChannelConfigReturns(ctxv)
		res := pc.InvokeNoShim(args, nil)
		assert.Equal(t, int32(shim.OK), res.Status, res.Message)
	})

	t.Run("BadUpdate", func(t *testing.T) {
		ctxv := &mock.ConfigtxValidator{}
		configMgr.GetChannelConfigReturns(ctxv)
		ctxv.ProposeConfigUpdateReturns(nil, fmt.Errorf("fake-error"))
		res := pc.InvokeNoShim(args, nil)
		assert.NotEqual(t, int32(shim.OK), res.Status)
		assert.Equal(t, "fake-error", res.Message)
	})

	t.Run("BadType", func(t *testing.T) {
		res := pc.InvokeNoShim([][]byte{
			args[0],
			args[1],
			protoutil.MarshalOrPanic(&cb.Envelope{
				Payload: protoutil.MarshalOrPanic(&cb.Payload{
					Header: &cb.Header{
						ChannelHeader: protoutil.MarshalOrPanic(&cb.ChannelHeader{
							Type: int32(cb.HeaderType_ENDORSER_TRANSACTION),
						}),
					},
				}),
			}),
		}, nil)
		assert.NotEqual(t, int32(shim.OK), res.Status)
		assert.Equal(t, "invalid payload header type: 3", res.Message)
	})

	t.Run("BadEnvelope", func(t *testing.T) {
		res := pc.InvokeNoShim([][]byte{
			args[0],
			args[1],
			[]byte("garbage"),
		}, nil)
		assert.NotEqual(t, int32(shim.OK), res.Status)
		assert.Contains(t, res.Message, "proto:")
	})

	t.Run("NilChainID", func(t *testing.T) {
		res := pc.InvokeNoShim([][]byte{
			args[0],
			nil,
			args[2],
		}, nil)
		assert.NotEqual(t, int32(shim.OK), res.Status)
		assert.Equal(t, "Chain ID must not be nil", res.Message)
	})

	t.Run("BadACL", func(t *testing.T) {
		aclProvider.CheckACLReturns(fmt.Errorf("fake-error"))
		res := pc.InvokeNoShim(args, nil)
		assert.NotEqual(t, int32(shim.OK), res.Status)
		assert.Equal(t, "access denied for [SimulateConfigTreeUpdate][testchan]: fake-error", res.Message)
	})
}

func TestPeerConfiger_SubmittingOrdererGenesis(t *testing.T) {
	viper.Set("peer.fileSystemPath", "/tmp/hyperledgertest/")
	os.Mkdir("/tmp/hyperledgertest", 0755)
	defer os.RemoveAll("/tmp/hyperledgertest/")

	e := New(nil, nil, nil, nil, nil)
	stub := shim.NewMockStub("PeerConfiger", e)

	if res := stub.MockInit("1", nil); res.Status != shim.OK {
		fmt.Println("Init failed", string(res.Message))
		t.FailNow()
	}
	conf := configtxgentest.Load(genesisconfig.SampleSingleMSPSoloProfile)
	conf.Application = nil
	cg, err := encoder.NewChannelGroup(conf)
	assert.NoError(t, err)
	block := genesis.NewFactoryImpl(cg).Block("mytestchainid")
	blockBytes := protoutil.MarshalOrPanic(block)

	// Failed path: wrong parameter type
	mockAclProvider.Reset()
	mockAclProvider.On("CheckACL", resources.Cscc_JoinChain, "", (*pb.SignedProposal)(nil)).Return(nil)
	args := [][]byte{[]byte("JoinChain"), []byte(blockBytes)}
	if res := stub.MockInvoke("2", args); res.Status == shim.OK {
		t.Fatalf("cscc invoke JoinChain should have failed with wrong genesis block.  args: %v", args)
	} else {
		assert.Contains(t, res.Message, "missing Application configuration group")
	}
}

func mockConfigBlock() []byte {
	var blockBytes []byte = nil
	block, err := configtxtest.MakeGenesisBlock("mytestchainid")
	if err == nil {
		blockBytes = protoutil.MarshalOrPanic(block)
	}
	return blockBytes
}
