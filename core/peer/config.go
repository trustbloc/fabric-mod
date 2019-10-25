/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// The 'viper' package for configuration handling is very flexible, but has
// been found to have extremely poor performance when configuration values are
// accessed repeatedly. The function CacheConfiguration() defined here caches
// all configuration values that are accessed frequently.  These parameters
// are now presented as function calls that access local configuration
// variables.  This seems to be the most robust way to represent these
// parameters in the face of the numerous ways that configuration files are
// loaded and used (e.g, normal usage vs. test cases).

// The CacheConfiguration() function is allowed to be called globally to
// ensure that the correct values are always cached; See for example how
// certain parameters are forced in 'ChaincodeDevMode' in main.go.

package peer

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"
	"runtime"
	"time"

	"github.com/hyperledger/fabric/core/comm"
	"github.com/hyperledger/fabric/core/config"
	"github.com/pkg/errors"
	viper "github.com/spf13/oldviper"
)

// Config is the struct that defines the Peer configurations.
type Config struct {
	// LocalMSPID is the identifier of the local MSP.
	LocalMSPID string
	// ListenAddress is the local address the peer will listen on. It must be
	// formatted as [host | ipaddr]:port.
	ListenAddress string
	// PeerID provides a name for this peer instance. It is used when naming
	// docker resources to segregate fabric networks and peers.
	PeerID string
	// PeerAddress is the address other peers and clients should use to
	// communicate with the peer. It must be formatted as [host | ipaddr]:port.
	// When used by the CLI, it represents the target peer endpoint.
	PeerAddress string
	// NetworkID specifies a name to use for logical separation of networks. It
	// is used when naming docker resources to segregate fabric networks and
	// peers.
	NetworkID string
	// ChaincodeListenAddress is the endpoint on which this peer will listen for
	// chaincode connections. If omitted, it defaults to the host portion of
	// PeerAddress and port 7052.
	ChaincodeListenAddress string
	// ChaincodeAddress specifies the endpoint chaincode launched by the peer
	// should use to connect to the peer. If omitted, it defaults to
	// ChaincodeListenAddress and falls back to ListenAddress.
	ChaincodeAddress string
	// ValidatorPoolSize indicates the number of goroutines that will execute
	// transaction validation in parallel. If omitted, it defaults to number of
	// hardware threads on the machine.
	ValidatorPoolSize int

	// ----- Profile -----
	// TODO: create separate sub-struct for Profile config.

	// ProfileEnabled determines if the go pprof endpoint is enabled in the peer.
	ProfileEnabled bool
	// ProfileListenAddress is the address the pprof server should accept
	// connections on.
	ProfileListenAddress string

	// ----- Discovery -----

	// The discovery service is used by clients to query information about peers,
	// such as - which peers have joined a certain channel, what is the latest
	// channel config, and most importantly - given a chaincode and a channel, what
	// possible sets of peers satisfy the endorsement policy.
	// TODO: create separate sub-struct for Discovery config.

	// DiscoveryEnabled is used to enable the discovery service.
	DiscoveryEnabled bool
	// DiscoveryOrgMembersAllowed allows non-admins to perform non channel-scoped queries.
	DiscoveryOrgMembersAllowed bool
	// DiscoveryAuthCacheEnabled is used to enable the authentication cache.
	DiscoveryAuthCacheEnabled bool
	// DiscoveryAuthCacheMaxSize sets the maximum size of authentication cache.
	DiscoveryAuthCacheMaxSize int
	// DiscoveryAuthCachePurgeRetentionRatio set the proportion of entries remains in cache
	// after overpopulation purge.
	DiscoveryAuthCachePurgeRetentionRatio float64

	// ----- Limits -----
	// Limits is used to configure some internal resource limits.
	// TODO: create separate sub-struct for Limits config.

	// LimitsConcurrencyQSCC sets the limits for number of concurrently running
	// qscc system chaincode requests.
	LimitsConcurrencyQSCC int

	// ----- TLS -----
	// Require server-side TLS.
	// TODO: create separate sub-struct for PeerTLS config.

	// PeerTLSEnabled enables/disables Peer TLS.
	PeerTLSEnabled bool

	// ----- Authentication -----
	// Authentication contains configuration parameters related to authenticating
	// client messages.
	// TODO: create separate sub-struct for Authentication config.

	// AuthenticationTimeWindow sets the acceptable time duration for current
	// server time and client's time as specified in a client request message.
	AuthenticationTimeWindow time.Duration

	// ----- AdminService -----
	// The admin service is used for adminstrative operations such as control over logger
	// levels, etc. Only peer administrators can use the service.
	// TODO: create separate sub-struct for AdminService config.

	// AdminListenAddress provides a interface and port for admin server to listen on.
	// Default to peer listen address.
	AdminListenAddress string

	// VMEndpoint sets the endpoint of the vm management systems.
	VMEndpoint string

	// ----- vm.docker.tls -----
	// TODO: create separate sub-struct for VM.Docker.TLS config.

	// VMDockerTLSEnabled enables/disables TLS for dockers.
	VMDockerTLSEnabled   bool
	VMDockerAttachStdout bool

	// ChaincodePull enables/disables force pulling of the base docker image.
	ChaincodePull bool

	// ----- Operations config -----
	// TODO: create separate sub-struct for Operations config.

	// OperationsListenAddress provides the host and port for the operations server
	OperationsListenAddress string
	// OperationsTLSEnabled enables/disables TLS for operations.
	OperationsTLSEnabled bool
	// OperationsTLSCertFile provides the path to PEM encoded server certificate for
	// the operations server.
	OperationsTLSCertFile string
	// OperationsTLSKeyFile provides the path to PEM encoded server key for the
	// operations server.
	OperationsTLSKeyFile string
	// OperationsTLSClientAuthRequired enables/disables the requirements for client
	// certificate authentication at the TLS layer to access all resource.
	OperationsTLSClientAuthRequired bool
	// OperationsTLSClientRootCAs provides the path to PEM encoded ca certiricates to
	// trust for client authentication.
	OperationsTLSClientRootCAs []string

	// ----- Metrics config -----
	// TODO: create separate sub-struct for Metrics config.

	// MetricsProvider provides the categories of metrics providers, which is one of
	// statsd, prometheus, or disabled.
	MetricsProvider string
	// StatsdNetwork indicate the network type used by statsd metrics. (tcp or udp).
	StatsdNetwork string
	// StatsdAaddress provides the address for statsd server.
	StatsdAaddress string
	// StatsdWriteInterval set the time interval at which locally cached counters and
	// gauges are pushed.
	StatsdWriteInterval time.Duration
	// StatsdPrefix provides the prefix that prepended to all emitted statsd metrics.
	StatsdPrefix string

	// ----- Docker config ------

	// DockerCert is the path to the PEM encoded TLS client certificate required to access
	// the docker daemon.
	DockerCert string
	// DockerKey is the path to the PEM encoded key required to access the docker daemon.
	DockerKey string
	// DockerCA is the path to the PEM encoded CA certificate for the docker daemon.
	DockerCA string
}

