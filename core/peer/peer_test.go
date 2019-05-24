/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package peer

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/viper"

	configtxtest "github.com/hyperledger/fabric/common/configtx/test"
	"github.com/hyperledger/fabric/common/metrics/disabled"
	mscc "github.com/hyperledger/fabric/common/mocks/scc"
	"github.com/hyperledger/fabric/core/chaincode/platforms"
	"github.com/hyperledger/fabric/core/comm"
	"github.com/hyperledger/fabric/core/committer/txvalidator/plugin"
	"github.com/hyperledger/fabric/core/deliverservice"
	"github.com/hyperledger/fabric/core/deliverservice/blocksprovider"
	validation "github.com/hyperledger/fabric/core/handlers/validation/api"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/mock"
	ledgermocks "github.com/hyperledger/fabric/core/ledger/mock"
	extmocks "github.com/hyperledger/fabric/extensions/mocks"
	xtestutil "github.com/hyperledger/fabric/extensions/testutil"
	"github.com/hyperledger/fabric/gossip/api"
	"github.com/hyperledger/fabric/gossip/service"
	peergossip "github.com/hyperledger/fabric/internal/peer/gossip"
	"github.com/hyperledger/fabric/internal/peer/gossip/mocks"
	"github.com/hyperledger/fabric/msp/mgmt"
	msptesttools "github.com/hyperledger/fabric/msp/mgmt/testtools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

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

func TestNewPeerServer(t *testing.T) {
	server, err := NewPeerServer(":4050", comm.ServerConfig{})
	assert.NoError(t, err, "NewPeerServer returned unexpected error")
	assert.Equal(t, "[::]:4050", server.Address(), "NewPeerServer returned the wrong address")
	server.Stop()

	_, err = NewPeerServer("", comm.ServerConfig{})
	assert.Error(t, err, "expected NewPeerServer to return error with missing address")
}

func TestInitChain(t *testing.T) {
	chainId := "testChain"
	chainInitializer = func(cid string) {
		assert.Equal(t, chainId, cid, "chainInitializer received unexpected cid")
	}
	InitChain(chainId)
}

func TestInitialize(t *testing.T) {

	resetExtTestEnv()

	rootFSPath, err := ioutil.TempDir("", "ledgersData")
	if err != nil {
		t.Fatalf("Failed to create ledger directory: %s", err)
	}
	defer os.RemoveAll(rootFSPath)

	Initialize(
		nil,
		(&mscc.MocksccProviderFactory{}).NewSystemChaincodeProvider(),
		plugin.MapBasedMapper(map[string]validation.PluginFactory{}),
		nil,
		&ledgermocks.DeployedChaincodeInfoProvider{},
		nil,
		&disabled.Provider{},
		nil,
		nil,
		&ledger.Config{
			RootFSPath: rootFSPath,
			StateDB: &ledger.StateDB{
				LevelDBPath: filepath.Join(rootFSPath, "stateleveldb"),
			},
			PrivateData: &ledger.PrivateData{
				StorePath:       filepath.Join(rootFSPath, "pvtdataStore"),
				MaxBatchSize:    5000,
				BatchesInterval: 1000,
				PurgeInterval:   100,
			},
			HistoryDB: &ledger.HistoryDB{
				Enabled: true,
			},
		},
		runtime.NumCPU(),
		&extmocks.DataProvider{},
	)
}

