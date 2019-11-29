/*
Copyright Digital Asset Holdings, LLC. All Rights Reserved.
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chaincode

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	cb "github.com/hyperledger/fabric-protos-go/common"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/bccsp/factory"
	"github.com/hyperledger/fabric/bccsp/sw"
	"github.com/hyperledger/fabric/common/cauthdsl"
	"github.com/hyperledger/fabric/core/config/configtest"
	"github.com/hyperledger/fabric/internal/configtxgen/encoder"
	"github.com/hyperledger/fabric/internal/configtxgen/genesisconfig"
	"github.com/hyperledger/fabric/internal/peer/chaincode/mock"
	"github.com/hyperledger/fabric/internal/peer/common"
	"github.com/hyperledger/fabric/internal/pkg/identity"
	"github.com/hyperledger/fabric/protoutil"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:generate counterfeiter -o mock/signer_serializer.go --fake-name SignerSerializer . signerSerializer

type signerSerializer interface {
	identity.SignerSerializer
}

//go:generate counterfeiter -o mock/deliver.go --fake-name Deliver . deliver

type deliver interface {
	pb.Deliver_DeliverClient
}

//go:generate counterfeiter -o mock/deliver_client.go --fake-name PeerDeliverClient . peerDeliverClient

type peerDeliverClient interface {
	pb.DeliverClient
}

func TestCheckChaincodeCmdParamsWithNewCallingSchema(t *testing.T) {
	chaincodeCtorJSON = `{ "Args":["func", "param"] }`
	chaincodePath = "some/path"
	chaincodeName = "somename"
	require := require.New(t)
	result := checkChaincodeCmdParams(&cobra.Command{})

	require.Nil(result)
}

func TestCheckChaincodeCmdParamsWithOldCallingSchema(t *testing.T) {
	chaincodeCtorJSON = `{ "Function":"func", "Args":["param"] }`
	chaincodePath = "some/path"
	chaincodeName = "somename"
	require := require.New(t)
	result := checkChaincodeCmdParams(&cobra.Command{})

	require.Nil(result)
}

func TestCheckChaincodeCmdParamsWithoutName(t *testing.T) {
	chaincodeCtorJSON = `{ "Function":"func", "Args":["param"] }`
	chaincodePath = "some/path"
	chaincodeName = ""
	require := require.New(t)
	result := checkChaincodeCmdParams(&cobra.Command{})

	require.Error(result)
}

func TestCheckChaincodeCmdParamsWithFunctionOnly(t *testing.T) {
	chaincodeCtorJSON = `{ "Function":"func" }`
	chaincodePath = "some/path"
	chaincodeName = "somename"
	require := require.New(t)
	result := checkChaincodeCmdParams(&cobra.Command{})

	require.Error(result)
}

func TestCheckChaincodeCmdParamsEmptyCtor(t *testing.T) {
	chaincodeCtorJSON = `{}`
	chaincodePath = "some/path"
	chaincodeName = "somename"
	require := require.New(t)
	result := checkChaincodeCmdParams(&cobra.Command{})

	require.Error(result)
}

func TestCheckValidJSON(t *testing.T) {
	validJSON := `{"Args":["a","b","c"]}`
	input := &chaincodeInput{}
	if err := json.Unmarshal([]byte(validJSON), &input); err != nil {
		t.Fail()
		t.Logf("Chaincode argument error: %s", err)
		return
	}

	validJSON = `{"Function":"f", "Args":["a","b","c"]}`
	if err := json.Unmarshal([]byte(validJSON), &input); err != nil {
		t.Fail()
		t.Logf("Chaincode argument error: %s", err)
		return
	}

	validJSON = `{"Function":"f", "Args":[]}`
	if err := json.Unmarshal([]byte(validJSON), &input); err != nil {
		t.Fail()
		t.Logf("Chaincode argument error: %s", err)
		return
	}

	validJSON = `{"Function":"f"}`
	if err := json.Unmarshal([]byte(validJSON), &input); err != nil {
		t.Fail()
		t.Logf("Chaincode argument error: %s", err)
		return
	}
}

func TestCheckInvalidJSON(t *testing.T) {
	invalidJSON := `{["a","b","c"]}`
	input := &chaincodeInput{}
	if err := json.Unmarshal([]byte(invalidJSON), &input); err == nil {
		t.Fail()
		t.Logf("Bar argument error should have been caught: %s", invalidJSON)
		return
	}

	invalidJSON = `{"Function":}`
	if err := json.Unmarshal([]byte(invalidJSON), &input); err == nil {
		t.Fail()
		t.Logf("Chaincode argument error: %s", err)
		t.Logf("Bar argument error should have been caught: %s", invalidJSON)
		return
	}
}

func TestGetOrdererEndpointFromConfigTx(t *testing.T) {
	signer, err := common.GetDefaultSigner()
	assert.NoError(t, err)

	mockchain := "mockchain"
	factory.InitFactories(nil)
	config := genesisconfig.Load(genesisconfig.SampleInsecureSoloProfile, configtest.GetDevConfigDir())
	pgen := encoder.New(config)
	genesisBlock := pgen.GenesisBlockForChannel(mockchain)

	mockResponse := &pb.ProposalResponse{
		Response:    &pb.Response{Status: 200, Payload: protoutil.MarshalOrPanic(genesisBlock)},
		Endorsement: &pb.Endorsement{},
	}
	mockEndorserClient := common.GetMockEndorserClient(mockResponse, nil)

	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	ordererEndpoints, err := common.GetOrdererEndpointOfChain(mockchain, signer, mockEndorserClient, cryptoProvider)
	assert.NoError(t, err, "GetOrdererEndpointOfChain from genesis block")

	assert.Equal(t, len(ordererEndpoints), 1)
	assert.Equal(t, ordererEndpoints[0], "127.0.0.1:7050")
}

func TestGetOrdererEndpointFail(t *testing.T) {
	signer, err := common.GetDefaultSigner()
	assert.NoError(t, err)

	mockchain := "mockchain"
	factory.InitFactories(nil)

	mockResponse := &pb.ProposalResponse{
		Response:    &pb.Response{Status: 404, Payload: []byte{}},
		Endorsement: &pb.Endorsement{},
	}
	mockEndorserClient := common.GetMockEndorserClient(mockResponse, nil)

	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.NoError(t, err)
	_, err = common.GetOrdererEndpointOfChain(mockchain, signer, mockEndorserClient, cryptoProvider)
	assert.Error(t, err, "GetOrdererEndpointOfChain from invalid response")
}

const sampleCollectionConfigGood = `[
	{
		"name": "foo",
		"policy": "OR('A.member', 'B.member')",
		"requiredPeerCount": 3,
		"maxPeerCount": 483279847,
		"blockToLive":10,
		"memberOnlyRead": true,
		"memberOnlyWrite": true
	}
]`

const sampleCollectionConfigGoodWithSignaturePolicy = `[
	{
		"name": "foo",
		"policy": "OR('A.member', 'B.member')",
		"requiredPeerCount": 3,
		"maxPeerCount": 483279847,
		"blockToLive":10,
		"memberOnlyRead": true,
		"memberOnlyWrite": true,
		"endorsementPolicy": {
			"signaturePolicy": "OR('A.member', 'B.member')"
		}
	}
]`

const sampleCollectionConfigGoodWithChannelConfigPolicy = `[
	{
		"name": "foo",
		"policy": "OR('A.member', 'B.member')",
		"requiredPeerCount": 3,
		"maxPeerCount": 483279847,
		"blockToLive":10,
		"memberOnlyRead": true,
		"memberOnlyWrite": true,
		"endorsementPolicy": {
			"channelConfigPolicy": "/Channel/Application/Endorsement"
		}
	}
]`

const sampleCollectionConfigBad = `[
	{
		"name": "foo",
		"policy": "barf",
		"requiredPeerCount": 3,
		"maxPeerCount": 483279847
	}
]`

const sampleCollectionConfigBadInvalidSignaturePolicy = `[
	{
		"name": "foo",
		"policy": "OR('A.member', 'B.member')",
		"requiredPeerCount": 3,
		"maxPeerCount": 483279847,
		"blockToLive":10,
		"memberOnlyRead": true,
		"memberOnlyWrite": true,
		"endorsementPolicy": {
			"signaturePolicy": "invalid"
		}
	}
]`

const sampleCollectionConfigBadSignaturePolicyAndChannelConfigPolicy = `[
	{
		"name": "foo",
		"policy": "OR('A.member', 'B.member')",
		"requiredPeerCount": 3,
		"maxPeerCount": 483279847,
		"blockToLive":10,
		"memberOnlyRead": true,
		"memberOnlyWrite": true,
		"endorsementPolicy": {
			"signaturePolicy": "OR('A.member', 'B.member')",
			"channelConfigPolicy": "/Channel/Application/Endorsement"
		}
	}
]`

func TestCollectionParsing(t *testing.T) {
	ccp, ccpBytes, err := getCollectionConfigFromBytes([]byte(sampleCollectionConfigGood))
	assert.NoError(t, err)
	assert.NotNil(t, ccp)
	assert.NotNil(t, ccpBytes)
	conf := ccp.Config[0].GetStaticCollectionConfig()
	pol, _ := cauthdsl.FromString("OR('A.member', 'B.member')")
	assert.Equal(t, 3, int(conf.RequiredPeerCount))
	assert.Equal(t, 483279847, int(conf.MaximumPeerCount))
	assert.Equal(t, "foo", conf.Name)
	assert.True(t, proto.Equal(pol, conf.MemberOrgsPolicy.GetSignaturePolicy()))
	assert.Equal(t, 10, int(conf.BlockToLive))
	assert.Equal(t, true, conf.MemberOnlyRead)
	assert.Nil(t, conf.EndorsementPolicy)
	t.Logf("conf=%s", conf)

	ccp, ccpBytes, err = getCollectionConfigFromBytes([]byte(sampleCollectionConfigGoodWithSignaturePolicy))
	assert.NoError(t, err)
	assert.NotNil(t, ccp)
	assert.NotNil(t, ccpBytes)
	conf = ccp.Config[0].GetStaticCollectionConfig()
	pol, _ = cauthdsl.FromString("OR('A.member', 'B.member')")
	assert.Equal(t, 3, int(conf.RequiredPeerCount))
	assert.Equal(t, 483279847, int(conf.MaximumPeerCount))
	assert.Equal(t, "foo", conf.Name)
	assert.True(t, proto.Equal(pol, conf.MemberOrgsPolicy.GetSignaturePolicy()))
	assert.Equal(t, 10, int(conf.BlockToLive))
	assert.Equal(t, true, conf.MemberOnlyRead)
	assert.True(t, proto.Equal(pol, conf.EndorsementPolicy.GetSignaturePolicy()))
	t.Logf("conf=%s", conf)

	ccp, ccpBytes, err = getCollectionConfigFromBytes([]byte(sampleCollectionConfigGoodWithChannelConfigPolicy))
	assert.NoError(t, err)
	assert.NotNil(t, ccp)
	assert.NotNil(t, ccpBytes)
	conf = ccp.Config[0].GetStaticCollectionConfig()
	pol, _ = cauthdsl.FromString("OR('A.member', 'B.member')")
	assert.Equal(t, 3, int(conf.RequiredPeerCount))
	assert.Equal(t, 483279847, int(conf.MaximumPeerCount))
	assert.Equal(t, "foo", conf.Name)
	assert.True(t, proto.Equal(pol, conf.MemberOrgsPolicy.GetSignaturePolicy()))
	assert.Equal(t, 10, int(conf.BlockToLive))
	assert.Equal(t, true, conf.MemberOnlyRead)
	assert.Equal(t, "/Channel/Application/Endorsement", conf.EndorsementPolicy.GetChannelConfigPolicyReference())
	t.Logf("conf=%s", conf)

	failureTests := []struct {
		name             string
		collectionConfig string
		expectedErr      string
	}{
		{
			name:             "Invalid member orgs policy",
			collectionConfig: sampleCollectionConfigBad,
			expectedErr:      "invalid policy barf: unrecognized token 'barf' in policy string",
		},
		{
			name:             "Invalid collection config",
			collectionConfig: "barf",
			expectedErr:      "could not parse the collection configuration: invalid character 'b' looking for beginning of value",
		},
		{
			name:             "Invalid signature policy",
			collectionConfig: sampleCollectionConfigBadInvalidSignaturePolicy,
			expectedErr:      `invalid endorsement policy [&chaincode.endorsementPolicy{ChannelConfigPolicy:"", SignaturePolicy:"invalid"}]: invalid signature policy: invalid`,
		},
		{
			name:             "Signature policy and channel config policy both specified",
			collectionConfig: sampleCollectionConfigBadSignaturePolicyAndChannelConfigPolicy,
			expectedErr:      `invalid endorsement policy [&chaincode.endorsementPolicy{ChannelConfigPolicy:"/Channel/Application/Endorsement", SignaturePolicy:"OR('A.member', 'B.member')"}]: cannot specify both "--signature-policy" and "--channel-config-policy"`,
		},
	}

	for _, test := range failureTests {
		t.Run(test.name, func(t *testing.T) {
			ccp, ccpBytes, err = getCollectionConfigFromBytes([]byte(test.collectionConfig))
			assert.EqualError(t, err, test.expectedErr)
			assert.Nil(t, ccp)
			assert.Nil(t, ccpBytes)
		})
	}
}

func TestValidatePeerConnectionParams(t *testing.T) {
	defer resetFlags()
	defer viper.Reset()
	assert := assert.New(t)
	cleanup := configtest.SetDevFabricConfigPath(t)
	defer cleanup()

	// TLS disabled
	viper.Set("peer.tls.enabled", false)

	// failure - more than one peer and TLS root cert - not invoke
	resetFlags()
	peerAddresses = []string{"peer0", "peer1"}
	tlsRootCertFiles = []string{"cert0", "cert1"}
	err := validatePeerConnectionParameters("query")
	assert.Error(err)
	assert.Contains(err.Error(), "command can only be executed against one peer")

	// success - peer provided and no TLS root certs
	// TLS disabled
	resetFlags()
	peerAddresses = []string{"peer0"}
	err = validatePeerConnectionParameters("query")
	assert.NoError(err)
	assert.Nil(tlsRootCertFiles)

	// success - more TLS root certs than peers
	// TLS disabled
	resetFlags()
	peerAddresses = []string{"peer0"}
	tlsRootCertFiles = []string{"cert0", "cert1"}
	err = validatePeerConnectionParameters("invoke")
	assert.NoError(err)
	assert.Nil(tlsRootCertFiles)

	// success - multiple peers and no TLS root certs - invoke
	// TLS disabled
	resetFlags()
	peerAddresses = []string{"peer0", "peer1"}
	err = validatePeerConnectionParameters("invoke")
	assert.NoError(err)
	assert.Nil(tlsRootCertFiles)

	// TLS enabled
	viper.Set("peer.tls.enabled", true)

	// failure - uneven number of peers and TLS root certs - invoke
	// TLS enabled
	resetFlags()
	peerAddresses = []string{"peer0", "peer1"}
	tlsRootCertFiles = []string{"cert0"}
	err = validatePeerConnectionParameters("invoke")
	assert.Error(err)
	assert.Contains(err.Error(), fmt.Sprintf("number of peer addresses (%d) does not match the number of TLS root cert files (%d)", len(peerAddresses), len(tlsRootCertFiles)))

	// success - more than one peer and TLS root certs - invoke
	// TLS enabled
	resetFlags()
	peerAddresses = []string{"peer0", "peer1"}
	tlsRootCertFiles = []string{"cert0", "cert1"}
	err = validatePeerConnectionParameters("invoke")
	assert.NoError(err)

	// failure - connection profile doesn't exist
	resetFlags()
	connectionProfile = "blah"
	err = validatePeerConnectionParameters("invoke")
	assert.Error(err)
	assert.Contains(err.Error(), "error reading connection profile")

	// failure - connection profile has peer defined in channel config but
	// not in peer config
	resetFlags()
	channelID = "mychannel"
	connectionProfile = "testdata/connectionprofile-uneven.yaml"
	err = validatePeerConnectionParameters("invoke")
	assert.Error(err)
	assert.Contains(err.Error(), "defined in the channel config but doesn't have associated peer config")

	// success - connection profile exists
	resetFlags()
	channelID = "mychannel"
	connectionProfile = "testdata/connectionprofile.yaml"
	err = validatePeerConnectionParameters("invoke")
	assert.NoError(err)
}

func TestInitCmdFactoryFailures(t *testing.T) {
	defer resetFlags()
	assert := assert.New(t)

	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	assert.Nil(err)

	// failure validating peer connection parameters
	resetFlags()
	peerAddresses = []string{"peer0", "peer1"}
	tlsRootCertFiles = []string{"cert0", "cert1"}
	cf, err := InitCmdFactory("query", true, false, cryptoProvider)
	assert.Error(err)
	assert.Contains(err.Error(), "error validating peer connection parameters: 'query' command can only be executed against one peer")
	assert.Nil(cf)

	// failure - no peers supplied and endorser client is needed
	resetFlags()
	peerAddresses = []string{}
	cf, err = InitCmdFactory("query", true, false, cryptoProvider)
	assert.Error(err)
	assert.Contains(err.Error(), "no endorser clients retrieved")
	assert.Nil(cf)

	// failure - orderer client is needed, ordering endpoint is empty and no
	// endorser client supplied
	resetFlags()
	peerAddresses = nil
	cf, err = InitCmdFactory("invoke", false, true, cryptoProvider)
	assert.Error(err)
	assert.Contains(err.Error(), "no ordering endpoint or endorser client supplied")
	assert.Nil(cf)
}

func TestDeliverGroupConnect(t *testing.T) {
	defer resetFlags()
	g := NewGomegaWithT(t)

	// success
	mockDeliverClients := []*DeliverClient{
		{
			Client:  getMockDeliverClientResponseWithTxStatusAndID(pb.TxValidationCode_VALID, "txid0"),
			Address: "peer0",
		},
		{
			Client:  getMockDeliverClientResponseWithTxStatusAndID(pb.TxValidationCode_VALID, "txid0"),
			Address: "peer1",
		},
	}
	dg := DeliverGroup{
		Clients:   mockDeliverClients,
		ChannelID: "testchannel",
		Signer:    &mock.SignerSerializer{},
		Certificate: tls.Certificate{
			Certificate: [][]byte{[]byte("test")},
		},
		TxID: "txid0",
	}
	err := dg.Connect(context.Background())
	g.Expect(err).To(BeNil())

	// failure - DeliverFiltered returns error
	mockDC := &mock.PeerDeliverClient{}
	mockDC.DeliverFilteredReturns(nil, errors.New("icecream"))
	mockDeliverClients = []*DeliverClient{
		{
			Client:  mockDC,
			Address: "peer0",
		},
	}
	dg = DeliverGroup{
		Clients:   mockDeliverClients,
		ChannelID: "testchannel",
		Signer:    &mock.SignerSerializer{},
		Certificate: tls.Certificate{
			Certificate: [][]byte{[]byte("test")},
		},
		TxID: "txid0",
	}
	err = dg.Connect(context.Background())
	g.Expect(err.Error()).To(ContainSubstring("error connecting to deliver filtered"))
	g.Expect(err.Error()).To(ContainSubstring("icecream"))

	// failure - Send returns error
	mockD := &mock.Deliver{}
	mockD.SendReturns(errors.New("blah"))
	mockDC.DeliverFilteredReturns(mockD, nil)
	mockDeliverClients = []*DeliverClient{
		{
			Client:  mockDC,
			Address: "peer0",
		},
	}
	dg = DeliverGroup{
		Clients:   mockDeliverClients,
		ChannelID: "testchannel",
		Signer:    &mock.SignerSerializer{},
		Certificate: tls.Certificate{
			Certificate: [][]byte{[]byte("test")},
		},
		TxID: "txid0",
	}
	err = dg.Connect(context.Background())
	g.Expect(err.Error()).To(ContainSubstring("error sending deliver seek info"))
	g.Expect(err.Error()).To(ContainSubstring("blah"))

	// failure - deliver registration timeout
	delayChan := make(chan struct{})
	mockDCDelay := getMockDeliverClientRegisterAfterDelay(delayChan)
	mockDeliverClients = []*DeliverClient{
		{
			Client:  mockDCDelay,
			Address: "peer0",
		},
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancelFunc()
	dg = DeliverGroup{
		Clients:   mockDeliverClients,
		ChannelID: "testchannel",
		Signer:    &mock.SignerSerializer{},
		Certificate: tls.Certificate{
			Certificate: [][]byte{[]byte("test")},
		},
		TxID: "txid0",
	}
	err = dg.Connect(ctx)
	g.Expect(err.Error()).To(ContainSubstring("timed out waiting for connection to deliver on all peers"))
	close(delayChan)
}

func TestDeliverGroupWait(t *testing.T) {
	defer resetFlags()
	g := NewGomegaWithT(t)

	// success
	mockConn := &mock.Deliver{}
	filteredResp := &pb.DeliverResponse{
		Type: &pb.DeliverResponse_FilteredBlock{FilteredBlock: createFilteredBlock(pb.TxValidationCode_VALID, "txid0")},
	}
	mockConn.RecvReturns(filteredResp, nil)
	mockDeliverClients := []*DeliverClient{
		{
			Connection: mockConn,
			Address:    "peer0",
		},
	}
	dg := DeliverGroup{
		Clients:   mockDeliverClients,
		ChannelID: "testchannel",
		Signer:    &mock.SignerSerializer{},
		Certificate: tls.Certificate{
			Certificate: [][]byte{[]byte("test")},
		},
		TxID: "txid0",
	}
	err := dg.Wait(context.Background())
	g.Expect(err).To(BeNil())

	// failure - Recv returns error
	mockConn = &mock.Deliver{}
	mockConn.RecvReturns(nil, errors.New("avocado"))
	mockDeliverClients = []*DeliverClient{
		{
			Connection: mockConn,
			Address:    "peer0",
		},
	}
	dg = DeliverGroup{
		Clients:   mockDeliverClients,
		ChannelID: "testchannel",
		Signer:    &mock.SignerSerializer{},
		Certificate: tls.Certificate{
			Certificate: [][]byte{[]byte("test")},
		},
		TxID: "txid0",
	}
	err = dg.Wait(context.Background())
	g.Expect(err.Error()).To(ContainSubstring("error receiving from deliver filtered"))
	g.Expect(err.Error()).To(ContainSubstring("avocado"))

	// failure - Recv returns unexpected type
	mockConn = &mock.Deliver{}
	resp := &pb.DeliverResponse{
		Type: &pb.DeliverResponse_Block{},
	}
	mockConn.RecvReturns(resp, nil)
	mockDeliverClients = []*DeliverClient{
		{
			Connection: mockConn,
			Address:    "peer0",
		},
	}
	dg = DeliverGroup{
		Clients:   mockDeliverClients,
		ChannelID: "testchannel",
		Signer:    &mock.SignerSerializer{},
		Certificate: tls.Certificate{
			Certificate: [][]byte{[]byte("test")},
		},
		TxID: "txid0",
	}
	err = dg.Wait(context.Background())
	g.Expect(err.Error()).To(ContainSubstring("unexpected response type"))

	// failure - both connections return error
	mockConn = &mock.Deliver{}
	mockConn.RecvReturns(nil, errors.New("barbeque"))
	mockConn2 := &mock.Deliver{}
	mockConn2.RecvReturns(nil, errors.New("tofu"))
	mockDeliverClients = []*DeliverClient{
		{
			Connection: mockConn,
			Address:    "peerBBQ",
		},
		{
			Connection: mockConn2,
			Address:    "peerTOFU",
		},
	}
	dg = DeliverGroup{
		Clients:   mockDeliverClients,
		ChannelID: "testchannel",
		Signer:    &mock.SignerSerializer{},
		Certificate: tls.Certificate{
			Certificate: [][]byte{[]byte("test")},
		},
		TxID: "txid0",
	}
	err = dg.Wait(context.Background())
	g.Expect(err.Error()).To(SatisfyAny(
		ContainSubstring("barbeque"),
		ContainSubstring("tofu")))
}

func TestChaincodeInvokeOrQuery_waitForEvent(t *testing.T) {
	defer resetFlags()

	waitForEvent = true
	mockCF, err := getMockChaincodeCmdFactory()
	assert.NoError(t, err)
	peerAddresses = []string{"peer0", "peer1"}
	channelID := "testchannel"
	txID := "txid0"

	t.Run("success - deliver clients returns event with expected txid", func(t *testing.T) {
		_, err = ChaincodeInvokeOrQuery(
			&pb.ChaincodeSpec{},
			channelID,
			txID,
			true,
			mockCF.Signer,
			mockCF.Certificate,
			mockCF.EndorserClients,
			mockCF.DeliverClients,
			mockCF.BroadcastClient,
		)
		assert.NoError(t, err)
	})

	t.Run("success - one deliver client first receives block without txid and then one with txid", func(t *testing.T) {
		filteredBlocks := []*pb.FilteredBlock{
			createFilteredBlock(pb.TxValidationCode_VALID, "theseare", "notthetxidsyouarelookingfor"),
			createFilteredBlock(pb.TxValidationCode_VALID, "txid0"),
		}
		mockDCTwoBlocks := getMockDeliverClientRespondsWithFilteredBlocks(filteredBlocks)
		mockDC := getMockDeliverClientResponseWithTxStatusAndID(pb.TxValidationCode_VALID, "txid0")
		mockDeliverClients := []pb.DeliverClient{mockDCTwoBlocks, mockDC}

		_, err = ChaincodeInvokeOrQuery(
			&pb.ChaincodeSpec{},
			channelID,
			txID,
			true,
			mockCF.Signer,
			mockCF.Certificate,
			mockCF.EndorserClients,
			mockDeliverClients,
			mockCF.BroadcastClient,
		)
		assert.NoError(t, err)
	})

	t.Run("failure - one of the deliver clients returns error", func(t *testing.T) {
		mockDCErr := getMockDeliverClientWithErr("moist")
		mockDC := getMockDeliverClientResponseWithTxStatusAndID(pb.TxValidationCode_VALID, "txid0")
		mockDeliverClients := []pb.DeliverClient{mockDCErr, mockDC}

		_, err = ChaincodeInvokeOrQuery(
			&pb.ChaincodeSpec{},
			channelID,
			txID,
			true,
			mockCF.Signer,
			mockCF.Certificate,
			mockCF.EndorserClients,
			mockDeliverClients,
			mockCF.BroadcastClient,
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "moist")
	})

	t.Run("failure - transaction committed with non-success validation code", func(t *testing.T) {
		mockDC := getMockDeliverClientResponseWithTxStatusAndID(pb.TxValidationCode_VALID, "txid0")
		mockDCFail := getMockDeliverClientResponseWithTxStatusAndID(pb.TxValidationCode_ENDORSEMENT_POLICY_FAILURE, "txid0")
		mockDeliverClients := []pb.DeliverClient{mockDCFail, mockDC}

		_, err = ChaincodeInvokeOrQuery(
			&pb.ChaincodeSpec{},
			channelID,
			txID,
			true,
			mockCF.Signer,
			mockCF.Certificate,
			mockCF.EndorserClients,
			mockDeliverClients,
			mockCF.BroadcastClient,
		)
		assert.Error(t, err)
		assert.Equal(t, err.Error(), "transaction invalidated with status (ENDORSEMENT_POLICY_FAILURE)")
	})

	t.Run("failure - deliver returns response status instead of block", func(t *testing.T) {
		mockDC := &mock.PeerDeliverClient{}
		mockDF := &mock.Deliver{}
		resp := &pb.DeliverResponse{
			Type: &pb.DeliverResponse_Status{
				Status: cb.Status_FORBIDDEN,
			},
		}
		mockDF.RecvReturns(resp, nil)
		mockDC.DeliverFilteredReturns(mockDF, nil)
		mockDeliverClients := []pb.DeliverClient{mockDC}
		_, err = ChaincodeInvokeOrQuery(
			&pb.ChaincodeSpec{},
			channelID,
			txID,
			true,
			mockCF.Signer,
			mockCF.Certificate,
			mockCF.EndorserClients,
			mockDeliverClients,
			mockCF.BroadcastClient,
		)
		assert.Error(t, err)
		assert.Equal(t, err.Error(), "deliver completed with status (FORBIDDEN) before txid received")
	})

	t.Run(" failure - timeout occurs - both deliver clients don't return an event with the expected txid before timeout", func(t *testing.T) {
		delayChan := make(chan struct{})
		mockDCDelay := getMockDeliverClientRespondAfterDelay(delayChan, pb.TxValidationCode_VALID, "txid0")
		mockDeliverClients := []pb.DeliverClient{mockDCDelay, mockDCDelay}
		waitForEventTimeout = 10 * time.Millisecond

		_, err = ChaincodeInvokeOrQuery(
			&pb.ChaincodeSpec{},
			channelID,
			txID,
			true,
			mockCF.Signer,
			mockCF.Certificate,
			mockCF.EndorserClients,
			mockDeliverClients,
			mockCF.BroadcastClient,
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timed out")
		close(delayChan)
	})
}

func TestProcessProposals(t *testing.T) {
	// Build clients that return a range of status codes (for verifying each client is called).
	mockClients := []pb.EndorserClient{}
	for i := 2; i <= 5; i++ {
		response := &pb.ProposalResponse{
			Response:    &pb.Response{Status: int32(i * 100)},
			Endorsement: &pb.Endorsement{},
		}
		mockClients = append(mockClients, common.GetMockEndorserClient(response, nil))
	}
	mockErrorClient := common.GetMockEndorserClient(nil, errors.New("failed to call endorser"))
	signedProposal := &pb.SignedProposal{}
	t.Run("should process a proposal for a single peer", func(t *testing.T) {
		responses, err := processProposals([]pb.EndorserClient{mockClients[0]}, signedProposal)
		assert.NoError(t, err)
		assert.Len(t, responses, 1)
		assert.Equal(t, responses[0].Response.Status, int32(200))
	})
	t.Run("should process a proposal for multiple peers", func(t *testing.T) {
		responses, err := processProposals(mockClients, signedProposal)
		assert.NoError(t, err)
		assert.Len(t, responses, 4)
		// Sort the statuses (as they may turn up in different order) before comparing.
		statuses := []int32{}
		for _, response := range responses {
			statuses = append(statuses, response.Response.Status)
		}
		sort.Slice(statuses, func(i, j int) bool { return statuses[i] < statuses[j] })
		assert.EqualValues(t, []int32{200, 300, 400, 500}, statuses)
	})
	t.Run("should return an error from processing a proposal for a single peer", func(t *testing.T) {
		responses, err := processProposals([]pb.EndorserClient{mockErrorClient}, signedProposal)
		assert.EqualError(t, err, "failed to call endorser")
		assert.Nil(t, responses)
	})
	t.Run("should return an error from processing a proposal for a single peer within multiple peers", func(t *testing.T) {
		responses, err := processProposals([]pb.EndorserClient{mockClients[0], mockErrorClient, mockClients[1]}, signedProposal)
		assert.EqualError(t, err, "failed to call endorser")
		assert.Nil(t, responses)
	})
}
