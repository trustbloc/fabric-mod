/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/
package peer

import (
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/hyperledger/fabric/core/comm"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestCacheConfigurationNegative(t *testing.T) {
	// set a bad peer.address
	viper.Set("peer.addressAutoDetect", true)
	viper.Set("peer.address", "testing.com")
	_, err := GlobalConfig()
	assert.Error(t, err, "Expected error for bad configuration")

	viper.Set("peer.addressAutoDetect", false)
	viper.Set("peer.address", "")
	_, err = GlobalConfig()
	assert.Error(t, err, "Expected error for bad configuration")

	viper.Set("peer.address", "wrongAddress")
	_, err = GlobalConfig()
	assert.Error(t, err, "Expected error for bad configuration")

}

func TestConfiguration(t *testing.T) {
	// get the interface addresses
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal("Failed to get interface addresses")
	}

	var ips []string
	for _, address := range addresses {
		// eliminate loopback interfaces
		if ip, ok := address.(*net.IPNet); ok && !ip.IP.IsLoopback() {
			ips = append(ips, ip.IP.String()+":7051")
			t.Logf("found interface address [%s]", ip.IP.String())
		}
	}

	// There is a flake where sometimes this returns no IP address.
	localIP, err := GetLocalIP()
	assert.NoError(t, err)

	var tests = []struct {
		name                string
		settings            map[string]interface{}
		validAddresses      []string
		invalidAddresses    []string
		expectedPeerAddress string
	}{
		{
			name: "test1",
			settings: map[string]interface{}{
				"peer.addressAutoDetect": false,
				"peer.address":           "testing.com:7051",
				"peer.id":                "testPeer",
			},
			validAddresses:      []string{"testing.com:7051"},
			invalidAddresses:    ips,
			expectedPeerAddress: "testing.com:7051",
		},
		{
			name: "test2",
			settings: map[string]interface{}{
				"peer.addressAutoDetect": true,
				"peer.address":           "testing.com:7051",
				"peer.id":                "testPeer",
			},
			validAddresses:      ips,
			invalidAddresses:    []string{"testing.com:7051"},
			expectedPeerAddress: net.JoinHostPort(localIP, "7051"),
		},
		{
			name: "test3",
			settings: map[string]interface{}{
				"peer.addressAutoDetect": false,
				"peer.address":           "0.0.0.0:7051",
				"peer.id":                "testPeer",
			},
			validAddresses:      []string{fmt.Sprintf("%s:7051", localIP)},
			invalidAddresses:    []string{"0.0.0.0:7051"},
			expectedPeerAddress: net.JoinHostPort(localIP, "7051"),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			for k, v := range test.settings {
				viper.Set(k, v)
			}
			// load Config file
			_, err := GlobalConfig()
			assert.NoError(t, err, "GlobalConfig returned unexpected error")
		})
	}
}

func TestGetServerConfig(t *testing.T) {
	// good config without TLS
	viper.Set("peer.tls.enabled", false)
	sc, _ := GetServerConfig()
	assert.Equal(t, false, sc.SecOpts.UseTLS, "ServerConfig.SecOpts.UseTLS should be false")

	// keepalive options
	assert.Equal(t, comm.DefaultKeepaliveOptions, sc.KaOpts, "ServerConfig.KaOpts should be set to default values")
	viper.Set("peer.keepalive.interval", "60m")
	sc, _ = GetServerConfig()
	assert.Equal(t, time.Duration(60)*time.Minute, sc.KaOpts.ServerInterval, "ServerConfig.KaOpts.ServerInterval should be set to 60 min")
	viper.Set("peer.keepalive.timeout", "30s")
	sc, _ = GetServerConfig()
	assert.Equal(t, time.Duration(30)*time.Second, sc.KaOpts.ServerTimeout, "ServerConfig.KaOpts.ServerTimeout should be set to 30 sec")
	viper.Set("peer.keepalive.minInterval", "2m")
	sc, _ = GetServerConfig()
	assert.Equal(t, time.Duration(2)*time.Minute, sc.KaOpts.ServerMinInterval, "ServerConfig.KaOpts.ServerMinInterval should be set to 2 min")

	// good config with TLS
	viper.Set("peer.tls.enabled", true)
	viper.Set("peer.tls.cert.file", filepath.Join("testdata", "Org1-server1-cert.pem"))
	viper.Set("peer.tls.key.file", filepath.Join("testdata", "Org1-server1-key.pem"))
	viper.Set("peer.tls.rootcert.file", filepath.Join("testdata", "Org1-cert.pem"))
	sc, _ = GetServerConfig()
	assert.Equal(t, true, sc.SecOpts.UseTLS, "ServerConfig.SecOpts.UseTLS should be true")
	assert.Equal(t, false, sc.SecOpts.RequireClientCert, "ServerConfig.SecOpts.RequireClientCert should be false")
	viper.Set("peer.tls.clientAuthRequired", true)
	viper.Set("peer.tls.clientRootCAs.files", []string{
		filepath.Join("testdata", "Org1-cert.pem"),
		filepath.Join("testdata", "Org2-cert.pem"),
	})
	sc, _ = GetServerConfig()
	assert.Equal(t, true, sc.SecOpts.RequireClientCert, "ServerConfig.SecOpts.RequireClientCert should be true")
	assert.Equal(t, 2, len(sc.SecOpts.ClientRootCAs), "ServerConfig.SecOpts.ClientRootCAs should contain 2 entries")

	// bad config with TLS
	viper.Set("peer.tls.rootcert.file", filepath.Join("testdata", "Org11-cert.pem"))
	_, err := GetServerConfig()
	assert.Error(t, err, "GetServerConfig should return error with bad root cert path")
	viper.Set("peer.tls.cert.file", filepath.Join("testdata", "Org11-cert.pem"))
	_, err = GetServerConfig()
	assert.Error(t, err, "GetServerConfig should return error with bad tls cert path")

	// disable TLS for remaining tests
	viper.Set("peer.tls.enabled", false)
	viper.Set("peer.tls.clientAuthRequired", false)
}