// GlobalConfig obtains a set of configuration from viper, build and returns
// the config struct.
func GlobalConfig() (*Config, error) {
	c := &Config{}
	if err := c.load(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) load() error {
	preeAddress, err := getLocalAddress()
	if err != nil {
		return err
	}
	c.PeerAddress = preeAddress
	c.PeerID = viper.GetString("peer.id")
	c.LocalMSPID = viper.GetString("peer.localMspId")
	c.ListenAddress = viper.GetString("peer.listenAddress")

	c.AuthenticationTimeWindow = viper.GetDuration("peer.authentication.timewindow")
	if c.AuthenticationTimeWindow == 0 {
		defaultTimeWindow := 15 * time.Minute
		logger.Warningf("`peer.authentication.timewindow` not set; defaulting to %s", defaultTimeWindow)
		c.AuthenticationTimeWindow = defaultTimeWindow
	}

	c.PeerTLSEnabled = viper.GetBool("peer.tls.enabled")
	c.NetworkID = viper.GetString("peer.networkId")
	c.LimitsConcurrencyQSCC = viper.GetInt("peer.limits.concurrency.qscc")
	c.DiscoveryEnabled = viper.GetBool("peer.discovery.enabled")
	c.ProfileEnabled = viper.GetBool("peer.profile.enabled")
	c.ProfileListenAddress = viper.GetString("peer.profile.listenAddress")
	c.DiscoveryOrgMembersAllowed = viper.GetBool("peer.discovery.orgMembersAllowedAccess")
	c.DiscoveryAuthCacheEnabled = viper.GetBool("peer.discovery.authCacheEnabled")
	c.DiscoveryAuthCacheMaxSize = viper.GetInt("peer.discovery.authCacheMaxSize")
	c.DiscoveryAuthCachePurgeRetentionRatio = viper.GetFloat64("peer.discovery.authCachePurgeRetentionRatio")
	c.ChaincodeListenAddress = viper.GetString("peer.chaincodeListenAddress")
	c.ChaincodeAddress = viper.GetString("peer.chaincodeAddress")
	c.AdminListenAddress = viper.GetString("peer.adminService.listenAddress")

	c.ValidatorPoolSize = viper.GetInt("peer.validatorPoolSize")
	if c.ValidatorPoolSize <= 0 {
		c.ValidatorPoolSize = runtime.NumCPU()
	}

	c.VMEndpoint = viper.GetString("vm.endpoint")
	c.VMDockerTLSEnabled = viper.GetBool("vm.docker.tls.enabled")
	c.VMDockerAttachStdout = viper.GetBool("vm.docker.attachStdout")

	c.ChaincodePull = viper.GetBool("chaincode.pull")

	c.OperationsListenAddress = viper.GetString("operations.listenAddress")
	c.OperationsTLSEnabled = viper.GetBool("operations.tls.enabled")
	c.OperationsTLSCertFile = viper.GetString("operations.tls.cert.file")
	c.OperationsTLSKeyFile = viper.GetString("operations.tls.key.file")
	c.OperationsTLSClientAuthRequired = viper.GetBool("operations.tls.clientAuthRequired")
	c.OperationsTLSClientRootCAs = viper.GetStringSlice("operations.tls.clientRootCAs.files")

	c.MetricsProvider = viper.GetString("metrics.provider")
	c.StatsdNetwork = viper.GetString("metrics.statsd.network")
	c.StatsdAaddress = viper.GetString("metrics.statsd.address")
	c.StatsdWriteInterval = viper.GetDuration("metrics.statsd.writeInterval")
	c.StatsdPrefix = viper.GetString("metrics.statsd.prefix")

	c.DockerCert = config.GetPath("vm.docker.tls.cert.file")
	c.DockerKey = config.GetPath("vm.docker.tls.key.file")
	c.DockerCA = config.GetPath("vm.docker.tls.ca.file")

	return nil
}

// getLocalAddress returns the address:port the local peer is operating on.  Affected by env:peer.addressAutoDetect
func getLocalAddress() (string, error) {
	peerAddress := viper.GetString("peer.address")
	if peerAddress == "" {
		return "", fmt.Errorf("peer.address isn't set")
	}
	host, port, err := net.SplitHostPort(peerAddress)
	if err != nil {
		return "", errors.Errorf("peer.address isn't in host:port format: %s", peerAddress)
	}

	localIP, err := GetLocalIP()
	if err != nil {
		peerLogger.Errorf("Local ip address not auto-detectable: %s", err)
		return "", err
	}
	autoDetectedIPAndPort := net.JoinHostPort(localIP, port)
	peerLogger.Info("Auto-detected peer address:", autoDetectedIPAndPort)
	// If host is the IPv4 address "0.0.0.0" or the IPv6 address "::",
	// then fallback to auto-detected address
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		peerLogger.Info("Host is", host, ", falling back to auto-detected address:", autoDetectedIPAndPort)
		return autoDetectedIPAndPort, nil
	}

	if viper.GetBool("peer.addressAutoDetect") {
		peerLogger.Info("Auto-detect flag is set, returning", autoDetectedIPAndPort)
		return autoDetectedIPAndPort, nil
	}
	peerLogger.Info("Returning", peerAddress)
	return peerAddress, nil

}

