/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package service

import (
	"sync"

	"github.com/hyperledger/fabric/common/metrics"
	"github.com/hyperledger/fabric/core/committer"
	"github.com/hyperledger/fabric/core/committer/txvalidator"
	"github.com/hyperledger/fabric/core/common/privdata"
	"github.com/hyperledger/fabric/core/deliverservice"
	"github.com/hyperledger/fabric/core/deliverservice/blocksprovider"
	"github.com/hyperledger/fabric/core/ledger"
	storeapi "github.com/hyperledger/fabric/extensions/collections/api/store"
	extgossipapi "github.com/hyperledger/fabric/extensions/gossip/api"
	"github.com/hyperledger/fabric/extensions/gossip/dispatcher"
	"github.com/hyperledger/fabric/gossip/api"
	gossipCommon "github.com/hyperledger/fabric/gossip/common"
	"github.com/hyperledger/fabric/gossip/election"
	"github.com/hyperledger/fabric/gossip/gossip"
	"github.com/hyperledger/fabric/gossip/integration"
	gossipMetrics "github.com/hyperledger/fabric/gossip/metrics"
	privdata2 "github.com/hyperledger/fabric/gossip/privdata"
	"github.com/hyperledger/fabric/gossip/state"
	"github.com/hyperledger/fabric/gossip/util"
	"github.com/hyperledger/fabric/internal/pkg/identity"
	gproto "github.com/hyperledger/fabric/protos/gossip"
	"github.com/hyperledger/fabric/protos/transientstore"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

var (
	gossipServiceInstance *gossipServiceImpl
	once                  sync.Once
)

type gossipSvc gossip.Gossip

// GossipService encapsulates gossip and state capabilities into single interface
type GossipService interface {
	gossip.Gossip

	// DistributePrivateData distributes private data to the peers in the collections
	// according to policies induced by the PolicyStore and PolicyParser
	DistributePrivateData(chainID string, txID string, privateData *transientstore.TxPvtReadWriteSetWithConfigInfo, blkHt uint64) error
	// NewConfigEventer creates a ConfigProcessor which the channelconfig.BundleSource can ultimately route config updates to
	NewConfigEventer() ConfigProcessor
	// InitializeChannel allocates the state provider and should be invoked once per channel per execution
	InitializeChannel(chainID string, endpoints []string, support Support)
	// AddPayload appends message payload to for given chain
	AddPayload(chainID string, payload *gproto.Payload) error
}

// DeliveryServiceFactory factory to create and initialize delivery service instance
type DeliveryServiceFactory interface {
	// Returns an instance of delivery client
	Service(g GossipService, endpoints []string, msc api.MessageCryptoService) (deliverservice.DeliverService, error)
}

type deliveryFactoryImpl struct {
	signer identity.SignerSerializer
}

// Returns an instance of delivery client
func (df *deliveryFactoryImpl) Service(g GossipService, endpoints []string, mcs api.MessageCryptoService) (deliverservice.DeliverService, error) {
	return deliverservice.NewDeliverService(&deliverservice.Config{
		CryptoSvc:   mcs,
		Gossip:      g,
		Endpoints:   endpoints,
		ConnFactory: deliverservice.DefaultConnectionFactory,
		ABCFactory:  deliverservice.DefaultABCFactory,
		Signer:      df.signer,
	})
}

type privateHandler struct {
	support     Support
	coordinator privdata2.Coordinator
	distributor privdata2.PvtDataDistributor
	reconciler  privdata2.PvtDataReconciler
}

func (p privateHandler) close() {
	p.coordinator.Close()
	p.reconciler.Stop()
}

type gossipServiceImpl struct {
	gossipSvc
	privateHandlers map[string]privateHandler
	chains          map[string]state.GossipStateProvider
	leaderElection  map[string]election.LeaderElectionService
	deliveryService map[string]deliverservice.DeliverService
	deliveryFactory DeliveryServiceFactory
	lock            sync.RWMutex
	mcs             api.MessageCryptoService
	peerIdentity    []byte
	secAdv          api.SecurityAdvisor
	metrics         *gossipMetrics.GossipMetrics
}

// This is an implementation of api.JoinChannelMessage.
type joinChannelMessage struct {
	seqNum              uint64
	members2AnchorPeers map[string][]api.AnchorPeer
}

func (jcm *joinChannelMessage) SequenceNumber() uint64 {
	return jcm.seqNum
}

// Members returns the organizations of the channel
func (jcm *joinChannelMessage) Members() []api.OrgIdentityType {
	members := make([]api.OrgIdentityType, len(jcm.members2AnchorPeers))
	i := 0
	for org := range jcm.members2AnchorPeers {
		members[i] = api.OrgIdentityType(org)
		i++
	}
	return members
}

// AnchorPeersOf returns the anchor peers of the given organization
func (jcm *joinChannelMessage) AnchorPeersOf(org api.OrgIdentityType) []api.AnchorPeer {
	return jcm.members2AnchorPeers[string(org)]
}

var logger = util.GetLogger(util.ServiceLogger, "")

// InitGossipService initialize gossip service
func InitGossipService(peerIdentity identity.SignerSerializer, metricsProvider metrics.Provider, endpoint string, s *grpc.Server,
	certs *gossipCommon.TLSCertificates, mcs api.MessageCryptoService, secAdv api.SecurityAdvisor,
	secureDialOpts api.PeerSecureDialOpts, bootPeers ...string) error {
	// TODO: Remove this.
	// TODO: This is a temporary work-around to make the gossip leader election module load its logger at startup
	// TODO: in order for the flogging package to register this logger in time so it can set the log levels as requested in the config
	util.GetLogger(util.ElectionLogger, "")

	return InitGossipServiceCustomDeliveryFactory(
		peerIdentity,
		metricsProvider,
		endpoint,
		s,
		certs,
		&deliveryFactoryImpl{
			signer: peerIdentity,
		},
		mcs,
		secAdv,
		secureDialOpts,
		bootPeers...,
	)
}

// InitGossipServiceCustomDeliveryFactory initialize gossip service with customize delivery factory
// implementation, might be useful for testing and mocking purposes
func InitGossipServiceCustomDeliveryFactory(
	peerIdentity identity.SignerSerializer,
	metricsProvider metrics.Provider,
	endpoint string,
	s *grpc.Server,
	certs *gossipCommon.TLSCertificates,
	factory DeliveryServiceFactory,
	mcs api.MessageCryptoService,
	secAdv api.SecurityAdvisor,
	secureDialOpts api.PeerSecureDialOpts,
	bootPeers ...string,
) error {
	var err error
	var gossip gossip.Gossip
	serializedIdentity, err := peerIdentity.Serialize()
	if err != nil {
		return err
	}
	once.Do(func() {
		if overrideEndpoint := viper.GetString("peer.gossip.endpoint"); overrideEndpoint != "" {
			endpoint = overrideEndpoint
		}

		logger.Info("Initialize gossip with endpoint", endpoint, "and bootstrap set", bootPeers)

		gossipMetrics := gossipMetrics.NewGossipMetrics(metricsProvider)

		gossip, err = integration.NewGossipComponent(serializedIdentity, endpoint, s, secAdv,
			mcs, secureDialOpts, certs, gossipMetrics, bootPeers...)
		gossipServiceInstance = &gossipServiceImpl{
			mcs:             mcs,
			gossipSvc:       gossip,
			privateHandlers: make(map[string]privateHandler),
			chains:          make(map[string]state.GossipStateProvider),
			leaderElection:  make(map[string]election.LeaderElectionService),
			deliveryService: make(map[string]deliverservice.DeliverService),
			deliveryFactory: factory,
			peerIdentity:    serializedIdentity,
			secAdv:          secAdv,
			metrics:         gossipMetrics,
		}
	})
	return errors.WithStack(err)
}

// GetGossipService returns an instance of gossip service
func GetGossipService() GossipService {
	return gossipServiceInstance
}

// DistributePrivateData distribute private read write set inside the channel based on the collections policies
func (g *gossipServiceImpl) DistributePrivateData(chainID string, txID string, privData *transientstore.TxPvtReadWriteSetWithConfigInfo, blkHt uint64) error {
	g.lock.RLock()
	handler, exists := g.privateHandlers[chainID]
	g.lock.RUnlock()
	if !exists {
		return errors.Errorf("No private data handler for %s", chainID)
	}

	if err := handler.distributor.Distribute(txID, privData, blkHt); err != nil {
		logger.Error("Failed to distributed private collection, txID", txID, "channel", chainID, "due to", err)
		return err
	}

	if err := handler.coordinator.StorePvtData(txID, privData, blkHt); err != nil {
		logger.Error("Failed to store private data into transient store, txID",
			txID, "channel", chainID, "due to", err)
		return err
	}
	return nil
}

// NewConfigEventer creates a ConfigProcessor which the channelconfig.BundleSource can ultimately route config updates to
func (g *gossipServiceImpl) NewConfigEventer() ConfigProcessor {
	return newConfigEventer(g)
}

// Support aggregates functionality of several
// interfaces required by gossip service
type Support struct {
	Validator            txvalidator.Validator
	Committer            committer.Committer
	Store                privdata2.TransientStore
	Cs                   privdata.CollectionStore
	IdDeserializeFactory privdata2.IdentityDeserializerFactory
	CollDataStore        storeapi.Store
	Ledger               ledger.PeerLedger
	BlockPublisher       extgossipapi.BlockPublisher
}

// DataStoreSupport aggregates interfaces capable
// of handling either incoming blocks or private data
type DataStoreSupport struct {
	committer.Committer
	privdata2.TransientStore
}

// InitializeChannel allocates the state provider and should be invoked once per channel per execution
func (g *gossipServiceImpl) InitializeChannel(chainID string, endpoints []string, support Support) {
	g.lock.Lock()
	defer g.lock.Unlock()
	// Initialize new state provider for given committer
	logger.Debug("Creating state provider for chainID", chainID)
	servicesAdapter := &state.ServicesMediator{GossipAdapter: g, MCSAdapter: g.mcs}

	// Embed transient store and committer APIs to fulfill
	// DataStore interface to capture ability of retrieving
	// private data
	storeSupport := &DataStoreSupport{
		TransientStore: support.Store,
		Committer:      support.Committer,
	}
	// Initialize private data fetcher
	dataRetriever := privdata2.NewDataRetriever(storeSupport)
	collectionAccessFactory := privdata2.NewCollectionAccessFactory(support.IdDeserializeFactory)
	fetcher := privdata2.NewPuller(g.metrics.PrivdataMetrics, support.Cs, g.gossipSvc, dataRetriever,
		collectionAccessFactory, chainID, privdata2.GetBtlPullMargin())

	coordinatorConfig := privdata2.CoordinatorConfig{
		TransientBlockRetention: privdata2.GetTransientBlockRetention(),
		PullRetryThreshold:      viper.GetDuration("peer.gossip.pvtData.pullRetryThreshold"),
	}
	coordinator := privdata2.NewCoordinator(privdata2.Support{
		ChainID:         chainID,
		CollectionStore: support.Cs,
		Validator:       support.Validator,
		TransientStore:  support.Store,
		CollDataStore:   support.CollDataStore,
		Committer:       support.Committer,
		Fetcher:         fetcher,
	}, g.createSelfSignedData(), g.metrics.PrivdataMetrics, coordinatorConfig)

	reconcilerConfig := privdata2.GetReconcilerConfig()
	var reconciler privdata2.PvtDataReconciler

	if reconcilerConfig.IsEnabled {
		reconciler = privdata2.NewReconciler(chainID, g.metrics.PrivdataMetrics,
			support.Committer, fetcher, reconcilerConfig)
	} else {
		reconciler = &privdata2.NoOpReconciler{}
	}

	pushAckTimeout := viper.GetDuration("peer.gossip.pvtData.pushAckTimeout")
	g.privateHandlers[chainID] = privateHandler{
		support:     support,
		coordinator: coordinator,
		distributor: privdata2.NewDistributor(chainID, g, collectionAccessFactory, g.metrics.PrivdataMetrics, pushAckTimeout),
		reconciler:  reconciler,
	}
	g.privateHandlers[chainID].reconciler.Start()

	blockingMode := !viper.GetBool("peer.gossip.nonBlockingCommitMode")
	g.chains[chainID] = state.NewGossipStateProvider(chainID, servicesAdapter, coordinator,
		g.metrics.StateMetrics, blockingMode,
		dispatcher.New(chainID, support.CollDataStore, servicesAdapter, support.Ledger, support.BlockPublisher))
	if g.deliveryService[chainID] == nil {
		var err error
		g.deliveryService[chainID], err = g.deliveryFactory.Service(g, endpoints, g.mcs)
		if err != nil {
			logger.Warningf("Cannot create delivery client, due to %+v", errors.WithStack(err))
		}
	}

	// Delivery service might be nil only if it was not able to get connected
	// to the ordering service
	if g.deliveryService[chainID] != nil {
		// Parameters:
		//              - peer.gossip.useLeaderElection
		//              - peer.gossip.orgLeader
		//
		// are mutual exclusive, setting both to true is not defined, hence
		// peer will panic and terminate
		leaderElection := viper.GetBool("peer.gossip.useLeaderElection")
		isStaticOrgLeader := viper.GetBool("peer.gossip.orgLeader")

		if leaderElection && isStaticOrgLeader {
			logger.Panic("Setting both orgLeader and useLeaderElection to true isn't supported, aborting execution")
		}

		if leaderElection {
			logger.Debug("Delivery uses dynamic leader election mechanism, channel", chainID)
			g.leaderElection[chainID] = g.newLeaderElectionComponent(chainID, g.onStatusChangeFactory(chainID,
				support.Committer), g.metrics.ElectionMetrics)
		} else if isStaticOrgLeader {
			logger.Debug("This peer is configured to connect to ordering service for blocks delivery, channel", chainID)
			g.deliveryService[chainID].StartDeliverForChannel(chainID, support.Committer, func() {})
		} else {
			logger.Debug("This peer is not configured to connect to ordering service for blocks delivery, channel", chainID)
		}
	} else {
		logger.Warning("Delivery client is down won't be able to pull blocks for chain", chainID)
	}
}

func (g *gossipServiceImpl) createSelfSignedData() protoutil.SignedData {
	msg := make([]byte, 32)
	sig, err := g.mcs.Sign(msg)
	if err != nil {
		logger.Panicf("Failed creating self signed data because message signing failed: %v", err)
	}
	return protoutil.SignedData{
		Data:      msg,
		Signature: sig,
		Identity:  g.peerIdentity,
	}
}

// updateAnchors constructs a joinChannelMessage and sends it to the gossipSvc
func (g *gossipServiceImpl) updateAnchors(config Config) {
	myOrg := string(g.secAdv.OrgByPeerIdentity(api.PeerIdentityType(g.peerIdentity)))
	if !g.amIinChannel(myOrg, config) {
		logger.Error("Tried joining channel", config.ChainID(), "but our org(", myOrg, "), isn't "+
			"among the orgs of the channel:", orgListFromConfig(config), ", aborting.")
		return
	}
	jcm := &joinChannelMessage{seqNum: config.Sequence(), members2AnchorPeers: map[string][]api.AnchorPeer{}}
	for _, appOrg := range config.Organizations() {
		logger.Debug(appOrg.MSPID(), "anchor peers:", appOrg.AnchorPeers())
		jcm.members2AnchorPeers[appOrg.MSPID()] = []api.AnchorPeer{}
		for _, ap := range appOrg.AnchorPeers() {
			anchorPeer := api.AnchorPeer{
				Host: ap.Host,
				Port: int(ap.Port),
			}
			jcm.members2AnchorPeers[appOrg.MSPID()] = append(jcm.members2AnchorPeers[appOrg.MSPID()], anchorPeer)
		}
	}

	// Initialize new state provider for given committer
	logger.Debug("Creating state provider for chainID", config.ChainID())
	g.JoinChan(jcm, gossipCommon.ChainID(config.ChainID()))
}

func (g *gossipServiceImpl) updateEndpoints(chainID string, endpoints []string) {
	if ds, ok := g.deliveryService[chainID]; ok {
		logger.Debugf("Updating endpoints for chainID %s", chainID)
		if err := ds.UpdateEndpoints(chainID, endpoints); err != nil {
			// The only reason to fail is because of absence of block provider
			// for given channel id, hence printing a warning will be enough
			logger.Warningf("Failed to update ordering service endpoints, due to %s", err)
		}
	}
}

// AddPayload appends message payload to for given chain
func (g *gossipServiceImpl) AddPayload(chainID string, payload *gproto.Payload) error {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.chains[chainID].AddPayload(payload)
}

// Stop stops the gossip component
func (g *gossipServiceImpl) Stop() {
	g.lock.Lock()
	defer g.lock.Unlock()

	for chainID := range g.chains {
		logger.Info("Stopping chain", chainID)
		if le, exists := g.leaderElection[chainID]; exists {
			logger.Infof("Stopping leader election for %s", chainID)
			le.Stop()
		}
		g.chains[chainID].Stop()
		g.privateHandlers[chainID].close()

		if g.deliveryService[chainID] != nil {
			g.deliveryService[chainID].Stop()
		}
	}
	g.gossipSvc.Stop()
}

func (g *gossipServiceImpl) newLeaderElectionComponent(chainID string, callback func(bool),
	electionMetrics *gossipMetrics.ElectionMetrics) election.LeaderElectionService {
	PKIid := g.mcs.GetPKIidOfCert(g.peerIdentity)
	adapter := election.NewAdapter(g, PKIid, gossipCommon.ChainID(chainID), electionMetrics)
	config := election.ElectionConfig{
		StartupGracePeriod:       util.GetDurationOrDefault("peer.gossip.election.startupGracePeriod", election.DefStartupGracePeriod),
		MembershipSampleInterval: util.GetDurationOrDefault("peer.gossip.election.membershipSampleInterval", election.DefMembershipSampleInterval),
		LeaderAliveThreshold:     util.GetDurationOrDefault("peer.gossip.election.leaderAliveThreshold", election.DefLeaderAliveThreshold),
		LeaderElectionDuration:   util.GetDurationOrDefault("peer.gossip.election.leaderElectionDuration", election.DefLeaderElectionDuration),
	}
	return election.NewLeaderElectionService(adapter, string(PKIid), callback, config)
}

func (g *gossipServiceImpl) amIinChannel(myOrg string, config Config) bool {
	for _, orgName := range orgListFromConfig(config) {
		if orgName == myOrg {
			return true
		}
	}
	return false
}

func (g *gossipServiceImpl) onStatusChangeFactory(chainID string, committer blocksprovider.LedgerInfo) func(bool) {
	return func(isLeader bool) {
		if isLeader {
			yield := func() {
				g.lock.RLock()
				le := g.leaderElection[chainID]
				g.lock.RUnlock()
				le.Yield()
			}
			logger.Info("Elected as a leader, starting delivery service for channel", chainID)
			if err := g.deliveryService[chainID].StartDeliverForChannel(chainID, committer, yield); err != nil {
				logger.Errorf("Delivery service is not able to start blocks delivery for chain, due to %+v", err)
			}
		} else {
			logger.Info("Renounced leadership, stopping delivery service for channel", chainID)
			if err := g.deliveryService[chainID].StopDeliverForChannel(chainID); err != nil {
				logger.Errorf("Delivery service is not able to stop blocks delivery for chain, due to %+v", err)
			}

		}

	}
}

func orgListFromConfig(config Config) []string {
	var orgList []string
	for _, appOrg := range config.Organizations() {
		orgList = append(orgList, appOrg.MSPID())
	}
	return orgList
}