func TestGetClientCertificate(t *testing.T) {
	viper.Set("peer.tls.key.file", "")
	viper.Set("peer.tls.cert.file", "")
	viper.Set("peer.tls.clientKey.file", "")
	viper.Set("peer.tls.clientCert.file", "")

	// neither client nor server key pairs set - expect error
	_, err := GetClientCertificate()
	assert.Error(t, err)

	viper.Set("peer.tls.key.file", "")
	viper.Set("peer.tls.cert.file", filepath.Join("testdata", "Org1-server1-cert.pem"))
	// missing server key file - expect error
	_, err = GetClientCertificate()
	assert.Error(t, err)

	viper.Set("peer.tls.key.file", filepath.Join("testdata", "Org1-server1-key.pem"))
	viper.Set("peer.tls.cert.file", "")
	// missing server cert file - expect error
	_, err = GetClientCertificate()
	assert.Error(t, err)

	// set server TLS settings to ensure we get the client TLS settings
	// when they are set properly
	viper.Set("peer.tls.key.file", filepath.Join("testdata", "Org1-server1-key.pem"))
	viper.Set("peer.tls.cert.file", filepath.Join("testdata", "Org1-server1-cert.pem"))

	// peer.tls.clientCert.file not set - expect error
	viper.Set("peer.tls.clientKey.file", filepath.Join("testdata", "Org2-server1-key.pem"))
	_, err = GetClientCertificate()
	assert.Error(t, err)

	// peer.tls.clientKey.file not set - expect error
	viper.Set("peer.tls.clientKey.file", "")
	viper.Set("peer.tls.clientCert.file", filepath.Join("testdata", "Org2-server1-cert.pem"))
	_, err = GetClientCertificate()
	assert.Error(t, err)

	// client auth required and clientKey/clientCert set
	expected, err := tls.LoadX509KeyPair(
		filepath.Join("testdata", "Org2-server1-cert.pem"),
		filepath.Join("testdata", "Org2-server1-key.pem"),
	)
	if err != nil {
		t.Fatalf("Failed to load test certificate (%s)", err)
	}
	viper.Set("peer.tls.clientKey.file", filepath.Join("testdata", "Org2-server1-key.pem"))
	cert, err := GetClientCertificate()
	assert.NoError(t, err)
	assert.Equal(t, expected, cert)

	// client auth required and clientKey/clientCert not set - expect
	// client cert to be the server cert
	viper.Set("peer.tls.clientKey.file", "")
	viper.Set("peer.tls.clientCert.file", "")
	expected, err = tls.LoadX509KeyPair(
		filepath.Join("testdata", "Org1-server1-cert.pem"),
		filepath.Join("testdata", "Org1-server1-key.pem"),
	)
	if err != nil {
		t.Fatalf("Failed to load test certificate (%s)", err)
	}
	cert, err = GetClientCertificate()
	assert.NoError(t, err)
	assert.Equal(t, expected, cert)
}

