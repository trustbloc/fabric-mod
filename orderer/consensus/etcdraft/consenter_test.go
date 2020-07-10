/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package etcdraft_test

import (
	"encoding/pem"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/orderer"
	etcdraftproto "github.com/hyperledger/fabric-protos-go/orderer/etcdraft"
	"github.com/hyperledger/fabric/bccsp/sw"
	"github.com/hyperledger/fabric/common/channelconfig"
	"github.com/hyperledger/fabric/common/crypto/tlsgen"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/internal/pkg/comm"
	"github.com/hyperledger/fabric/orderer/common/cluster"
	clustermocks "github.com/hyperledger/fabric/orderer/common/cluster/mocks"
	"github.com/hyperledger/fabric/orderer/common/multichannel"
	"github.com/hyperledger/fabric/orderer/common/types"
	"github.com/hyperledger/fabric/orderer/consensus/etcdraft"
	"github.com/hyperledger/fabric/orderer/consensus/etcdraft/mocks"
	"github.com/hyperledger/fabric/orderer/consensus/follower"
	consensusmocks "github.com/hyperledger/fabric/orderer/consensus/mocks"
	"github.com/hyperledger/fabric/protoutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

//go:generate counterfeiter -o mocks/orderer_capabilities.go --fake-name OrdererCapabilities . ordererCapabilities

type ordererCapabilities interface {
	channelconfig.OrdererCapabilities
}

//go:generate counterfeiter -o mocks/orderer_config.go --fake-name OrdererConfig . ordererConfig

type ordererConfig interface {
	channelconfig.Orderer
}

var _ = Describe("Consenter", func() {
	var (
		certAsPEM   []byte
		chainGetter *mocks.ChainGetter
		support     *consensusmocks.FakeConsenterSupport
		dataDir     string
		snapDir     string
		walDir      string
		err         error

		joinBlock      *common.Block
		configMetaData *etcdraftproto.ConfigMetadata
	)

	BeforeEach(func() {
		certAsPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("cert bytes")})
		chainGetter = &mocks.ChainGetter{}
		support = &consensusmocks.FakeConsenterSupport{}
		dataDir, err = ioutil.TempDir("", "snap-")
		Expect(err).NotTo(HaveOccurred())
		walDir = path.Join(dataDir, "wal-")
		snapDir = path.Join(dataDir, "snap-")

		blockBytes, err := ioutil.ReadFile("testdata/mychannel.block")
		Expect(err).NotTo(HaveOccurred())

		goodConfigBlock := &common.Block{}
		proto.Unmarshal(blockBytes, goodConfigBlock)

		lastBlock := &common.Block{
			Header: &common.BlockHeader{
				Number: 1,
			},
			Data: goodConfigBlock.Data,
			Metadata: &common.BlockMetadata{
				Metadata: [][]byte{{}, protoutil.MarshalOrPanic(&common.Metadata{
					Value: protoutil.MarshalOrPanic(&common.LastConfig{Index: 0}),
				})},
			},
		}

		support.BlockReturns(lastBlock)
		joinBlock = lastBlock
	})

	AfterEach(func() {
		os.RemoveAll(dataDir)
	})

	When("the consenter is extracting the channel", func() {
		It("extracts successfully from step requests", func() {
			consenter := newConsenter(chainGetter)
			ch := consenter.TargetChannel(&orderer.ConsensusRequest{Channel: "mychannel"})
			Expect(ch).To(BeIdenticalTo("mychannel"))
		})
		It("extracts successfully from submit requests", func() {
			consenter := newConsenter(chainGetter)
			ch := consenter.TargetChannel(&orderer.SubmitRequest{Channel: "mychannel"})
			Expect(ch).To(BeIdenticalTo("mychannel"))
		})
		It("returns an empty string for the rest of the messages", func() {
			consenter := newConsenter(chainGetter)
			ch := consenter.TargetChannel(&common.Block{})
			Expect(ch).To(BeEmpty())
		})
	})

	When("the consenter is asked for a chain", func() {
		cryptoProvider, _ := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
		chainInstance := &etcdraft.Chain{CryptoProvider: cryptoProvider}
		cs := &multichannel.ChainSupport{
			Chain: chainInstance,
			BCCSP: cryptoProvider,
		}
		BeforeEach(func() {
			chainGetter.On("GetChain", "mychannel").Return(cs)
			chainGetter.On("GetChain", "badChainObject").Return(&multichannel.ChainSupport{})
			chainGetter.On("GetChain", "notmychannel").Return(nil)
			chainGetter.On("GetChain", "notraftchain").Return(&multichannel.ChainSupport{
				Chain: &multichannel.ChainSupport{},
			})
		})
		It("calls the chain getter and returns the reference when it is found", func() {
			consenter := newConsenter(chainGetter)
			Expect(consenter).NotTo(BeNil())

			chain := consenter.ReceiverByChain("mychannel")
			Expect(chain).NotTo(BeNil())
			Expect(chain).To(BeIdenticalTo(chainInstance))
		})
		It("calls the chain getter and returns nil when it's not found", func() {
			consenter := newConsenter(chainGetter)
			Expect(consenter).NotTo(BeNil())

			chain := consenter.ReceiverByChain("notmychannel")
			Expect(chain).To(BeNil())
		})
		It("calls the chain getter and returns nil when it's not a raft chain", func() {
			consenter := newConsenter(chainGetter)
			Expect(consenter).NotTo(BeNil())

			chain := consenter.ReceiverByChain("notraftchain")
			Expect(chain).To(BeNil())
		})
		It("calls the chain getter and panics when the chain has a bad internal state", func() {
			consenter := newConsenter(chainGetter)
			Expect(consenter).NotTo(BeNil())

			Expect(func() {
				consenter.ReceiverByChain("badChainObject")
			}).To(Panic())
		})
	})

	It("successfully constructs a Chain", func() {
		// We append a line feed to our cert, just to ensure that we can still consume it and ignore.
		certAsPEMWithLineFeed := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("cert bytes")})
		certAsPEMWithLineFeed = append(certAsPEMWithLineFeed, []byte("\n")...)
		m := &etcdraftproto.ConfigMetadata{
			Consenters: []*etcdraftproto.Consenter{
				{ServerTlsCert: certAsPEMWithLineFeed},
			},
			Options: &etcdraftproto.Options{
				TickInterval:      "500ms",
				ElectionTick:      10,
				HeartbeatTick:     1,
				MaxInflightBlocks: 5,
			},
		}
		metadata := protoutil.MarshalOrPanic(m)
		mockOrderer := &mocks.OrdererConfig{}
		mockOrderer.ConsensusMetadataReturns(metadata)
		mockOrderer.BatchSizeReturns(
			&orderer.BatchSize{
				PreferredMaxBytes: 2 * 1024 * 1024,
			},
		)
		support.SharedConfigReturns(mockOrderer)

		consenter := newConsenter(chainGetter)
		consenter.EtcdRaftConfig.WALDir = walDir
		consenter.EtcdRaftConfig.SnapDir = snapDir
		// consenter.EtcdRaftConfig.EvictionSuspicion is missing
		var defaultSuspicionFallback bool
		consenter.Metrics = newFakeMetrics(newFakeMetricsFields())
		consenter.Logger = consenter.Logger.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
			if strings.Contains(entry.Message, "EvictionSuspicion not set, defaulting to 10m0s") {
				defaultSuspicionFallback = true
			}
			return nil
		}))

		chain, err := consenter.HandleChain(support, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(chain).NotTo(BeNil())

		Expect(chain.Start).NotTo(Panic())
		Expect(defaultSuspicionFallback).To(BeTrue())
	})

	It("successfully constructs a Chain without a system channel", func() {
		// We append a line feed to our cert, just to ensure that we can still consume it and ignore.
		certAsPEMWithLineFeed := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("cert bytes")})
		certAsPEMWithLineFeed = append(certAsPEMWithLineFeed, []byte("\n")...)
		m := &etcdraftproto.ConfigMetadata{
			Consenters: []*etcdraftproto.Consenter{
				{ServerTlsCert: certAsPEMWithLineFeed},
			},
			Options: &etcdraftproto.Options{
				TickInterval:      "500ms",
				ElectionTick:      10,
				HeartbeatTick:     1,
				MaxInflightBlocks: 5,
			},
		}
		metadata := protoutil.MarshalOrPanic(m)
		mockOrderer := &mocks.OrdererConfig{}
		mockOrderer.ConsensusMetadataReturns(metadata)
		mockOrderer.BatchSizeReturns(
			&orderer.BatchSize{
				PreferredMaxBytes: 2 * 1024 * 1024,
			},
		)
		support.SharedConfigReturns(mockOrderer)

		consenter := newConsenter(chainGetter)
		consenter.EtcdRaftConfig.WALDir = walDir
		consenter.EtcdRaftConfig.SnapDir = snapDir
		//without a system channel, the InactiveChainRegistry is nil
		consenter.InactiveChainRegistry = nil
		consenter.icr = nil

		// consenter.EtcdRaftConfig.EvictionSuspicion is missing
		var defaultSuspicionFallback bool
		consenter.Metrics = newFakeMetrics(newFakeMetricsFields())
		consenter.Logger = consenter.Logger.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
			if strings.Contains(entry.Message, "EvictionSuspicion not set, defaulting to 10m0s") {
				defaultSuspicionFallback = true
			}
			return nil
		}))

		chain, err := consenter.HandleChain(support, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(chain).NotTo(BeNil())

		Expect(chain.Start).NotTo(Panic())
		Expect(defaultSuspicionFallback).To(BeTrue())
		Expect(chain.Halt).NotTo(Panic())
	})

	It("fails to handle chain if no matching cert found", func() {
		m := &etcdraftproto.ConfigMetadata{
			Consenters: []*etcdraftproto.Consenter{
				{ServerTlsCert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("foo")})},
			},
			Options: &etcdraftproto.Options{
				TickInterval:      "500ms",
				ElectionTick:      10,
				HeartbeatTick:     1,
				MaxInflightBlocks: 5,
			},
		}
		metadata := protoutil.MarshalOrPanic(m)
		support := &consensusmocks.FakeConsenterSupport{}
		mockOrderer := &mocks.OrdererConfig{}
		mockOrderer.ConsensusMetadataReturns(metadata)
		mockOrderer.BatchSizeReturns(
			&orderer.BatchSize{
				PreferredMaxBytes: 2 * 1024 * 1024,
			},
		)
		support.SharedConfigReturns(mockOrderer)
		support.ChannelIDReturns("foo")

		consenter := newConsenter(chainGetter)

		chain, err := consenter.HandleChain(support, &common.Metadata{})
		Expect(chain).To(Not(BeNil()))
		Expect(err).To(Not(HaveOccurred()))
		Expect(chain.Order(nil, 0).Error()).To(Equal("channel foo is not serviced by me"))
		consenter.icr.AssertNumberOfCalls(testingInstance, "TrackChain", 1)
	})

	It("fails to handle chain if etcdraft options have not been provided", func() {
		m := &etcdraftproto.ConfigMetadata{
			Consenters: []*etcdraftproto.Consenter{
				{ServerTlsCert: []byte("cert.orderer1.org1")},
			},
		}
		metadata := protoutil.MarshalOrPanic(m)
		mockOrderer := &mocks.OrdererConfig{}
		mockOrderer.ConsensusMetadataReturns(metadata)
		mockOrderer.BatchSizeReturns(
			&orderer.BatchSize{
				PreferredMaxBytes: 2 * 1024 * 1024,
			},
		)
		support.SharedConfigReturns(mockOrderer)

		consenter := newConsenter(chainGetter)

		chain, err := consenter.HandleChain(support, nil)
		Expect(chain).To(BeNil())
		Expect(err).To(MatchError("etcdraft options have not been provided"))
	})

	It("fails to handle chain if tick interval is invalid", func() {
		m := &etcdraftproto.ConfigMetadata{
			Consenters: []*etcdraftproto.Consenter{
				{ServerTlsCert: certAsPEM},
			},
			Options: &etcdraftproto.Options{
				TickInterval:      "500",
				ElectionTick:      10,
				HeartbeatTick:     1,
				MaxInflightBlocks: 5,
			},
		}
		metadata := protoutil.MarshalOrPanic(m)
		mockOrderer := &mocks.OrdererConfig{}
		mockOrderer.ConsensusMetadataReturns(metadata)
		mockOrderer.BatchSizeReturns(
			&orderer.BatchSize{
				PreferredMaxBytes: 2 * 1024 * 1024,
			},
		)
		mockOrderer.CapabilitiesReturns(&mocks.OrdererCapabilities{})
		support.SharedConfigReturns(mockOrderer)

		consenter := newConsenter(chainGetter)

		chain, err := consenter.HandleChain(support, nil)
		Expect(chain).To(BeNil())
		Expect(err).To(MatchError("failed to parse TickInterval (500) to time duration"))
	})

	It("constructs a follower chain if no matching cert found", func() {
		m := &etcdraftproto.ConfigMetadata{
			Consenters: []*etcdraftproto.Consenter{
				{ServerTlsCert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("foo")})},
			},
			Options: &etcdraftproto.Options{
				TickInterval:      "500ms",
				ElectionTick:      10,
				HeartbeatTick:     1,
				MaxInflightBlocks: 5,
			},
		}
		metadata := protoutil.MarshalOrPanic(m)
		support := &consensusmocks.FakeConsenterSupport{}
		mockOrderer := &mocks.OrdererConfig{}
		mockOrderer.ConsensusMetadataReturns(metadata)
		mockOrderer.BatchSizeReturns(
			&orderer.BatchSize{
				PreferredMaxBytes: 2 * 1024 * 1024,
			},
		)
		support.SharedConfigReturns(mockOrderer)
		support.ChannelIDReturns("foo")

		consenter := newConsenter(chainGetter)
		//without a system channel, the InactiveChainRegistry is nil
		consenter.InactiveChainRegistry = nil
		consenter.icr = nil

		chain, err := consenter.HandleChain(support, &common.Metadata{})
		Expect(chain).To(Not(BeNil()))
		Expect(err).To(Not(HaveOccurred()))
		Expect(chain.Order(nil, 0).Error()).To(Equal("orderer is a follower of channel foo"))
		_, ok := chain.(*follower.Chain)
		Expect(ok).To(BeTrue())
	})

	When("Join Chain", func() {
		BeforeEach(func() {
			ca, err := tlsgen.NewCA()
			Expect(err).NotTo(HaveOccurred())
			pair, err := ca.NewServerCertKeyPair("o1.example.com")
			Expect(err).NotTo(HaveOccurred())
			configMetaData = &etcdraftproto.ConfigMetadata{
				Consenters: []*etcdraftproto.Consenter{
					{
						Host:          "o1.example.com",
						Port:          10666,
						ClientTlsCert: pair.Cert,
						ServerTlsCert: pair.Cert,
					},
				},
				Options: &etcdraftproto.Options{
					TickInterval:      "500ms",
					ElectionTick:      10,
					HeartbeatTick:     1,
					MaxInflightBlocks: 5,
				},
			}
		})

		It("Successfully constructs a follower.Chain", func() {
			metadata := protoutil.MarshalOrPanic(configMetaData)
			mockOrderer := &mocks.OrdererConfig{}
			mockOrderer.ConsensusMetadataReturns(metadata)
			support.SharedConfigReturns(mockOrderer)
			support.ChannelIDReturns("foo")

			consenter := newConsenter(chainGetter)
			consenter.icr = nil
			consenter.InactiveChainRegistry = nil

			chain, err := consenter.JoinChain(support, joinBlock)
			Expect(err).NotTo(HaveOccurred())
			Expect(chain).NotTo(BeNil())

			Expect(chain.Order(nil, 0).Error()).To(Equal("orderer is a follower of channel foo"))
			followerChain, ok := chain.(*follower.Chain)
			Expect(ok).To(BeTrue())
			cRel, st := followerChain.StatusReport()
			Expect(cRel).To(Equal(types.ClusterRelationFollower))
			Expect(st).To(Equal(types.StatusOnBoarding))

			Expect(chain.Start).NotTo(Panic())
			Expect(chain.Halt).NotTo(Panic())
		})

		It("Fails when metadata is bad - no consenters", func() {
			configMetaData.Consenters = nil
			metadata := protoutil.MarshalOrPanic(configMetaData)
			mockOrderer := &mocks.OrdererConfig{}
			mockOrderer.ConsensusMetadataReturns(metadata)
			support.SharedConfigReturns(mockOrderer)
			support.ChannelIDReturns("foo")

			consenter := newConsenter(chainGetter)
			consenter.icr = nil
			consenter.InactiveChainRegistry = nil

			chain, err := consenter.JoinChain(support, joinBlock)
			Expect(err.Error()).To(Equal("error checking etcdraft.ConfigMetadata: empty consenter set"))
			Expect(chain).To(BeNil())
		})

		It("Fails when metadata is bad - bad certificate", func() {
			configMetaData.Consenters[0].ClientTlsCert = []byte("not a certificate")
			metadata := protoutil.MarshalOrPanic(configMetaData)
			mockOrderer := &mocks.OrdererConfig{}
			mockOrderer.ConsensusMetadataReturns(metadata)
			support.SharedConfigReturns(mockOrderer)
			support.ChannelIDReturns("foo")

			consenter := newConsenter(chainGetter)
			consenter.icr = nil
			consenter.InactiveChainRegistry = nil

			chain, err := consenter.JoinChain(support, joinBlock)
			Expect(err.Error()).To(Equal("error checking etcdraft.ConfigMetadata: client TLS certificate is not PEM encoded: not a certificate"))
			Expect(chain).To(BeNil())
		})

		It("Fails when metadata is bad - no options", func() {
			configMetaData.Options = nil
			metadata := protoutil.MarshalOrPanic(configMetaData)
			mockOrderer := &mocks.OrdererConfig{}
			mockOrderer.ConsensusMetadataReturns(metadata)
			support.SharedConfigReturns(mockOrderer)
			support.ChannelIDReturns("foo")

			consenter := newConsenter(chainGetter)
			consenter.icr = nil
			consenter.InactiveChainRegistry = nil

			chain, err := consenter.JoinChain(support, joinBlock)
			Expect(err.Error()).To(Equal("error checking etcdraft.ConfigMetadata: nil Raft config metadata options"))
			Expect(chain).To(BeNil())
		})

		It("Fails when it cannot invoke a membership predicate", func() {
			metadata := protoutil.MarshalOrPanic(configMetaData)
			mockOrderer := &mocks.OrdererConfig{}
			mockOrderer.ConsensusMetadataReturns(metadata)
			support.SharedConfigReturns(mockOrderer)
			support.ChannelIDReturns("foo")

			consenter := newConsenter(chainGetter)
			consenter.icr = nil
			consenter.InactiveChainRegistry = nil

			chain, err := consenter.JoinChain(support, &common.Block{}) // This will fail the membership predicate invocation
			Expect(err.Error()).To(Equal("error checking if the consenter is a member of the channel using the join-block: block data is nil"))
			Expect(chain).To(BeNil())
		})

		It("Fails when it cannot build a block puller", func() {
			metadata := protoutil.MarshalOrPanic(configMetaData)
			mockOrderer := &mocks.OrdererConfig{}
			mockOrderer.ConsensusMetadataReturns(metadata)
			support.SharedConfigReturns(mockOrderer)
			support.ChannelIDReturns("foo")

			consenter := newConsenter(chainGetter)
			consenter.icr = nil
			consenter.InactiveChainRegistry = nil
			consenter.Dialer.Config.SecOpts.Certificate = []byte("not-a-certificate") // this will cause a failure in block puller creation

			chain, err := consenter.JoinChain(support, joinBlock)
			Expect(err.Error()).To(Equal("error creating a block puller from join-block: client certificate isn't in PEM format: not-a-certificate"))
			Expect(chain).To(BeNil())
		})

	})
})

