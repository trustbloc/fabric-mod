/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	cb "github.com/hyperledger/fabric-protos-go/common"
	discprotos "github.com/hyperledger/fabric-protos-go/discovery"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/bccsp/factory"
	"github.com/hyperledger/fabric/common/cauthdsl"
	ccdef "github.com/hyperledger/fabric/common/chaincode"
	"github.com/hyperledger/fabric/common/crypto"
	"github.com/hyperledger/fabric/common/crypto/tlsgen"
	"github.com/hyperledger/fabric/common/deliver"
	"github.com/hyperledger/fabric/common/flogging"
	floggingmetrics "github.com/hyperledger/fabric/common/flogging/metrics"
	"github.com/hyperledger/fabric/common/grpclogging"
	"github.com/hyperledger/fabric/common/grpcmetrics"
	"github.com/hyperledger/fabric/common/metadata"
	"github.com/hyperledger/fabric/common/metrics"
	"github.com/hyperledger/fabric/common/policies"
	"github.com/hyperledger/fabric/core/aclmgmt"
	"github.com/hyperledger/fabric/core/cclifecycle"
	"github.com/hyperledger/fabric/core/chaincode"
	"github.com/hyperledger/fabric/core/chaincode/accesscontrol"
	"github.com/hyperledger/fabric/core/chaincode/lifecycle"
	"github.com/hyperledger/fabric/core/chaincode/persistence"
	"github.com/hyperledger/fabric/core/chaincode/platforms"
	"github.com/hyperledger/fabric/core/comm"
	"github.com/hyperledger/fabric/core/committer/txvalidator/plugin"
	"github.com/hyperledger/fabric/core/common/ccprovider"
	"github.com/hyperledger/fabric/core/common/privdata"
	coreconfig "github.com/hyperledger/fabric/core/config"
	"github.com/hyperledger/fabric/core/container"
	"github.com/hyperledger/fabric/core/container/dockercontroller"
	"github.com/hyperledger/fabric/core/container/externalbuilder"
	"github.com/hyperledger/fabric/core/deliverservice"
	"github.com/hyperledger/fabric/core/dispatcher"
	"github.com/hyperledger/fabric/core/endorser"
	authHandler "github.com/hyperledger/fabric/core/handlers/auth"
	endorsement2 "github.com/hyperledger/fabric/core/handlers/endorsement/api"
	endorsement3 "github.com/hyperledger/fabric/core/handlers/endorsement/api/identities"
	"github.com/hyperledger/fabric/core/handlers/library"
	validation "github.com/hyperledger/fabric/core/handlers/validation/api"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/cceventmgmt"
	"github.com/hyperledger/fabric/core/ledger/kvledger"
	"github.com/hyperledger/fabric/core/ledger/ledgermgmt"
	"github.com/hyperledger/fabric/core/operations"
	"github.com/hyperledger/fabric/core/peer"
	"github.com/hyperledger/fabric/core/policy"
	"github.com/hyperledger/fabric/core/scc"
	"github.com/hyperledger/fabric/core/scc/lscc"
	"github.com/hyperledger/fabric/core/scc/qscc"
	"github.com/hyperledger/fabric/discovery"
	"github.com/hyperledger/fabric/discovery/endorsement"
	discsupport "github.com/hyperledger/fabric/discovery/support"
	discacl "github.com/hyperledger/fabric/discovery/support/acl"
	ccsupport "github.com/hyperledger/fabric/discovery/support/chaincode"
	"github.com/hyperledger/fabric/discovery/support/config"
	"github.com/hyperledger/fabric/discovery/support/gossip"
	extcc "github.com/hyperledger/fabric/extensions/chaincode"
	collretriever "github.com/hyperledger/fabric/extensions/collections/retriever"
	extcscc "github.com/hyperledger/fabric/extensions/cscc"
	"github.com/hyperledger/fabric/extensions/resource"
	transientstoreext "github.com/hyperledger/fabric/extensions/storage/transientstore"
	gossipcommon "github.com/hyperledger/fabric/gossip/common"
	gossipgossip "github.com/hyperledger/fabric/gossip/gossip"
	gossipmetrics "github.com/hyperledger/fabric/gossip/metrics"
	"github.com/hyperledger/fabric/gossip/service"
	gossipservice "github.com/hyperledger/fabric/gossip/service"
	peergossip "github.com/hyperledger/fabric/internal/peer/gossip"
	"github.com/hyperledger/fabric/internal/peer/version"
	"github.com/hyperledger/fabric/msp"
	"github.com/hyperledger/fabric/msp/mgmt"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

const (
	chaincodeAddrKey       = "peer.chaincodeAddress"
	chaincodeListenAddrKey = "peer.chaincodeListenAddress"
	defaultChaincodePort   = 7052
)

var chaincodeDevMode bool

func startCmd() *cobra.Command {
	// Set the flags on the node start command.
	flags := nodeStartCmd.Flags()
	flags.BoolVarP(&chaincodeDevMode, "peer-chaincodedev", "", false, "start peer in chaincode development mode")
	return nodeStartCmd
}

var nodeStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the node.",
	Long:  `Starts a node that interacts with the network.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 {
			return fmt.Errorf("trailing args detected")
		}
		// Parsing of the command line is done so silence cmd usage
		cmd.SilenceUsage = true
		return serve(args)
	},
}

// externalVMAdapter adapts coerces the result of Build to the
// container.Interface type expected by the VM interface.
type externalVMAdapter struct {
	detector *externalbuilder.Detector
}

func (e externalVMAdapter) Build(
	ccid string,
	metadata *persistence.ChaincodePackageMetadata,
	codePackage io.Reader,
) (container.Instance, error) {
	i, err := e.detector.Build(ccid, metadata, codePackage)
	if err != nil {
		return nil, err
	}

	// ensure <nil> is returned instead of (*externalbuilder.Instance)(nil)
	if i == nil {
		return nil, nil
	}
	return i, err
}

type endorserChannelAdapter struct {
	peer *peer.Peer
}

func (e endorserChannelAdapter) Channel(channelID string) *endorser.Channel {
	if peerChannel := e.peer.Channel(channelID); peerChannel != nil {
		return &endorser.Channel{
			IdentityDeserializer: peerChannel.MSPManager(),
		}
	}

	return nil
}

func serve(args []string) error {
	// currently the peer only works with the standard MSP
	// because in certain scenarios the MSP has to make sure
	// that from a single credential you only have a single 'identity'.
	// Idemix does not support this *YET* but it can be easily
	// fixed to support it. For now, we just make sure that
	// the peer only comes up with the standard MSP
	mspType := mgmt.GetLocalMSP(factory.GetDefault()).GetType()
	if mspType != msp.FABRIC {
		panic("Unsupported msp type " + msp.ProviderTypeToString(mspType))
	}

	// Trace RPCs with the golang.org/x/net/trace package. This was moved out of
	// the deliver service connection factory as it has process wide implications
	// and was racy with respect to initialization of gRPC clients and servers.
	grpc.EnableTracing = true

	logger.Infof("Starting %s", version.GetInfo())

	//obtain coreConfiguration
	coreConfig, err := peer.GlobalConfig()
	if err != nil {
		return err
	}

	platformRegistry := platforms.NewRegistry(platforms.SupportedPlatforms...)

	identityDeserializerFactory := func(chainID string) msp.IdentityDeserializer {
		return mgmt.GetManagerForChain(chainID)
	}

	opsSystem := newOperationsSystem(coreConfig)
	err = opsSystem.Start()
	if err != nil {
		return errors.WithMessage(err, "failed to initialize operations subsystems")
	}
	defer opsSystem.Stop()

	metricsProvider := opsSystem.Provider
	logObserver := floggingmetrics.NewObserver(metricsProvider)
	flogging.SetObserver(logObserver)

	membershipInfoProvider := privdata.NewMembershipInfoProvider(createSelfSignedData(), identityDeserializerFactory)

	mspID := coreConfig.LocalMSPID

	chaincodeInstallPath := filepath.Join(coreconfig.GetPath("peer.fileSystemPath"), "lifecycle", "chaincodes")
	ccStore := persistence.NewStore(chaincodeInstallPath)
	ccPackageParser := &persistence.ChaincodePackageParser{
		MetadataProvider: ccprovider.PersistenceAdapter(ccprovider.MetadataAsTarEntries),
	}

	peerHost, _, err := net.SplitHostPort(coreConfig.PeerAddress)
	if err != nil {
		return fmt.Errorf("peer address is not in the format of host:port: %v", err)
	}

	listenAddr := coreConfig.ListenAddress
	serverConfig, err := peer.GetServerConfig()
	if err != nil {
		logger.Fatalf("Error loading secure config for peer (%s)", err)
	}

	serverConfig.Logger = flogging.MustGetLogger("core.comm").With("server", "PeerServer")
	serverConfig.ServerStatsHandler = comm.NewServerStatsHandler(metricsProvider)
	serverConfig.UnaryInterceptors = append(
		serverConfig.UnaryInterceptors,
		grpcmetrics.UnaryServerInterceptor(grpcmetrics.NewUnaryMetrics(metricsProvider)),
		grpclogging.UnaryServerInterceptor(flogging.MustGetLogger("comm.grpc.server").Zap()),
	)
	serverConfig.StreamInterceptors = append(
		serverConfig.StreamInterceptors,
		grpcmetrics.StreamServerInterceptor(grpcmetrics.NewStreamMetrics(metricsProvider)),
		grpclogging.StreamServerInterceptor(flogging.MustGetLogger("comm.grpc.server").Zap()),
	)

	cs := comm.NewCredentialSupport()
	if serverConfig.SecOpts.UseTLS {
		logger.Info("Starting peer with TLS enabled")
		cs = comm.NewCredentialSupport(serverConfig.SecOpts.ServerRootCAs...)

		// set the cert to use if client auth is requested by remote endpoints
		clientCert, err := peer.GetClientCertificate()
		if err != nil {
			logger.Fatalf("Failed to set TLS client certificate (%s)", err)
		}
		cs.SetClientCertificate(clientCert)
	}

	peerServer, err := comm.NewGRPCServer(listenAddr, serverConfig)
	if err != nil {
		logger.Fatalf("Failed to create peer server (%s)", err)
	}

	transientStoreProvider, err := transientstoreext.NewStoreProvider(
		filepath.Join(coreconfig.GetPath("peer.fileSystemPath"), "transientstore"),
	)
	if err != nil {
		return errors.WithMessage(err, "failed to open transient store")
	}

	deliverServiceConfig := deliverservice.GlobalConfig()

	peerInstance := &peer.Peer{
		Server:                   peerServer,
		ServerConfig:             serverConfig,
		CredentialSupport:        cs,
		StoreProvider:            transientStoreProvider,
		CryptoProvider:           factory.GetDefault(),
		OrdererEndpointOverrides: deliverServiceConfig.OrdererEndpointOverrides,
	}

	localMSP := mgmt.GetLocalMSP(factory.GetDefault())
	signingIdentity, err := localMSP.GetDefaultSigningIdentity()
	if err != nil {
		logger.Panicf("Could not get the default signing identity from the local MSP: [%+v]", err)
	}

	signingIdentityBytes, err := signingIdentity.Serialize()
	if err != nil {
		logger.Panicf("Failed to serialize the signing identity: %v", err)
	}

	expirationLogger := flogging.MustGetLogger("certmonitor")
	crypto.TrackExpiration(
		serverConfig.SecOpts.UseTLS,
		serverConfig.SecOpts.Certificate,
		cs.GetClientCertificate().Certificate,
		signingIdentityBytes,
		expirationLogger.Warnf, // This can be used to piggyback a metric event in the future
		time.Now(),
		time.AfterFunc)

	policyMgr := policies.PolicyManagerGetterFunc(peerInstance.GetPolicyManager)

	deliverGRPCClient, err := comm.NewGRPCClient(comm.ClientConfig{
		Timeout: deliverServiceConfig.ConnectionTimeout,
		KaOpts:  deliverServiceConfig.KeepaliveOptions,
		SecOpts: deliverServiceConfig.SecOpts,
	})
	if err != nil {
		logger.Panicf("Could not create the deliver grpc client: [%+v]", err)
	}

	// FIXME: Creating the gossip service has the side effect of starting a bunch
	// of go routines and registration with the grpc server.
	gossipService, err := initGossipService(
		policyMgr,
		metricsProvider,
		peerServer,
		signingIdentity,
		cs,
		coreConfig.PeerAddress,
		deliverGRPCClient,
		deliverServiceConfig,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to initialize gossip service")
	}
	defer gossipService.Stop()

	peerInstance.GossipService = gossipService

	policyChecker := policy.NewPolicyChecker(
		policies.PolicyManagerGetterFunc(peerInstance.GetPolicyManager),
		mgmt.GetLocalMSP(factory.GetDefault()),
		mgmt.NewLocalMSPPrincipalGetter(factory.GetDefault()),
	)

	//startup aclmgmt with default ACL providers (resource based and default 1.0 policies based).
	//Users can pass in their own ACLProvider to RegisterACLProvider (currently unit tests do this)
	aclProvider := aclmgmt.NewACLProvider(
		aclmgmt.ResourceGetter(peerInstance.GetStableChannelConfig),
		policyChecker,
	)

	// TODO, unfortunately, the lifecycle initialization is very unclean at the
	// moment. This is because ccprovider.SetChaincodePath only works after
	// ledgermgmt.Initialize, but ledgermgmt.Initialize requires a reference to
	// lifecycle.  Finally, lscc requires a reference to the system chaincode
	// provider in order to be created, which requires chaincode support to be
	// up, which also requires, you guessed it, lifecycle. Once we remove the
	// v1.0 lifecycle, we should be good to collapse all of the init of lifecycle
	// to this point.
	lifecycleResources := &lifecycle.Resources{
		Serializer:          &lifecycle.Serializer{},
		ChannelConfigSource: peerInstance,
		ChaincodeStore:      ccStore,
		PackageParser:       ccPackageParser,
	}

	lifecycleValidatorCommitter := &lifecycle.ValidatorCommitter{
		Resources:                    lifecycleResources,
		LegacyDeployedCCInfoProvider: &lscc.DeployedCCInfoProvider{},
	}

	ccInfoFSImpl := &ccprovider.CCInfoFSImpl{GetHasher: factory.GetDefault()}

	// Configure CC package storage
	lsccInstallPath := filepath.Join(coreconfig.GetPath("peer.fileSystemPath"), "chaincodes")
	ccprovider.SetChaincodesPath(lsccInstallPath)

	// legacyMetadataManager collects metadata information from the legacy
	// lifecycle (lscc). This is expected to disappear with FAB-15061.
	legacyMetadataManager, err := cclifecycle.NewMetadataManager(
		cclifecycle.EnumerateFunc(
			func() ([]ccdef.InstalledChaincode, error) {
				return ccInfoFSImpl.ListInstalledChaincodes(ccInfoFSImpl.GetChaincodeInstallPath(), ioutil.ReadDir, ccprovider.LoadPackage)
			},
		),
	)
	if err != nil {
		logger.Panicf("Failed creating LegacyMetadataManager: +%v", err)
	}

	// metadataManager aggregates metadata information from _lifecycle and
	// the legacy lifecycle (lscc).
	metadataManager := lifecycle.NewMetadataManager()

	// the purpose of these two managers is to feed per-channel chaincode data
	// into gossip owing to the fact that we are transitioning from lscc to
	// _lifecycle, we still have two providers of such information until v2.1,
	// in which we will remove the legacy.
	//
	// the flow of information is the following
	//
	// gossip <-- metadataManager <-- lifecycleCache  (for _lifecycle)
	//                             \
	//                              - legacyMetadataManager (for lscc)
	//
	// FAB-15061 tracks the work necessary to remove LSCC, at which point we
	// will be able to simplify the flow to simply be
	//
	// gossip <-- lifecycleCache

	chaincodeCustodian := lifecycle.NewChaincodeCustodian()

	externalBuilderOutput := filepath.Join(coreconfig.GetPath("peer.fileSystemPath"), "externalbuilder", "builds")
	if err := os.MkdirAll(externalBuilderOutput, 0700); err != nil {
		logger.Panicf("could not create externalbuilder build output dir: %s", err)
	}

	ebMetadataProvider := &externalbuilder.MetadataProvider{
		DurablePath: externalBuilderOutput,
	}

	lifecycleCache := lifecycle.NewCache(lifecycleResources, mspID, metadataManager, chaincodeCustodian, ebMetadataProvider)

	txProcessors := map[common.HeaderType]ledger.CustomTxProcessor{
		common.HeaderType_CONFIG: &peer.ConfigTxProcessor{},
	}

	collDataProvider := collretriever.NewProvider()

	ledgerConfig := ledgerConfig()

	peerInstance.LedgerMgr = ledgermgmt.NewLedgerMgr(
		&ledgermgmt.Initializer{
			CustomTxProcessors:              txProcessors,
			DeployedChaincodeInfoProvider:   lifecycleValidatorCommitter,
			MembershipInfoProvider:          membershipInfoProvider,
			ChaincodeLifecycleEventProvider: lifecycleCache,
			MetricsProvider:                 metricsProvider,
			HealthCheckRegistry:             opsSystem,
			StateListeners:                  []ledger.StateListener{lifecycleCache},
			Config:                          ledgerConfig,
			Hasher:                          factory.GetDefault(),
			EbMetadataProvider:              ebMetadataProvider,
			CollDataProvider:                collDataProvider,
		},
	)

	if err := lifecycleCache.InitializeLocalChaincodes(); err != nil {
		return errors.WithMessage(err, "could not initialize local chaincodes")
	}

	// Parameter overrides must be processed before any parameters are
	// cached. Failures to cache cause the server to terminate immediately.
	if chaincodeDevMode {
		logger.Info("Running in chaincode development mode")
		logger.Info("Disable loading validity system chaincode")

		viper.Set("chaincode.mode", chaincode.DevModeUserRunsChaincode)
	}

	mutualTLS := serverConfig.SecOpts.UseTLS && serverConfig.SecOpts.RequireClientCert
	policyCheckerProvider := func(resourceName string) deliver.PolicyCheckerFunc {
		return func(env *cb.Envelope, channelID string) error {
			return aclProvider.CheckACL(resourceName, channelID, env)
		}
	}

	metrics := deliver.NewMetrics(metricsProvider)
	abServer := &peer.DeliverServer{
		DeliverHandler: deliver.NewHandler(
			&peer.DeliverChainManager{Peer: peerInstance},
			coreConfig.AuthenticationTimeWindow,
			mutualTLS,
			metrics,
			false,
		),
		PolicyCheckerProvider: policyCheckerProvider,
	}
	pb.RegisterDeliverServer(peerServer.Server(), abServer)

	// Create a self-signed CA for chaincode service
	ca, err := tlsgen.NewCA()
	if err != nil {
		logger.Panic("Failed creating authentication layer:", err)
	}
	ccSrv, ccEndpoint, err := createChaincodeServer(coreConfig, ca, peerHost)
	if err != nil {
		logger.Panicf("Failed to create chaincode server: %s", err)
	}

	//get user mode
	userRunsCC := chaincode.IsDevMode()
	tlsEnabled := coreConfig.PeerTLSEnabled

	// create chaincode specific tls CA
	authenticator := accesscontrol.NewAuthenticator(ca)

	chaincodeHandlerRegistry := chaincode.NewHandlerRegistry(userRunsCC)
	lifecycleTxQueryExecutorGetter := &chaincode.TxQueryExecutorGetter{
		CCID:            scc.ChaincodeID(lifecycle.LifecycleNamespace),
		HandlerRegistry: chaincodeHandlerRegistry,
	}
	var client *docker.Client
	if coreConfig.VMDockerTLSEnabled {
		client, err = docker.NewTLSClient(coreConfig.VMEndpoint, coreConfig.DockerCert, coreConfig.DockerKey, coreConfig.DockerCA)
	} else {
		client, err = docker.NewClient(coreConfig.VMEndpoint)
	}
	if err != nil {
		logger.Panicf("cannot create docker client: %s", err)
	}

	chaincodeConfig := chaincode.GlobalConfig()

	dockerVM := &dockercontroller.DockerVM{
		PeerID:        coreConfig.PeerID,
		NetworkID:     coreConfig.NetworkID,
		BuildMetrics:  dockercontroller.NewBuildMetrics(opsSystem.Provider),
		Client:        client,
		AttachStdOut:  coreConfig.VMDockerAttachStdout,
		HostConfig:    getDockerHostConfig(),
		ChaincodePull: coreConfig.ChaincodePull,
		NetworkMode:   coreConfig.VMNetworkMode,
		PlatformBuilder: &platforms.Builder{
			Registry: platformRegistry,
			Client:   client,
		},
		// This field is superfluous for chaincodes built with v2.0+ binaries
		// however, to prevent users from being forced to rebuild leaving for now
		// but it should be removed in the future.
		LoggingEnv: []string{
			"CORE_CHAINCODE_LOGGING_LEVEL=" + chaincodeConfig.LogLevel,
			"CORE_CHAINCODE_LOGGING_SHIM=" + chaincodeConfig.ShimLogLevel,
			"CORE_CHAINCODE_LOGGING_FORMAT=" + chaincodeConfig.LogFormat,
		},
	}
	if err := opsSystem.RegisterChecker("docker", dockerVM); err != nil {
		logger.Panicf("failed to register docker health check: %s", err)
	}

	externalVM := &externalbuilder.Detector{
		Builders:    externalbuilder.CreateBuilders(coreConfig.ExternalBuilders),
		DurablePath: externalBuilderOutput,
	}

	buildRegistry := &container.BuildRegistry{}

	containerRouter := &container.Router{
		DockerVM:   dockerVM,
		ExternalVM: externalVMAdapter{externalVM},
		PackageProvider: &persistence.FallbackPackageLocator{
			ChaincodePackageLocator: &persistence.ChaincodePackageLocator{
				ChaincodeDir: chaincodeInstallPath,
			},
			LegacyCCPackageLocator: &ccprovider.CCInfoFSImpl{GetHasher: factory.GetDefault()},
		},
	}

	builtinSCCs := map[string]struct{}{
		"lscc":       {},
		"qscc":       {},
		"cscc":       {},
		"_lifecycle": {},
	}

	lsccInst := &lscc.SCC{
		BuiltinSCCs: builtinSCCs,
		Support: &lscc.SupportImpl{
			GetMSPIDs: peerInstance.GetMSPIDs,
		},
		SCCProvider:        &lscc.PeerShim{Peer: peerInstance},
		ACLProvider:        aclProvider,
		GetMSPIDs:          peerInstance.GetMSPIDs,
		PolicyChecker:      policyChecker,
		BCCSP:              factory.GetDefault(),
		BuildRegistry:      buildRegistry,
		ChaincodeBuilder:   containerRouter,
		EbMetadataProvider: ebMetadataProvider,
	}

	chaincodeEndorsementInfo := &lifecycle.ChaincodeEndorsementInfoSource{
		LegacyImpl:  lsccInst,
		Resources:   lifecycleResources,
		Cache:       lifecycleCache,
		BuiltinSCCs: builtinSCCs,
	}

	containerRuntime := &chaincode.ContainerRuntime{
		BuildRegistry:   buildRegistry,
		ContainerRouter: containerRouter,
	}

	lifecycleFunctions := &lifecycle.ExternalFunctions{
		Resources:                 lifecycleResources,
		InstallListener:           lifecycleCache,
		InstalledChaincodesLister: lifecycleCache,
		ChaincodeBuilder:          containerRouter,
		BuildRegistry:             buildRegistry,
	}

	lifecycleSCC := &lifecycle.SCC{
		Dispatcher: &dispatcher.Dispatcher{
			Protobuf: &dispatcher.ProtobufImpl{},
		},
		DeployedCCInfoProvider: lifecycleValidatorCommitter,
		QueryExecutorProvider:  lifecycleTxQueryExecutorGetter,
		Functions:              lifecycleFunctions,
		OrgMSPID:               mspID,
		ChannelConfigSource:    peerInstance,
		ACLProvider:            aclProvider,
	}

	chaincodeLauncher := &chaincode.RuntimeLauncher{
		Metrics:        chaincode.NewLaunchMetrics(opsSystem.Provider),
		Registry:       chaincodeHandlerRegistry,
		Runtime:        containerRuntime,
		StartupTimeout: chaincodeConfig.StartupTimeout,
		CertGenerator:  authenticator,
		CACert:         ca.CertBytes(),
		PeerAddress:    ccEndpoint,
	}

	// Keep TestQueries working
	if !chaincodeConfig.TLSEnabled {
		chaincodeLauncher.CertGenerator = nil
	}

	go chaincodeCustodian.Work(buildRegistry, containerRouter, chaincodeLauncher)

	chaincodeSupport := &chaincode.ChaincodeSupport{
		ACLProvider:            aclProvider,
		AppConfig:              peerInstance,
		DeployedCCInfoProvider: lifecycleValidatorCommitter,
		ExecuteTimeout:         chaincodeConfig.ExecuteTimeout,
		InstallTimeout:         chaincodeConfig.InstallTimeout,
		HandlerRegistry:        chaincodeHandlerRegistry,
		HandlerMetrics:         chaincode.NewHandlerMetrics(opsSystem.Provider),
		Keepalive:              chaincodeConfig.Keepalive,
		Launcher:               chaincodeLauncher,
		Lifecycle:              chaincodeEndorsementInfo,
		Peer:                   peerInstance,
		Runtime:                containerRuntime,
		BuiltinSCCs:            builtinSCCs,
		TotalQueryLimit:        chaincodeConfig.TotalQueryLimit,
		UserRunsCC:             userRunsCC,
	}

	ccSupSrv := pb.ChaincodeSupportServer(chaincodeSupport)
	if tlsEnabled {
		ccSupSrv = authenticator.Wrap(ccSupSrv)
	}

	csccInst := extcscc.New(
		aclProvider,
		lifecycleValidatorCommitter,
		lsccInst,
		lifecycleValidatorCommitter,
		policyChecker,
		peerInstance,
		factory.GetDefault(),
	)
	qsccInst := scc.SelfDescribingSysCC(qscc.New(aclProvider, peerInstance))
	if maxConcurrency := coreConfig.LimitsConcurrencyQSCC; maxConcurrency != 0 {
		qsccInst = scc.Throttle(maxConcurrency, qsccInst)
	}

	pb.RegisterChaincodeSupportServer(ccSrv.Server(), ccSupSrv)

	// start the chaincode specific gRPC listening service
	go ccSrv.Start()

	logger.Debugf("Running peer")

	libConf, err := library.LoadConfig()
	if err != nil {
		return errors.WithMessage(err, "could not decode peer handlers configuration")
	}

	reg := library.InitRegistry(libConf)

	authFilters := reg.Lookup(library.Auth).([]authHandler.Filter)
	endorserSupport := &endorser.SupportImpl{
		SignerSerializer: signingIdentity,
		Peer:             peerInstance,
		ChaincodeSupport: chaincodeSupport,
		ACLProvider:      aclProvider,
		BuiltinSCCs:      builtinSCCs,
	}
	endorsementPluginsByName := reg.Lookup(library.Endorsement).(map[string]endorsement2.PluginFactory)
	validationPluginsByName := reg.Lookup(library.Validation).(map[string]validation.PluginFactory)
	signingIdentityFetcher := (endorsement3.SigningIdentityFetcher)(endorserSupport)
	channelStateRetriever := endorser.ChannelStateRetriever(endorserSupport)
	pluginMapper := endorser.MapBasedPluginMapper(endorsementPluginsByName)
	pluginEndorser := endorser.NewPluginEndorser(&endorser.PluginSupport{
		ChannelStateRetriever:   channelStateRetriever,
		TransientStoreRetriever: peerInstance,
		PluginMapper:            pluginMapper,
		SigningIdentityFetcher:  signingIdentityFetcher,
	})
	endorserSupport.PluginEndorser = pluginEndorser
	channelFetcher := endorserChannelAdapter{
		peer: peerInstance,
	}
	serverEndorser := &endorser.Endorser{
		PrivateDataDistributor: gossipService,
		ChannelFetcher:         channelFetcher,
		LocalMSP:               localMSP,
		Support:                endorserSupport,
		Metrics:                endorser.NewMetrics(metricsProvider),
	}

	// Initialize all of the registered resources
	err = resource.Initialize(
		newGossipProvider(peerInstance.GossipService),
		peerInstance,
		newMSPProvider(),
		newLedgerConfigProvider(ledgerConfig),
		newCCEventMgrProvider(),
		pluginMapper,
	)
	if err != nil {
		panic(err)
	}
	defer resource.Close()

	// get the list of system chain codes provided by extensions
	extscc := extcc.CreateSCC(aclProvider, lifecycleValidatorCommitter)

	var chaincodes []scc.SelfDescribingSysCC
	chaincodes = append(chaincodes, lsccInst, csccInst, qsccInst, lifecycleSCC)
	chaincodes = append(chaincodes, extscc...)

	// deploy system chaincodes
	for _, cc := range chaincodes {
		if enabled, ok := chaincodeConfig.SCCWhitelist[cc.Name()]; !ok || !enabled {
			logger.Infof("not deploying chaincode %s as it is not enabled", cc.Name())
			continue
		}
		scc.DeploySysCC(cc, chaincodeSupport)
	}

	logger.Infof("Deployed system chaincodes")

	// register the lifecycleMetadataManager to get updates from the legacy
	// chaincode; lifecycleMetadataManager will aggregate these updates with
	// the ones from the new lifecycle and deliver both
	// this is expected to disappear with FAB-15061
	legacyMetadataManager.AddListener(metadataManager)

	// register gossip as a listener for updates from lifecycleMetadataManager
	metadataManager.AddListener(lifecycle.HandleMetadataUpdateFunc(func(channel string, chaincodes ccdef.MetadataSet) {
		gossipService.UpdateChaincodes(chaincodes.AsChaincodes(), gossipcommon.ChannelID(channel))
	}))

	// this brings up all the channels
	peerInstance.Initialize(
		func(cid string) {
			// initialize the metadata for this channel.
			// This call will pre-populate chaincode information for this
			// channel but it won't fire any updates to its listeners
			lifecycleCache.InitializeMetadata(cid)

			// initialize the legacyMetadataManager for this channel.
			// This call will pre-populate chaincode information from
			// the legacy lifecycle for this channel; it will also fire
			// the listener, which will cascade to metadataManager
			// and eventually to gossip to pre-populate data structures.
			// this is expected to disappear with FAB-15061
			sub, err := legacyMetadataManager.NewChannelSubscription(cid, cclifecycle.QueryCreatorFunc(func() (cclifecycle.Query, error) {
				return peerInstance.GetLedger(cid).NewQueryExecutor()
			}))
			if err != nil {
				logger.Panicf("Failed subscribing to chaincode lifecycle updates")
			}

			// register this channel's legacyMetadataManager (sub) to get ledger updates
			// this is expected to disappear with FAB-15061
			cceventmgmt.GetMgr().Register(cid, sub)
		},
		plugin.MapBasedMapper(validationPluginsByName),
		lifecycleValidatorCommitter,
		lsccInst,
		lifecycleValidatorCommitter,
		coreConfig.ValidatorPoolSize,
		collDataProvider,
	)

	if coreConfig.DiscoveryEnabled {
		registerDiscoveryService(
			coreConfig,
			peerInstance,
			peerServer,
			policyMgr,
			lifecycle.NewMetadataProvider(
				lifecycleCache,
				legacyMetadataManager,
				peerInstance,
			),
			gossipService,
		)
	}

	logger.Infof("Starting peer with ID=[%s], network ID=[%s], address=[%s]", coreConfig.PeerID, coreConfig.NetworkID, coreConfig.PeerAddress)

	// Get configuration before starting go routines to avoid
	// racing in tests
	profileEnabled := coreConfig.ProfileEnabled
	profileListenAddress := coreConfig.ProfileListenAddress

	// Start the grpc server. Done in a goroutine so we can deploy the
	// genesis block if needed.
	serve := make(chan error)

	// Start profiling http endpoint if enabled
	if profileEnabled {
		go func() {
			logger.Infof("Starting profiling server with listenAddress = %s", profileListenAddress)
			if profileErr := http.ListenAndServe(profileListenAddress, nil); profileErr != nil {
				logger.Errorf("Error starting profiler: %s", profileErr)
			}
		}()
	}

	go handleSignals(addPlatformSignals(map[os.Signal]func(){
		syscall.SIGINT:  func() { serve <- nil },
		syscall.SIGTERM: func() { serve <- nil },
	}))

	logger.Infof("Started peer with ID=[%s], network ID=[%s], address=[%s]", coreConfig.PeerID, coreConfig.NetworkID, coreConfig.PeerAddress)

	// get a list of ledger IDs and load preResetHeight files for these ledger IDs
	ledgerIDs, err := peerInstance.LedgerMgr.GetLedgerIDs()
	if err != nil {
		return errors.WithMessage(err, "failed to get ledger IDs")
	}

	// check to see if the peer ledgers have been reset
	rootFSPath := filepath.Join(coreconfig.GetPath("peer.fileSystemPath"), "ledgersData")
	preResetHeights, err := kvledger.LoadPreResetHeight(rootFSPath, ledgerIDs)
	if err != nil {
		return fmt.Errorf("error loading prereset height: %s", err)
	}

	for cid, height := range preResetHeights {
		logger.Infof("Ledger rebuild: channel [%s]: preresetHeight: [%d]", cid, height)
	}

	if len(preResetHeights) > 0 {
		logger.Info("Ledger rebuild: Entering loop to check if current ledger heights surpass prereset ledger heights. Endorsement request processing will be disabled.")
		resetFilter := &reset{
			reject: true,
		}
		authFilters = append(authFilters, resetFilter)
		go resetLoop(resetFilter, preResetHeights, ledgerIDs, peerInstance.GetLedger, 10*time.Second)
	}

	// start the peer server
	auth := authHandler.ChainFilters(serverEndorser, authFilters...)
	// Register the Endorser server
	pb.RegisterEndorserServer(peerServer.Server(), auth)

	go func() {
		var grpcErr error
		if grpcErr = peerServer.Start(); grpcErr != nil {
			grpcErr = fmt.Errorf("grpc server exited with error: %s", grpcErr)
		}
		serve <- grpcErr
	}()

	// Block until grpc server exits
	return <-serve
}

func handleSignals(handlers map[os.Signal]func()) {
	var signals []os.Signal
	for sig := range handlers {
		signals = append(signals, sig)
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, signals...)

	for sig := range signalChan {
		logger.Infof("Received signal: %d (%s)", sig, sig)
		handlers[sig]()
	}
}

func localPolicy(policyObject proto.Message) policies.Policy {
	localMSP := mgmt.GetLocalMSP(factory.GetDefault())
	pp := cauthdsl.NewPolicyProvider(localMSP)
	policy, _, err := pp.NewPolicy(protoutil.MarshalOrPanic(policyObject))
	if err != nil {
		logger.Panicf("Failed creating local policy: +%v", err)
	}
	return policy
}

func createSelfSignedData() protoutil.SignedData {
	sID := mgmt.GetLocalSigningIdentityOrPanic(factory.GetDefault())
	msg := make([]byte, 32)
	sig, err := sID.Sign(msg)
	if err != nil {
		logger.Panicf("Failed creating self signed data because message signing failed: %v", err)
	}
	peerIdentity, err := sID.Serialize()
	if err != nil {
		logger.Panicf("Failed creating self signed data because peer identity couldn't be serialized: %v", err)
	}
	return protoutil.SignedData{
		Data:      msg,
		Signature: sig,
		Identity:  peerIdentity,
	}
}

func registerDiscoveryService(
	coreConfig *peer.Config,
	peerInstance *peer.Peer,
	peerServer *comm.GRPCServer,
	polMgr policies.ChannelPolicyManagerGetter,
	metadataProvider *lifecycle.MetadataProvider,
	gossipService *gossipservice.GossipService,
) {
	mspID := coreConfig.LocalMSPID
	localAccessPolicy := localPolicy(cauthdsl.SignedByAnyAdmin([]string{mspID}))
	if coreConfig.DiscoveryOrgMembersAllowed {
		localAccessPolicy = localPolicy(cauthdsl.SignedByAnyMember([]string{mspID}))
	}
	channelVerifier := discacl.NewChannelVerifier(policies.ChannelApplicationWriters, polMgr)
	acl := discacl.NewDiscoverySupport(channelVerifier, localAccessPolicy, discacl.ChannelConfigGetterFunc(peerInstance.GetStableChannelConfig))
	gSup := gossip.NewDiscoverySupport(gossipService)
	ccSup := ccsupport.NewDiscoverySupport(metadataProvider)
	ea := endorsement.NewEndorsementAnalyzer(gSup, ccSup, acl, metadataProvider)
	confSup := config.NewDiscoverySupport(config.CurrentConfigBlockGetterFunc(func(channelID string) *common.Block {
		channel := peerInstance.Channel(channelID)
		if channel == nil {
			return nil
		}
		block, err := peer.ConfigBlockFromLedger(channel.Ledger())
		if err != nil {
			logger.Error("failed to get config block", err)
			return nil
		}
		return block
	}))
	support := discsupport.NewDiscoverySupport(acl, gSup, ea, confSup, acl)
	svc := discovery.NewService(discovery.Config{
		TLS:                          peerServer.TLSEnabled(),
		AuthCacheEnabled:             coreConfig.DiscoveryAuthCacheEnabled,
		AuthCacheMaxSize:             coreConfig.DiscoveryAuthCacheMaxSize,
		AuthCachePurgeRetentionRatio: coreConfig.DiscoveryAuthCachePurgeRetentionRatio,
	}, support)
	logger.Info("Discovery service activated")
	discprotos.RegisterDiscoveryServer(peerServer.Server(), svc)
}

//create a CC listener using peer.chaincodeListenAddress (and if that's not set use peer.peerAddress)
func createChaincodeServer(coreConfig *peer.Config, ca tlsgen.CA, peerHostname string) (srv *comm.GRPCServer, ccEndpoint string, err error) {
	// before potentially setting chaincodeListenAddress, compute chaincode endpoint at first
	ccEndpoint, err = computeChaincodeEndpoint(coreConfig.ChaincodeAddress, coreConfig.ChaincodeListenAddress, peerHostname)
	if err != nil {
		if chaincode.IsDevMode() {
			// if any error for dev mode, we use 0.0.0.0:7052
			ccEndpoint = fmt.Sprintf("%s:%d", "0.0.0.0", defaultChaincodePort)
			logger.Warningf("use %s as chaincode endpoint because of error in computeChaincodeEndpoint: %s", ccEndpoint, err)
		} else {
			// for non-dev mode, we have to return error
			logger.Errorf("Error computing chaincode endpoint: %s", err)
			return nil, "", err
		}
	}

	host, _, err := net.SplitHostPort(ccEndpoint)
	if err != nil {
		logger.Panic("Chaincode service host", ccEndpoint, "isn't a valid hostname:", err)
	}

	cclistenAddress := coreConfig.ChaincodeListenAddress
	if cclistenAddress == "" {
		cclistenAddress = fmt.Sprintf("%s:%d", peerHostname, defaultChaincodePort)
		logger.Warningf("%s is not set, using %s", chaincodeListenAddrKey, cclistenAddress)
		coreConfig.ChaincodeListenAddress = cclistenAddress
	}

	config, err := peer.GetServerConfig()
	if err != nil {
		logger.Errorf("Error getting server config: %s", err)
		return nil, "", err
	}

	// set the logger for the server
	config.Logger = flogging.MustGetLogger("core.comm").With("server", "ChaincodeServer")

	// Override TLS configuration if TLS is applicable
	if config.SecOpts.UseTLS {
		// Create a self-signed TLS certificate with a SAN that matches the computed chaincode endpoint
		certKeyPair, err := ca.NewServerCertKeyPair(host)
		if err != nil {
			logger.Panicf("Failed generating TLS certificate for chaincode service: +%v", err)
		}
		config.SecOpts = comm.SecureOptions{
			UseTLS: true,
			// Require chaincode shim to authenticate itself
			RequireClientCert: true,
			// Trust only client certificates signed by ourselves
			ClientRootCAs: [][]byte{ca.CertBytes()},
			// Use our own self-signed TLS certificate and key
			Certificate: certKeyPair.Cert,
			Key:         certKeyPair.Key,
			// No point in specifying server root CAs since this TLS config is only used for
			// a gRPC server and not a client
			ServerRootCAs: nil,
		}
	}

	// Chaincode keepalive options - static for now
	chaincodeKeepaliveOptions := comm.KeepaliveOptions{
		ServerInterval:    time.Duration(2) * time.Hour,    // 2 hours - gRPC default
		ServerTimeout:     time.Duration(20) * time.Second, // 20 sec - gRPC default
		ServerMinInterval: time.Duration(1) * time.Minute,  // match ClientInterval
	}
	config.KaOpts = chaincodeKeepaliveOptions
	config.HealthCheckEnabled = true

	srv, err = comm.NewGRPCServer(cclistenAddress, config)
	if err != nil {
		logger.Errorf("Error creating GRPC server: %s", err)
		return nil, "", err
	}

	return srv, ccEndpoint, nil
}

// computeChaincodeEndpoint will utilize chaincode address, chaincode listen
// address (these two are from viper) and peer address to compute chaincode endpoint.
// There could be following cases of computing chaincode endpoint:
// Case A: if chaincodeAddrKey is set, use it if not "0.0.0.0" (or "::")
// Case B: else if chaincodeListenAddressKey is set and not "0.0.0.0" or ("::"), use it
// Case C: else use peer address if not "0.0.0.0" (or "::")
// Case D: else return error
func computeChaincodeEndpoint(chaincodeAddress string, chaincodeListenAddress string, peerHostname string) (ccEndpoint string, err error) {
	logger.Infof("Entering computeChaincodeEndpoint with peerHostname: %s", peerHostname)
	// Case A: the chaincodeAddrKey is set
	if chaincodeAddress != "" {
		host, _, err := net.SplitHostPort(chaincodeAddress)
		if err != nil {
			logger.Errorf("Fail to split chaincodeAddress: %s", err)
			return "", err
		}
		ccIP := net.ParseIP(host)
		if ccIP != nil && ccIP.IsUnspecified() {
			logger.Errorf("ChaincodeAddress' IP cannot be %s in non-dev mode", ccIP)
			return "", errors.New("invalid endpoint for chaincode to connect")
		}
		logger.Infof("Exit with ccEndpoint: %s", chaincodeAddress)
		return chaincodeAddress, nil
	}

	// Case B: chaincodeListenAddrKey is set
	if chaincodeListenAddress != "" {
		ccEndpoint = chaincodeListenAddress
		host, port, err := net.SplitHostPort(ccEndpoint)
		if err != nil {
			logger.Errorf("ChaincodeAddress is nil and fail to split chaincodeListenAddress: %s", err)
			return "", err
		}

		ccListenerIP := net.ParseIP(host)
		// ignoring other values such as Multicast address etc ...as the server
		// wouldn't start up with this address anyway
		if ccListenerIP != nil && ccListenerIP.IsUnspecified() {
			// Case C: if "0.0.0.0" or "::", we have to use peer address with the listen port
			peerIP := net.ParseIP(peerHostname)
			if peerIP != nil && peerIP.IsUnspecified() {
				// Case D: all we have is "0.0.0.0" or "::" which chaincode cannot connect to
				logger.Error("ChaincodeAddress is nil while both chaincodeListenAddressIP and peerIP are 0.0.0.0")
				return "", errors.New("invalid endpoint for chaincode to connect")
			}
			ccEndpoint = fmt.Sprintf("%s:%s", peerHostname, port)
		}
		logger.Infof("Exit with ccEndpoint: %s", ccEndpoint)
		return ccEndpoint, nil
	}

	// Case C: chaincodeListenAddrKey is not set, use peer address
	peerIP := net.ParseIP(peerHostname)
	if peerIP != nil && peerIP.IsUnspecified() {
		// Case D: all we have is "0.0.0.0" or "::" which chaincode cannot connect to
		logger.Errorf("ChaincodeAddress and chaincodeListenAddress are nil and peerIP is %s", peerIP)
		return "", errors.New("invalid endpoint for chaincode to connect")
	}

	// use peerAddress:defaultChaincodePort
	ccEndpoint = fmt.Sprintf("%s:%d", peerHostname, defaultChaincodePort)

	logger.Infof("Exit with ccEndpoint: %s", ccEndpoint)
	return ccEndpoint, nil
}

// secureDialOpts is the callback function for secure dial options for gossip service
func secureDialOpts(credSupport *comm.CredentialSupport) func() []grpc.DialOption {
	return func() []grpc.DialOption {
		var dialOpts []grpc.DialOption
		// set max send/recv msg sizes
		dialOpts = append(
			dialOpts,
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(comm.MaxRecvMsgSize), grpc.MaxCallSendMsgSize(comm.MaxSendMsgSize)),
		)
		// set the keepalive options
		kaOpts := comm.DefaultKeepaliveOptions
		if viper.IsSet("peer.keepalive.client.interval") {
			kaOpts.ClientInterval = viper.GetDuration("peer.keepalive.client.interval")
		}
		if viper.IsSet("peer.keepalive.client.timeout") {
			kaOpts.ClientTimeout = viper.GetDuration("peer.keepalive.client.timeout")
		}
		dialOpts = append(dialOpts, comm.ClientKeepaliveOptions(kaOpts)...)

		if viper.GetBool("peer.tls.enabled") {
			dialOpts = append(dialOpts, grpc.WithTransportCredentials(credSupport.GetPeerCredentials()))
		} else {
			dialOpts = append(dialOpts, grpc.WithInsecure())
		}
		return dialOpts
	}
}

// initGossipService will initialize the gossip service by:
// 1. Enable TLS if configured;
// 2. Init the message crypto service;
// 3. Init the security advisor;
// 4. Init gossip related struct.
func initGossipService(
	policyMgr policies.ChannelPolicyManagerGetter,
	metricsProvider metrics.Provider,
	peerServer *comm.GRPCServer,
	signer msp.SigningIdentity,
	credSupport *comm.CredentialSupport,
	peerAddress string,
	deliverGRPCClient *comm.GRPCClient,
	deliverServiceConfig *deliverservice.DeliverServiceConfig,
) (*gossipservice.GossipService, error) {

	var certs *gossipcommon.TLSCertificates
	if peerServer.TLSEnabled() {
		serverCert := peerServer.ServerCertificate()
		clientCert, err := peer.GetClientCertificate()
		if err != nil {
			return nil, errors.Wrap(err, "failed obtaining client certificates")
		}
		certs = &gossipcommon.TLSCertificates{}
		certs.TLSServerCert.Store(&serverCert)
		certs.TLSClientCert.Store(&clientCert)
	}

	messageCryptoService := peergossip.NewMCS(
		policyMgr,
		signer,
		mgmt.NewDeserializersManager(factory.GetDefault()),
		factory.GetDefault(),
	)
	secAdv := peergossip.NewSecurityAdvisor(mgmt.NewDeserializersManager(factory.GetDefault()))
	bootstrap := viper.GetStringSlice("peer.gossip.bootstrap")

	serviceConfig := service.GlobalConfig()
	if serviceConfig.Endpoint != "" {
		peerAddress = serviceConfig.Endpoint
	}
	gossipConfig, err := gossipgossip.GlobalConfig(peerAddress, certs, bootstrap...)
	if err != nil {
		return nil, errors.Wrap(err, "failed obtaining gossip config")
	}

	return gossipservice.New(
		signer,
		gossipmetrics.NewGossipMetrics(metricsProvider),
		peerAddress,
		peerServer.Server(),
		messageCryptoService,
		secAdv,
		secureDialOpts(credSupport),
		credSupport,
		deliverGRPCClient,
		gossipConfig,
		serviceConfig,
		deliverServiceConfig,
	)
}

func newOperationsSystem(coreConfig *peer.Config) *operations.System {
	return operations.NewSystem(operations.Options{
		Logger:        flogging.MustGetLogger("peer.operations"),
		ListenAddress: coreConfig.OperationsListenAddress,
		Metrics: operations.MetricsOptions{
			Provider: coreConfig.MetricsProvider,
			Statsd: &operations.Statsd{
				Network:       coreConfig.StatsdNetwork,
				Address:       coreConfig.StatsdAaddress,
				WriteInterval: coreConfig.StatsdWriteInterval,
				Prefix:        coreConfig.StatsdPrefix,
			},
		},
		TLS: operations.TLS{
			Enabled:            coreConfig.OperationsTLSEnabled,
			CertFile:           coreConfig.OperationsTLSCertFile,
			KeyFile:            coreConfig.OperationsTLSKeyFile,
			ClientCertRequired: coreConfig.OperationsTLSClientAuthRequired,
			ClientCACertFiles:  coreConfig.OperationsTLSClientRootCAs,
		},
		Version: metadata.Version,
	})
}

func getDockerHostConfig() *docker.HostConfig {
	dockerKey := func(key string) string { return "vm.docker.hostConfig." + key }
	getInt64 := func(key string) int64 { return int64(viper.GetInt(dockerKey(key))) }

	var logConfig docker.LogConfig
	err := viper.UnmarshalKey(dockerKey("LogConfig"), &logConfig)
	if err != nil {
		logger.Panicf("unable to parse Docker LogConfig: %s", err)
	}

	networkMode := viper.GetString(dockerKey("NetworkMode"))
	if networkMode == "" {
		networkMode = "host"
	}

	memorySwappiness := getInt64("MemorySwappiness")
	oomKillDisable := viper.GetBool(dockerKey("OomKillDisable"))

	return &docker.HostConfig{
		CapAdd:  viper.GetStringSlice(dockerKey("CapAdd")),
		CapDrop: viper.GetStringSlice(dockerKey("CapDrop")),

		DNS:         viper.GetStringSlice(dockerKey("Dns")),
		DNSSearch:   viper.GetStringSlice(dockerKey("DnsSearch")),
		ExtraHosts:  viper.GetStringSlice(dockerKey("ExtraHosts")),
		NetworkMode: networkMode,
		IpcMode:     viper.GetString(dockerKey("IpcMode")),
		PidMode:     viper.GetString(dockerKey("PidMode")),
		UTSMode:     viper.GetString(dockerKey("UTSMode")),
		LogConfig:   logConfig,

		ReadonlyRootfs:   viper.GetBool(dockerKey("ReadonlyRootfs")),
		SecurityOpt:      viper.GetStringSlice(dockerKey("SecurityOpt")),
		CgroupParent:     viper.GetString(dockerKey("CgroupParent")),
		Memory:           getInt64("Memory"),
		MemorySwap:       getInt64("MemorySwap"),
		MemorySwappiness: &memorySwappiness,
		OOMKillDisable:   &oomKillDisable,
		CPUShares:        getInt64("CpuShares"),
		CPUSet:           viper.GetString(dockerKey("Cpuset")),
		CPUSetCPUs:       viper.GetString(dockerKey("CpusetCPUs")),
		CPUSetMEMs:       viper.GetString(dockerKey("CpusetMEMs")),
		CPUQuota:         getInt64("CpuQuota"),
		CPUPeriod:        getInt64("CpuPeriod"),
		BlkioWeight:      getInt64("BlkioWeight"),
	}
}

//go:generate counterfeiter -o mock/get_ledger.go -fake-name GetLedger . getLedger
//go:generate counterfeiter -o mock/peer_ledger.go -fake-name PeerLedger . peerLedger

type peerLedger interface {
	ledger.PeerLedger
}

type getLedger func(string) ledger.PeerLedger

func resetLoop(
	resetFilter *reset,
	preResetHeights map[string]uint64,
	ledgerIDs []string,
	pLedger getLedger,
	interval time.Duration,
) {
	// periodically check to see if current ledger height(s) surpass prereset height(s)
	ticker := time.NewTicker(interval)

	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			logger.Info("Ledger rebuild: Checking if current ledger heights surpass prereset ledger heights")
			logger.Debugf("Ledger rebuild: Number of ledgers still rebuilding before check: %d", len(preResetHeights))
			for cid, height := range preResetHeights {
				var l peerLedger
				l = pLedger(cid)
				if l == nil {
					logger.Warningf("No ledger found for channel [%s]", cid)
					continue
				}
				bcInfo, err := l.GetBlockchainInfo()
				if bcInfo != nil {
					logger.Debugf("Ledger rebuild: channel [%s]: currentHeight [%d] : preresetHeight [%d]", cid, bcInfo.GetHeight(), height)
					if bcInfo.GetHeight() >= height {
						delete(preResetHeights, cid)
					} else {
						break
					}
				} else {
					if err != nil {
						logger.Warningf("Ledger rebuild: could not retrieve info for channel [%s]: %s", cid, err.Error())
					}
				}
			}

			logger.Debugf("Ledger rebuild: Number of ledgers still rebuilding after check: %d", len(preResetHeights))
			if len(preResetHeights) == 0 {
				logger.Infof("Ledger rebuild: Complete, all ledgers surpass prereset heights. Endorsement request processing will be enabled.")
				rootFSPath := filepath.Join(coreconfig.GetPath("peer.fileSystemPath"), "ledgersData")
				err := kvledger.ClearPreResetHeight(rootFSPath, ledgerIDs)
				if err != nil {
					logger.Warningf("Ledger rebuild: could not clear off prerest files: error=%s", err)
				}
				resetFilter.setReject(false)
				return
			}
		}
	}
}

//implements the auth.Filter interface
type reset struct {
	sync.RWMutex
	next   pb.EndorserServer
	reject bool
}

func (r *reset) setReject(reject bool) {
	r.Lock()
	defer r.Unlock()
	r.reject = reject
}

// Init initializes Reset with the next EndorserServer
func (r *reset) Init(next pb.EndorserServer) {
	r.next = next
}

// ProcessProposal processes a signed proposal
func (r *reset) ProcessProposal(ctx context.Context, signedProp *pb.SignedProposal) (*pb.ProposalResponse, error) {
	r.RLock()
	defer r.RUnlock()
	if r.reject {
		return nil, errors.New("endorse requests are blocked while ledgers are being rebuilt")
	}
	return r.next.ProcessProposal(ctx, signedProp)
}

func Serve(args []string) error {
	return serve(args)
}
