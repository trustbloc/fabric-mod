/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package common_test

import (
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hyperledger/fabric/internal/peer/common"
	"github.com/hyperledger/fabric/internal/pkg/comm"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func initPeerTestEnv(t *testing.T) (cleanup func()) {
	t.Helper()
	cfgPath := "./testdata"
	os.Setenv("FABRIC_CFG_PATH", cfgPath)
	viper.Reset()
	_ = common.InitConfig("test")

	return func() {
		err := os.Unsetenv("FABRIC_CFG_PATH")
		require.NoError(t, err)
		viper.Reset()
	}
}

func TestNewPeerClientFromEnv(t *testing.T) {
	cleanup := initPeerTestEnv(t)
	defer cleanup()

	pClient, err := common.NewPeerClientFromEnv()
	require.NoError(t, err)
	require.NotNil(t, pClient)

	viper.Set("peer.tls.enabled", true)
	pClient, err = common.NewPeerClientFromEnv()
	require.NoError(t, err)
	require.NotNil(t, pClient)

	viper.Set("peer.tls.enabled", true)
	viper.Set("peer.tls.clientAuthRequired", true)
	pClient, err = common.NewPeerClientFromEnv()
	require.NoError(t, err)
	require.NotNil(t, pClient)

	// bad key file
	badKeyFile := filepath.Join("certs", "bad.key")
	viper.Set("peer.tls.clientKey.file", badKeyFile)
	pClient, err = common.NewPeerClientFromEnv()
	require.Contains(t, err.Error(), "failed to create PeerClient from config")
	require.Nil(t, pClient)

	// bad cert file path
	viper.Set("peer.tls.clientCert.file", "./nocert.crt")
	pClient, err = common.NewPeerClientFromEnv()
	require.Contains(t, err.Error(), "unable to load peer.tls.clientCert.file")
	require.Contains(t, err.Error(), "failed to load config for PeerClient")
	require.Nil(t, pClient)

	// bad key file path
	viper.Set("peer.tls.clientKey.file", "./nokey.key")
	pClient, err = common.NewPeerClientFromEnv()
	require.Contains(t, err.Error(), "unable to load peer.tls.clientKey.file")
	require.Nil(t, pClient)

	// bad ca path
	viper.Set("peer.tls.rootcert.file", "noroot.crt")
	pClient, err = common.NewPeerClientFromEnv()
	require.Contains(t, err.Error(), "unable to load peer.tls.rootcert.file")
	require.Nil(t, pClient)
}

func TestPeerClient(t *testing.T) {
	cleanup := initPeerTestEnv(t)
	defer cleanup()

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("error creating server for test: %v", err)
	}
	defer lis.Close()
	srv, err := comm.NewGRPCServerFromListener(lis, comm.ServerConfig{})
	if err != nil {
		t.Fatalf("error creating gRPC server for test: %v", err)
	}
	go srv.Start()
	defer srv.Stop()
	viper.Set("peer.address", lis.Addr().String())
	pClient1, err := common.NewPeerClientFromEnv()
	if err != nil {
		t.Fatalf("failed to create PeerClient for test: %v", err)
	}
	eClient, err := pClient1.Endorser()
	require.NoError(t, err)
	require.NotNil(t, eClient)
	eClient, err = common.GetEndorserClient("", "")
	require.NoError(t, err)
	require.NotNil(t, eClient)

	dClient, err := pClient1.Deliver()
	require.NoError(t, err)
	require.NotNil(t, dClient)
	dClient, err = common.GetDeliverClient("", "")
	require.NoError(t, err)
	require.NotNil(t, dClient)
}