type consenter struct {
	*etcdraft.Consenter
	icr *mocks.InactiveChainRegistry
}

func newConsenter(chainGetter *mocks.ChainGetter) *consenter {
	communicator := &clustermocks.Communicator{}
	ca, err := tlsgen.NewCA()
	Expect(err).NotTo(HaveOccurred())
	communicator.On("Configure", mock.Anything, mock.Anything)
	icr := &mocks.InactiveChainRegistry{}
	icr.On("TrackChain", "foo", mock.Anything, mock.Anything)
	certAsPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("cert bytes")})

	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	Expect(err).NotTo(HaveOccurred())

	c := &etcdraft.Consenter{
		InactiveChainRegistry: icr,
		Communication:         communicator,
		Cert:                  certAsPEM,
		Logger:                flogging.MustGetLogger("test"),
		Chains:                chainGetter,
		Dispatcher: &etcdraft.Dispatcher{
			Logger:        flogging.MustGetLogger("test"),
			ChainSelector: &mocks.ReceiverGetter{},
		},
		Dialer: &cluster.PredicateDialer{
			Config: comm.ClientConfig{
				SecOpts: comm.SecureOptions{
					Certificate: ca.CertBytes(),
				},
			},
		},
		BCCSP: cryptoProvider,
	}
	return &consenter{
		Consenter: c,
		icr:       icr,
	}
}