// GetServerConfig returns the gRPC server configuration for the peer
func GetServerConfig() (comm.ServerConfig, error) {
	secureOptions := &comm.SecureOptions{
		UseTLS: viper.GetBool("peer.tls.enabled"),
	}
	serverConfig := comm.ServerConfig{SecOpts: secureOptions}
	if secureOptions.UseTLS {
		// get the certs from the file system
		serverKey, err := ioutil.ReadFile(config.GetPath("peer.tls.key.file"))
		if err != nil {
			return serverConfig, fmt.Errorf("error loading TLS key (%s)", err)
		}
		serverCert, err := ioutil.ReadFile(config.GetPath("peer.tls.cert.file"))
		if err != nil {
			return serverConfig, fmt.Errorf("error loading TLS certificate (%s)", err)
		}
		secureOptions.Certificate = serverCert
		secureOptions.Key = serverKey
		secureOptions.RequireClientCert = viper.GetBool("peer.tls.clientAuthRequired")
		if secureOptions.RequireClientCert {
			var clientRoots [][]byte
			for _, file := range viper.GetStringSlice("peer.tls.clientRootCAs.files") {
				clientRoot, err := ioutil.ReadFile(
					config.TranslatePath(filepath.Dir(viper.ConfigFileUsed()), file))
				if err != nil {
					return serverConfig,
						fmt.Errorf("error loading client root CAs (%s)", err)
				}
				clientRoots = append(clientRoots, clientRoot)
			}
			secureOptions.ClientRootCAs = clientRoots
		}
		// check for root cert
		if config.GetPath("peer.tls.rootcert.file") != "" {
			rootCert, err := ioutil.ReadFile(config.GetPath("peer.tls.rootcert.file"))
			if err != nil {
				return serverConfig, fmt.Errorf("error loading TLS root certificate (%s)", err)
			}
			secureOptions.ServerRootCAs = [][]byte{rootCert}
		}
	}
	// get the default keepalive options
	serverConfig.KaOpts = comm.DefaultKeepaliveOptions
	// check to see if interval is set for the env
	if viper.IsSet("peer.keepalive.interval") {
		serverConfig.KaOpts.ServerInterval = viper.GetDuration("peer.keepalive.interval")
	}
	// check to see if timeout is set for the env
	if viper.IsSet("peer.keepalive.timeout") {
		serverConfig.KaOpts.ServerTimeout = viper.GetDuration("peer.keepalive.timeout")
	}
	// check to see if minInterval is set for the env
	if viper.IsSet("peer.keepalive.minInterval") {
		serverConfig.KaOpts.ServerMinInterval = viper.GetDuration("peer.keepalive.minInterval")
	}
	return serverConfig, nil
}