func TestGlobalConfig(t *testing.T) {
	//Capture the configuration from viper
	viper.Set("peer.addressAutoDetect", false)
	viper.Set("peer.address", "localhost:8080")
	viper.Set("peer.id", "testPeerID")
	viper.Set("peer.localMspId", "SampleOrg")
	viper.Set("peer.listenAddress", "0.0.0.0:7051")
	viper.Set("peer.authentication.timewindow", "15m")
	viper.Set("peer.tls.enabled", "false")
	viper.Set("peer.networkId", "testNetwork")
	viper.Set("peer.limits.concurrency.qscc", 5000)
	viper.Set("peer.discovery.enabled", true)
	viper.Set("peer.profile.enabled", false)
	viper.Set("peer.profile.listenAddress", "peer.authentication.timewindow")
	viper.Set("peer.discovery.orgMembersAllowedAccess", false)
	viper.Set("peer.discovery.authCacheEnabled", true)
	viper.Set("peer.discovery.authCacheMaxSize", 1000)
	viper.Set("peer.discovery.authCachePurgeRetentionRatio", 0.75)
	viper.Set("peer.chaincodeListenAddress", "0.0.0.0:7052")
	viper.Set("peer.chaincodeAddress", "0.0.0.0:7052")
	viper.Set("peer.adminService.listenAddress", "0.0.0.0:7055")
	viper.Set("peer.validatorPoolSize", 1)

	viper.Set("vm.endpoint", "unix:///var/run/docker.sock")
	viper.Set("vm.docker.tls.enabled", false)
	viper.Set("vm.docker.attachStdout", false)

	viper.Set("operations.listenAddress", "127.0.0.1:9443")
	viper.Set("operations.tls.enabled", false)
	viper.Set("operations.tls.cert.file", "test/tls/cert/file")
	viper.Set("operations.tls.key.file", "test/tls/key/file")
	viper.Set("operations.tls.clientAuthRequired", false)
	viper.Set("operations.tls.clientRootCAs.files", []string{"file1, file2"})

	viper.Set("metrics.provider", "disabled")
	viper.Set("metrics.statsd.network", "udp")
	viper.Set("metrics.statsd.address", "127.0.0.1:8125")
	viper.Set("metrics.statsd.writeInterval", "10s")
	viper.Set("metrics.statsd.prefix", "testPrefix")

	viper.Set("chaincode.pull", false)

	coreConfig, err := GlobalConfig()
	assert.NoError(t, err)

	expectedConfig := &Config{
		LocalMSPID:                            "SampleOrg",
		ListenAddress:                         "0.0.0.0:7051",
		AuthenticationTimeWindow:              15 * time.Minute,
		PeerTLSEnabled:                        false,
		PeerAddress:                           "localhost:8080",
		PeerID:                                "testPeerID",
		NetworkID:                             "testNetwork",
		LimitsConcurrencyQSCC:                 5000,
		DiscoveryEnabled:                      true,
		ProfileEnabled:                        false,
		ProfileListenAddress:                  "peer.authentication.timewindow",
		DiscoveryOrgMembersAllowed:            false,
		DiscoveryAuthCacheEnabled:             true,
		DiscoveryAuthCacheMaxSize:             1000,
		DiscoveryAuthCachePurgeRetentionRatio: 0.75,
		ChaincodeListenAddress:                "0.0.0.0:7052",
		ChaincodeAddress:                      "0.0.0.0:7052",
		AdminListenAddress:                    "0.0.0.0:7055",
		ValidatorPoolSize:                     1,

		VMEndpoint:           "unix:///var/run/docker.sock",
		VMDockerTLSEnabled:   false,
		VMDockerAttachStdout: false,

		ChaincodePull: false,

		OperationsListenAddress:         "127.0.0.1:9443",
		OperationsTLSEnabled:            false,
		OperationsTLSCertFile:           "test/tls/cert/file",
		OperationsTLSKeyFile:            "test/tls/key/file",
		OperationsTLSClientAuthRequired: false,
		OperationsTLSClientRootCAs:      []string{"file1, file2"},

		MetricsProvider:     "disabled",
		StatsdNetwork:       "udp",
		StatsdAaddress:      "127.0.0.1:8125",
		StatsdWriteInterval: 10 * time.Second,
		StatsdPrefix:        "testPrefix",
	}

	assert.Equal(t, coreConfig, expectedConfig)
}
