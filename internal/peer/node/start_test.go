/*
Copyright Hitachi America, Ltd.

SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"bytes"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/core/handlers/library"
	"github.com/hyperledger/fabric/core/testutil"
	xtestutil "github.com/hyperledger/fabric/extensions/testutil"
	"github.com/hyperledger/fabric/internal/peer/node/mock"
	msptesttools "github.com/hyperledger/fabric/msp/mgmt/testtools"
	"github.com/mitchellh/mapstructure"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

func TestStartCmd(t *testing.T) {
	defer viper.Reset()
	g := NewGomegaWithT(t)

	tempDir, err := ioutil.TempDir("", "startcmd")
	g.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)
	//setup extension test environment
	_, _, destroy := xtestutil.SetupExtTestEnv()
	defer destroy()

	viper.Set("peer.address", "localhost:6051")
	viper.Set("peer.listenAddress", "0.0.0.0:6051")
	viper.Set("peer.chaincodeListenAddress", "0.0.0.0:6052")
	viper.Set("peer.fileSystemPath", tempDir)
	viper.Set("chaincode.executetimeout", "30s")
	viper.Set("chaincode.mode", "dev")
	viper.Set("vm.endpoint", "unix:///var/run/docker.sock")

	msptesttools.LoadMSPSetupForTesting()

	go func() {
		cmd := startCmd()
		assert.NoError(t, cmd.Execute(), "expected to successfully start command")
	}()

	grpcProbe := func(addr string) bool {
		c, err := grpc.Dial(addr, grpc.WithBlock(), grpc.WithInsecure())
		if err == nil {
			c.Close()
			return true
		}
		return false
	}
	g.Eventually(grpcProbe("localhost:6051")).Should(BeTrue())
}

func TestHandlerMap(t *testing.T) {
	config1 := `
  peer:
    handlers:
      authFilters:
        - name: filter1
          library: /opt/lib/filter1.so
        - name: filter2
  `
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(config1)))
	assert.NoError(t, err)

	var libConf library.Config
	err = mapstructure.Decode(viper.Get("peer.handlers"), &libConf)
	assert.NoError(t, err)
	assert.Len(t, libConf.AuthFilters, 2, "expected two filters")
	assert.Equal(t, "/opt/lib/filter1.so", libConf.AuthFilters[0].Library)
	assert.Equal(t, "filter2", libConf.AuthFilters[1].Name)
}

func TestComputeChaincodeEndpoint(t *testing.T) {
	var tests = []struct {
		peerAddress            string
		chaincodeAddress       string
		chaincodeListenAddress string
		expectedError          string
		expectedEndpoint       string
	}{
		{
			peerAddress:   "0.0.0.0",
			expectedError: "invalid endpoint for chaincode to connect",
		},
		{
			peerAddress:      "127.0.0.1",
			expectedEndpoint: "127.0.0.1:7052",
		},
		{
			peerAddress:            "0.0.0.0",
			chaincodeListenAddress: "0.0.0.0:8052",
			expectedError:          "invalid endpoint for chaincode to connect",
		},
		{
			peerAddress:            "127.0.0.1",
			chaincodeListenAddress: "0.0.0.0:8052",
			expectedEndpoint:       "127.0.0.1:8052",
		},
		{
			peerAddress:            "127.0.0.1",
			chaincodeListenAddress: "127.0.0.1:8052",
			expectedEndpoint:       "127.0.0.1:8052",
		},
		{
			peerAddress:            "127.0.0.1",
			chaincodeListenAddress: "abc",
			expectedError:          "address abc: missing port in address",
		},
		{
			peerAddress:      "127.0.0.1",
			chaincodeAddress: "0.0.0.0:9052",
			expectedError:    "invalid endpoint for chaincode to connect",
		},
		{
			peerAddress:      "127.0.0.1",
			chaincodeAddress: "127.0.0.2:9052",
			expectedEndpoint: "127.0.0.2:9052",
		},
		{
			peerAddress:            "127.0.0.1",
			chaincodeAddress:       "bcd",
			chaincodeListenAddress: "ignored",
			expectedError:          "address bcd: missing port in address",
		},
		{
			peerAddress:            "127.0.0.1",
			chaincodeAddress:       "127.0.0.2:9052",
			chaincodeListenAddress: "ignored",
			expectedEndpoint:       "127.0.0.2:9052",
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			ccEndpoint, err := computeChaincodeEndpoint(tt.chaincodeAddress, tt.chaincodeListenAddress, tt.peerAddress)
			if tt.expectedError != "" {
				assert.EqualErrorf(t, err, tt.expectedError, "peerAddress: %q, ccListenAddr: %q, ccAddr: %q", tt.peerAddress, tt.chaincodeListenAddress, tt.chaincodeAddress)
				return
			}
			assert.NoErrorf(t, err, "peerAddress: %q, ccListenAddr: %q, ccAddr: %q", tt.peerAddress, tt.chaincodeListenAddress, tt.chaincodeAddress)
			assert.Equalf(t, tt.expectedEndpoint, ccEndpoint, "peerAddress: %q, ccListenAddr: %q, ccAddr: %q", tt.peerAddress, tt.chaincodeListenAddress, tt.chaincodeAddress)
		})
	}
}

func TestGetDockerHostConfig(t *testing.T) {
	testutil.SetupTestConfig()
	hostConfig := getDockerHostConfig()
	assert.NotNil(t, hostConfig)
	assert.Equal(t, "host", hostConfig.NetworkMode)
	assert.Equal(t, "json-file", hostConfig.LogConfig.Type)
	assert.Equal(t, "50m", hostConfig.LogConfig.Config["max-size"])
	assert.Equal(t, "5", hostConfig.LogConfig.Config["max-file"])
	assert.Equal(t, int64(1024*1024*1024*2), hostConfig.Memory)
	assert.Equal(t, int64(0), hostConfig.CPUShares)
}

func TestResetLoop(t *testing.T) {
	peerLedger := &mock.PeerLedger{}
	peerLedger.GetBlockchainInfoReturnsOnCall(
		0,
		&common.BlockchainInfo{
			Height: uint64(1),
		},
		nil,
	)

	peerLedger.GetBlockchainInfoReturnsOnCall(
		1,
		&common.BlockchainInfo{
			Height: uint64(5),
		},
		nil,
	)

	peerLedger.GetBlockchainInfoReturnsOnCall(
		2,
		&common.BlockchainInfo{
			Height: uint64(11),
		},
		nil,
	)

	peerLedger.GetBlockchainInfoReturnsOnCall(
		3,
		&common.BlockchainInfo{
			Height: uint64(11),
		},
		nil,
	)

	getLedger := &mock.GetLedger{}
	getLedger.Returns(peerLedger)
	resetFilter := &reset{
		reject: true,
	}

	ledgerIDs := []string{"testchannel", "testchannel2"}
	heights := map[string]uint64{
		"testchannel":  uint64(10),
		"testchannel2": uint64(10),
	}

	resetLoop(resetFilter, heights, ledgerIDs, getLedger.Spy, 1*time.Second)
	assert.False(t, resetFilter.reject)
	assert.Equal(t, 4, peerLedger.GetBlockchainInfoCallCount())
}