// GetClientCertificate returns the TLS certificate to use for gRPC client
// connections
func GetClientCertificate() (tls.Certificate, error) {
	cert := tls.Certificate{}

	keyPath := viper.GetString("peer.tls.clientKey.file")
	certPath := viper.GetString("peer.tls.clientCert.file")

	if keyPath != "" || certPath != "" {
		// need both keyPath and certPath to be set
		if keyPath == "" || certPath == "" {
			return cert, errors.New("peer.tls.clientKey.file and " +
				"peer.tls.clientCert.file must both be set or must both be empty")
		}
		keyPath = config.GetPath("peer.tls.clientKey.file")
		certPath = config.GetPath("peer.tls.clientCert.file")

	} else {
		// use the TLS server keypair
		keyPath = viper.GetString("peer.tls.key.file")
		certPath = viper.GetString("peer.tls.cert.file")

		if keyPath != "" || certPath != "" {
			// need both keyPath and certPath to be set
			if keyPath == "" || certPath == "" {
				return cert, errors.New("peer.tls.key.file and " +
					"peer.tls.cert.file must both be set or must both be empty")
			}
			keyPath = config.GetPath("peer.tls.key.file")
			certPath = config.GetPath("peer.tls.cert.file")
		} else {
			return cert, errors.New("must set either " +
				"[peer.tls.key.file and peer.tls.cert.file] or " +
				"[peer.tls.clientKey.file and peer.tls.clientCert.file]" +
				"when peer.tls.clientAuthEnabled is set to true")
		}
	}
	// get the keypair from the file system
	clientKey, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return cert, errors.WithMessage(err,
			"error loading client TLS key")
	}
	clientCert, err := ioutil.ReadFile(certPath)
	if err != nil {
		return cert, errors.WithMessage(err,
			"error loading client TLS certificate")
	}
	cert, err = tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		return cert, errors.WithMessage(err,
			"error parsing client TLS key pair")
	}
	return cert, nil
}