func TestPeerClientTimeout(t *testing.T) {
	t.Run("PeerClient.GetEndorser() timeout", func(t *testing.T) {
		cleanup := initPeerTestEnv(t)
		viper.Set("peer.client.connTimeout", 10*time.Millisecond)
		defer cleanup()
		pClient, err := common.NewPeerClientFromEnv()
		if err != nil {
			t.Fatalf("failed to create PeerClient for test: %v", err)
		}
		_, err = pClient.Endorser()
		require.Contains(t, err.Error(), "endorser client failed to connect")
	})
	t.Run("GetEndorserClient() timeout", func(t *testing.T) {
		cleanup := initPeerTestEnv(t)
		viper.Set("peer.client.connTimeout", 10*time.Millisecond)
		defer cleanup()
		_, err := common.GetEndorserClient("", "")
		require.Contains(t, err.Error(), "endorser client failed to connect")
	})
	t.Run("PeerClient.Deliver() timeout", func(t *testing.T) {
		cleanup := initPeerTestEnv(t)
		viper.Set("peer.client.connTimeout", 10*time.Millisecond)
		defer cleanup()
		pClient, err := common.NewPeerClientFromEnv()
		if err != nil {
			t.Fatalf("failed to create PeerClient for test: %v", err)
		}
		_, err = pClient.Deliver()
		require.Contains(t, err.Error(), "deliver client failed to connect")
	})
	t.Run("GetDeliverClient() timeout", func(t *testing.T) {
		cleanup := initPeerTestEnv(t)
		viper.Set("peer.client.connTimeout", 10*time.Millisecond)
		defer cleanup()
		_, err := common.GetDeliverClient("", "")
		require.Contains(t, err.Error(), "deliver client failed to connect")
	})
	t.Run("PeerClient.Certificate()", func(t *testing.T) {
		cleanup := initPeerTestEnv(t)
		defer cleanup()
		pClient, err := common.NewPeerClientFromEnv()
		if err != nil {
			t.Fatalf("failed to create PeerClient for test: %v", err)
		}
		cert := pClient.Certificate()
		require.NotNil(t, cert)
	})
	t.Run("GetCertificate()", func(t *testing.T) {
		cleanup := initPeerTestEnv(t)
		defer cleanup()
		cert, err := common.GetCertificate()
		require.NotEqual(t, cert, &tls.Certificate{})
		require.NoError(t, err)
	})
}

func TestNewPeerClientForAddress(t *testing.T) {
	cleanup := initPeerTestEnv(t)
	defer cleanup()

	// TLS disabled
	viper.Set("peer.tls.enabled", false)

	// success case
	pClient, err := common.NewPeerClientForAddress("testPeer", "")
	require.NoError(t, err)
	require.NotNil(t, pClient)

	// failure - no peer address supplied
	pClient, err = common.NewPeerClientForAddress("", "")
	require.Contains(t, err.Error(), "peer address must be set")
	require.Nil(t, pClient)

	// TLS enabled
	viper.Set("peer.tls.enabled", true)

	// Enable clientAuthRequired
	viper.Set("peer.tls.clientAuthRequired", true)

	// success case
	pClient, err = common.NewPeerClientForAddress("tlsPeer", "./testdata/certs/ca.crt")
	require.NoError(t, err)
	require.NotNil(t, pClient)

	// failure - bad tls root cert file
	pClient, err = common.NewPeerClientForAddress("badPeer", "bad.crt")
	require.Contains(t, err.Error(), "unable to load TLS root cert file from bad.crt")
	require.Nil(t, pClient)

	// failure - empty tls root cert file
	pClient, err = common.NewPeerClientForAddress("badPeer", "")
	require.Contains(t, err.Error(), "tls root cert file must be set")
	require.Nil(t, pClient)

	// failure - empty tls root cert file
	viper.Set("peer.tls.clientCert.file", "./nocert.crt")
	pClient, err = common.NewPeerClientForAddress("badPeer", "")
	require.Contains(t, err.Error(), "unable to load peer.tls.clientCert.file")
	require.Nil(t, pClient)

	// bad key file
	viper.Set("peer.tls.clientKey.file", "./nokey.key")
	viper.Set("peer.client.connTimeout", time.Duration(0))
	pClient, err = common.NewPeerClientForAddress("badPeer", "")
	require.Contains(t, err.Error(), "unable to load peer.tls.clientKey.file")
	require.Nil(t, pClient)

}

func TestGetClients_AddressError(t *testing.T) {
	cleanup := initPeerTestEnv(t)
	defer cleanup()

	viper.Set("peer.tls.enabled", true)

	// failure
	eClient, err := common.GetEndorserClient("peer0", "")
	require.Contains(t, err.Error(), "tls root cert file must be set")
	require.Nil(t, eClient)

	dClient, err := common.GetDeliverClient("peer0", "")
	require.Contains(t, err.Error(), "tls root cert file must be set")
	require.Nil(t, dClient)
}
