/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"io/ioutil"
	"os"
	"testing"

	xtestutil "github.com/hyperledger/fabric/extensions/testutil"
	msptesttools "github.com/hyperledger/fabric/msp/mgmt/testtools"
	viper "github.com/spf13/oldviper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestStart(t *testing.T) {
	defer viper.Reset()
	_, _, destroy := xtestutil.SetupExtTestEnv()
	defer destroy()

	tempDir, err := ioutil.TempDir("", "startcmd")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tempDir))
	}()

	viper.Set("peer.address", "localhost:6651")
	viper.Set("peer.listenAddress", "0.0.0.0:6651")
	viper.Set("peer.chaincodeListenAddress", "0.0.0.0:6652")
	viper.Set("peer.fileSystemPath", tempDir)
	viper.Set("chaincode.executetimeout", "30s")
	viper.Set("chaincode.mode", "dev")
	viper.Set("vm.endpoint", "unix:///var/run/docker.sock")

	require.NoError(t, msptesttools.LoadMSPSetupForTesting())

	go func() {
		assert.NoError(t, Start(), "expected to successfully start command")
	}()

	grpcProbe := func(addr string) bool {
		c, err := grpc.Dial(addr, grpc.WithBlock(), grpc.WithInsecure())
		if err == nil {
			assert.NoError(t, c.Close())
			return true
		}
		return false
	}
	assert.True(t, grpcProbe("localhost:6651"))
}