func TestCreateChainFromBlock(t *testing.T) {

	resetExtTestEnv()

	peerFSPath, err := ioutil.TempDir("", "ledgersData")
	if err != nil {
		t.Fatalf("Failed to create peer directory: %s", err)
	}
	defer os.RemoveAll(peerFSPath)
	viper.Set("peer.fileSystemPath", peerFSPath)

	Initialize(
		nil,
		(&mscc.MocksccProviderFactory{}).NewSystemChaincodeProvider(),
		plugin.MapBasedMapper(map[string]validation.PluginFactory{}),
		&platforms.Registry{},
		&ledgermocks.DeployedChaincodeInfoProvider{},
		nil,
		&disabled.Provider{},
		nil,
		nil,
		&ledger.Config{
			RootFSPath: filepath.Join(peerFSPath, "ledgersData"),
			StateDB: &ledger.StateDB{
				LevelDBPath: filepath.Join(peerFSPath, "ledgersData", "stateleveldb"),
			},
			PrivateData: &ledger.PrivateData{
				StorePath:       filepath.Join(peerFSPath, "ledgersData", "pvtdataStore"),
				MaxBatchSize:    5000,
				BatchesInterval: 1000,
				PurgeInterval:   100,
			},
			HistoryDB: &ledger.HistoryDB{
				Enabled: true,
			},
		},
		runtime.NumCPU(),
		&extmocks.DataProvider{},
	)
	testChainID := fmt.Sprintf("mytestchainid-%d", rand.Int())
	defer reset(testChainID)
	block, err := configtxtest.MakeGenesisBlock(testChainID)
	if err != nil {
		fmt.Printf("Failed to create a config block, err %s\n", err)
		t.FailNow()
	}

	// Initialize gossip service
	grpcServer := grpc.NewServer()
	socket, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	msptesttools.LoadMSPSetupForTesting()

	signer := mgmt.GetLocalSigningIdentityOrPanic()
	messageCryptoService := peergossip.NewMCS(&mocks.ChannelPolicyManagerGetter{}, signer, mgmt.NewDeserializersManager())
	secAdv := peergossip.NewSecurityAdvisor(mgmt.NewDeserializersManager())
	var defaultSecureDialOpts = func() []grpc.DialOption {
		var dialOpts []grpc.DialOption
		dialOpts = append(dialOpts, grpc.WithInsecure())
		return dialOpts
	}
	err = service.InitGossipServiceCustomDeliveryFactory(
		signer,
		&disabled.Provider{},
		socket.Addr().String(),
		grpcServer,
		nil,
		&mockDeliveryClientFactory{},
		messageCryptoService,
		secAdv,
		defaultSecureDialOpts,
	)

	assert.NoError(t, err)

	go grpcServer.Serve(socket)
	defer grpcServer.Stop()

	err = CreateChainFromBlock(block, nil, &mock.DeployedChaincodeInfoProvider{}, nil, nil)
	if err != nil {
		t.Fatalf("failed to create chain %s", err)
	}

	// Correct ledger
	ledger := GetLedger(testChainID)
	if ledger == nil {
		t.Fatalf("failed to get correct ledger")
	}

	// Get config block from ledger
	block, err = getCurrConfigBlockFromLedger(ledger)
	assert.NoError(t, err, "Failed to get config block from ledger")
	assert.NotNil(t, block, "Config block should not be nil")
	assert.Equal(t, uint64(0), block.Header.Number, "config block should have been block 0")

	// Bad ledger
	ledger = GetLedger("BogusChain")
	if ledger != nil {
		t.Fatalf("got a bogus ledger")
	}

	// Correct block
	block = GetCurrConfigBlock(testChainID)
	if block == nil {
		t.Fatalf("failed to get correct block")
	}

	cfgSupport := configSupport{}
	chCfg := cfgSupport.GetChannelConfig(testChainID)
	assert.NotNil(t, chCfg, "failed to get channel config")

	// Bad block
	block = GetCurrConfigBlock("BogusBlock")
	if block != nil {
		t.Fatalf("got a bogus block")
	}

	// Correct PolicyManager
	pmgr := GetPolicyManager(testChainID)
	if pmgr == nil {
		t.Fatal("failed to get PolicyManager")
	}

	// Bad PolicyManager
	pmgr = GetPolicyManager("BogusChain")
	if pmgr != nil {
		t.Fatal("got a bogus PolicyManager")
	}

	// PolicyManagerGetter
	pmg := NewChannelPolicyManagerGetter()
	assert.NotNil(t, pmg, "PolicyManagerGetter should not be nil")

	pmgr, ok := pmg.Manager(testChainID)
	assert.NotNil(t, pmgr, "PolicyManager should not be nil")
	assert.Equal(t, true, ok, "expected Manage() to return true")

	SetCurrConfigBlock(block, testChainID)

	channels := GetChannelsInfo()
	if len(channels) != 1 {
		t.Fatalf("incorrect number of channels")
	}

	// cleanup the chain referenes to enable execution with -count n
	chains.Lock()
	chains.list = map[string]*chain{}
	chains.Unlock()
}

func TestGetLocalIP(t *testing.T) {
	ip, err := GetLocalIP()
	assert.NoError(t, err)
	t.Log(ip)
}

func TestDeliverSupportManager(t *testing.T) {
	_, _, destroy := xtestutil.SetupExtTestEnv()
	defer destroy()

	// reset chains for testing
	MockInitialize()

	manager := &DeliverChainManager{}
	chainSupport := manager.GetChain("fake")
	assert.Nil(t, chainSupport, "chain support should be nil")

	MockCreateChain("testchain")
	chainSupport = manager.GetChain("testchain")
	assert.NotNil(t, chainSupport, "chain support should not be nil")
}
