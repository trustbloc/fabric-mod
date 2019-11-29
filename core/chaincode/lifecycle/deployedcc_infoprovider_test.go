/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package lifecycle_test

import (
	"fmt"

	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/ledger/rwset/kvrwset"
	"github.com/hyperledger/fabric-protos-go/msp"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	lb "github.com/hyperledger/fabric-protos-go/peer/lifecycle"
	"github.com/hyperledger/fabric/common/channelconfig"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/core/chaincode/lifecycle"
	"github.com/hyperledger/fabric/core/chaincode/lifecycle/mock"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/protoutil"

	"github.com/golang/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ValidatorCommitter", func() {
	var (
		vc                      *lifecycle.ValidatorCommitter
		resources               *lifecycle.Resources
		fakeLegacyProvider      *mock.LegacyDeployedCCInfoProvider
		fakeQueryExecutor       *mock.SimpleQueryExecutor
		fakeChannelConfigSource *mock.ChannelConfigSource
		fakeChannelConfig       *mock.ChannelConfig
		fakeApplicationConfig   *mock.ApplicationConfig
		fakeOrgConfigs          []*mock.ApplicationOrgConfig
		fakePolicyManager       *mock.PolicyManager

		fakePublicState MapLedgerShim
	)

	BeforeEach(func() {
		fakeLegacyProvider = &mock.LegacyDeployedCCInfoProvider{}
		fakeChannelConfigSource = &mock.ChannelConfigSource{}
		fakeChannelConfig = &mock.ChannelConfig{}
		fakeChannelConfigSource.GetStableChannelConfigReturns(fakeChannelConfig)
		fakeApplicationConfig = &mock.ApplicationConfig{}
		fakeChannelConfig.ApplicationConfigReturns(fakeApplicationConfig, true)
		fakeOrgConfigs = []*mock.ApplicationOrgConfig{{}, {}}
		fakeOrgConfigs[0].MSPIDReturns("first-mspid")
		fakeOrgConfigs[1].MSPIDReturns("second-mspid")
		fakePolicyManager = &mock.PolicyManager{}
		fakePolicyManager.GetPolicyReturns(nil, true)
		fakeChannelConfig.PolicyManagerReturns(fakePolicyManager)

		fakeApplicationConfig.OrganizationsReturns(map[string]channelconfig.ApplicationOrg{
			"org0": fakeOrgConfigs[0],
			"org1": fakeOrgConfigs[1],
		})

		resources = &lifecycle.Resources{
			ChannelConfigSource: fakeChannelConfigSource,
			Serializer:          &lifecycle.Serializer{},
		}

		vc = &lifecycle.ValidatorCommitter{
			Resources:                    resources,
			LegacyDeployedCCInfoProvider: fakeLegacyProvider,
		}

		fakePublicState = MapLedgerShim(map[string][]byte{})
		fakeQueryExecutor = &mock.SimpleQueryExecutor{}
		fakeQueryExecutor.GetStateStub = func(namespace, key string) ([]byte, error) {
			return fakePublicState.GetState(key)
		}

		err := resources.Serializer.Serialize(lifecycle.NamespacesName, "cc-name", &lifecycle.ChaincodeDefinition{
			EndorsementInfo: &lb.ChaincodeEndorsementInfo{
				Version: "version",
			},
			ValidationInfo: &lb.ChaincodeValidationInfo{
				ValidationPlugin:    "validation-plugin",
				ValidationParameter: []byte("validation-parameter"),
			},
			Collections: &pb.CollectionConfigPackage{
				Config: []*pb.CollectionConfig{
					{
						Payload: &pb.CollectionConfig_StaticCollectionConfig{
							StaticCollectionConfig: &pb.StaticCollectionConfig{
								Name: "collection-name",
							},
						},
					},
				},
			},
		}, fakePublicState)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Namespaces", func() {
		BeforeEach(func() {
			fakeLegacyProvider.NamespacesReturns([]string{"a", "b", "c"})
		})

		It("appends its own namespaces the legacy impl", func() {
			res := vc.Namespaces()
			Expect(res).To(Equal([]string{"_lifecycle", "a", "b", "c"}))
			Expect(fakeLegacyProvider.NamespacesCallCount()).To(Equal(1))
		})
	})

	Describe("UpdatedChaincodes", func() {
		var (
			updates map[string][]*kvrwset.KVWrite
		)

		BeforeEach(func() {
			updates = map[string][]*kvrwset.KVWrite{
				"_lifecycle": {
					{Key: "some/random/value"},
					{Key: "namespaces/fields/cc-name/Sequence"},
					{Key: "prefix/namespaces/fields/cc-name/Sequence"},
					{Key: "namespaces/fields/Sequence/infix"},
					{Key: "namespaces/fields/cc-name/Sequence/Postfix"},
				},
				"other-namespace": nil,
			}
			fakeLegacyProvider.UpdatedChaincodesReturns([]*ledger.ChaincodeLifecycleInfo{
				{Name: "foo"},
				{Name: "bar"},
			}, nil)
		})

		It("checks its own namespace, then passes through to the legacy impl", func() {
			res, err := vc.UpdatedChaincodes(updates)
			Expect(res).To(Equal([]*ledger.ChaincodeLifecycleInfo{
				{Name: "cc-name"},
				{Name: "foo"},
				{Name: "bar"},
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeLegacyProvider.UpdatedChaincodesCallCount()).To(Equal(1))
			Expect(fakeLegacyProvider.UpdatedChaincodesArgsForCall(0)).To(Equal(updates))
		})

		Context("when the legacy provider returns an error", func() {
			BeforeEach(func() {
				fakeLegacyProvider.UpdatedChaincodesReturns(nil, fmt.Errorf("legacy-error"))
			})

			It("wraps and returns the error", func() {
				_, err := vc.UpdatedChaincodes(updates)
				Expect(err).To(MatchError("error invoking legacy deployed cc info provider: legacy-error"))
			})
		})
	})

	Describe("ChaincodeInfo", func() {
		It("returns the info found in the new lifecycle", func() {
			res, err := vc.ChaincodeInfo("channel-name", "cc-name", fakeQueryExecutor)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Name).To(Equal("cc-name"))
			Expect(res.Version).To(Equal("version"))
			Expect(res.Hash).To(Equal(util.ComputeSHA256([]byte("cc-name:version"))))
			Expect(len(res.ExplicitCollectionConfigPkg.Config)).To(Equal(1))
		})

		Context("when the requested chaincode is _lifecycle", func() {
			It("returns the implicit collections only", func() {
				res, err := vc.ChaincodeInfo("channel-name", "_lifecycle", fakeQueryExecutor)
				Expect(err).NotTo(HaveOccurred())
				Expect(res.Name).To(Equal("_lifecycle"))
				Expect(res.ExplicitCollectionConfigPkg).To(BeNil())
			})
		})

		Context("when the ledger returns an error", func() {
			BeforeEach(func() {
				fakeQueryExecutor.GetStateReturns(nil, fmt.Errorf("state-error"))
			})

			It("wraps and returns the error", func() {
				_, err := vc.ChaincodeInfo("channel-name", "cc-name", fakeQueryExecutor)
				Expect(err).To(MatchError("could not get info about chaincode: could not deserialize metadata for chaincode cc-name: could not query metadata for namespace namespaces/cc-name: state-error"))
			})
		})

		Context("when the chaincode cannot be found in the new lifecycle", func() {
			BeforeEach(func() {
				fakeLegacyProvider.ChaincodeInfoReturns(&ledger.DeployedChaincodeInfo{
					Name:    "legacy-name",
					Hash:    []byte("hash"),
					Version: "cc-version",
				}, fmt.Errorf("chaincode-info-error"))
			})

			It("passes through to the legacy impl", func() {
				res, err := vc.ChaincodeInfo("channel-name", "legacy-name", fakeQueryExecutor)
				Expect(res).To(Equal(&ledger.DeployedChaincodeInfo{
					Name:    "legacy-name",
					Hash:    []byte("hash"),
					Version: "cc-version",
				}))
				Expect(err).To(MatchError("chaincode-info-error"))
				Expect(fakeLegacyProvider.ChaincodeInfoCallCount()).To(Equal(1))
				channelID, ccName, qe := fakeLegacyProvider.ChaincodeInfoArgsForCall(0)
				Expect(channelID).To(Equal("channel-name"))
				Expect(ccName).To(Equal("legacy-name"))
				Expect(qe).To(Equal(fakeQueryExecutor))
			})
		})

		Context("when the data is corrupt", func() {
			BeforeEach(func() {
				fakePublicState["namespaces/fields/cc-name/ValidationInfo"] = []byte("garbage")
			})

			It("wraps and returns that error", func() {
				_, err := vc.ChaincodeInfo("channel-name", "cc-name", fakeQueryExecutor)
				Expect(err).To(MatchError("could not get info about chaincode: could not deserialize chaincode definition for chaincode cc-name: could not unmarshal state for key namespaces/fields/cc-name/ValidationInfo: proto: can't skip unknown wire type 7"))
			})
		})
	})

	Describe("CollectionInfo", func() {
		It("returns the collection info as defined in the new lifecycle", func() {
			res, err := vc.CollectionInfo("channel-name", "cc-name", "collection-name", fakeQueryExecutor)
			Expect(err).NotTo(HaveOccurred())
			Expect(proto.Equal(res, &pb.StaticCollectionConfig{
				Name: "collection-name",
			})).To(BeTrue())
		})

		Context("when no matching collection is found", func() {
			It("returns nil", func() {
				res, err := vc.CollectionInfo("channel-name", "cc-name", "non-extant-name", fakeQueryExecutor)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).To(BeNil())
			})
		})

		Context("when the chaincode in question is _lifecycle", func() {
			It("skips the existence checks and checks the implicit collections", func() {
				res, err := vc.CollectionInfo("channel-name", "_lifecycle", "_implicit_org_first-mspid", fakeQueryExecutor)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).NotTo(BeNil())
			})
		})

		Context("when the ledger returns an error", func() {
			BeforeEach(func() {
				fakeQueryExecutor.GetStateReturns(nil, fmt.Errorf("state-error"))
			})

			It("wraps and returns the error", func() {
				_, err := vc.CollectionInfo("channel-name", "cc-name", "collection-name", fakeQueryExecutor)
				Expect(err).To(MatchError("could not get chaincode: could not deserialize metadata for chaincode cc-name: could not query metadata for namespace namespaces/cc-name: state-error"))
			})
		})

		Context("when the chaincode is not in the new lifecycle", func() {
			var (
				collInfo *pb.StaticCollectionConfig
			)

			BeforeEach(func() {
				collInfo = &pb.StaticCollectionConfig{}
				fakeLegacyProvider.CollectionInfoReturns(collInfo, fmt.Errorf("collection-info-error"))
			})

			It("passes through to the legacy impl", func() {
				res, err := vc.CollectionInfo("channel-name", "legacy-name", "collection-name", fakeQueryExecutor)
				Expect(res).To(Equal(collInfo))
				Expect(err).To(MatchError("collection-info-error"))
				Expect(fakeLegacyProvider.CollectionInfoCallCount()).To(Equal(1))
				channelID, ccName, collName, qe := fakeLegacyProvider.CollectionInfoArgsForCall(0)
				Expect(channelID).To(Equal("channel-name"))
				Expect(ccName).To(Equal("legacy-name"))
				Expect(collName).To(Equal("collection-name"))
				Expect(qe).To(Equal(fakeQueryExecutor))
			})
		})

		Context("when the data is corrupt", func() {
			BeforeEach(func() {
				fakePublicState["namespaces/fields/cc-name/ValidationInfo"] = []byte("garbage")
			})

			It("wraps and returns that error", func() {
				_, err := vc.CollectionInfo("channel-name", "cc-name", "collection-name", fakeQueryExecutor)
				Expect(err).To(MatchError("could not get chaincode: could not deserialize chaincode definition for chaincode cc-name: could not unmarshal state for key namespaces/fields/cc-name/ValidationInfo: proto: can't skip unknown wire type 7"))
			})
		})
	})

	Describe("ImplicitCollections", func() {
		It("returns an implicit collection for every org", func() {
			res, err := vc.ImplicitCollections("channel-id", "cc-name", fakeQueryExecutor)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(res)).To(Equal(2))
			var firstOrg, secondOrg *pb.StaticCollectionConfig
			for _, collection := range res {
				switch collection.Name {
				case "_implicit_org_first-mspid":
					firstOrg = collection
				case "_implicit_org_second-mspid":
					secondOrg = collection
				}
			}
			Expect(firstOrg).NotTo(BeNil())
			Expect(secondOrg).NotTo(BeNil())
		})

		Context("when the chaincode does not exist", func() {
			It("returns nil, nil", func() {
				res, err := vc.ImplicitCollections("channel-id", "missing-name", fakeQueryExecutor)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).To(BeNil())
			})
		})

		Context("when the ledger returns an error", func() {
			BeforeEach(func() {
				fakeQueryExecutor.GetStateReturns(nil, fmt.Errorf("state-error"))
			})

			It("wraps and returns the error", func() {
				_, err := vc.ImplicitCollections("channel-id", "missing-name", fakeQueryExecutor)
				Expect(err).To(MatchError("could not get info about chaincode: could not deserialize metadata for chaincode missing-name: could not query metadata for namespace namespaces/missing-name: state-error"))
			})
		})

		Context("when there is no channel config", func() {
			BeforeEach(func() {
				fakeChannelConfigSource.GetStableChannelConfigReturns(nil)
			})

			It("returns an error", func() {
				_, err := vc.ImplicitCollections("channel-id", "cc-name", fakeQueryExecutor)
				Expect(err).To(MatchError("could not get channelconfig for channel channel-id"))
			})
		})

		Context("when there is no application config", func() {
			BeforeEach(func() {
				fakeChannelConfig.ApplicationConfigReturns(nil, false)
			})

			It("returns an error", func() {
				_, err := vc.ImplicitCollections("channel-id", "cc-name", fakeQueryExecutor)
				Expect(err).To(MatchError("could not get application config for channel channel-id"))
			})
		})
	})

	Describe("AllCollectionsConfigPkg", func() {
		It("returns the collection config package that includes both explicit and implicit collections as defined in the new lifecycle", func() {
			ccPkg, err := vc.AllCollectionsConfigPkg("channel-name", "cc-name", fakeQueryExecutor)
			Expect(err).NotTo(HaveOccurred())
			collectionNames := []string{}
			for _, config := range ccPkg.Config {
				collectionNames = append(collectionNames, config.GetStaticCollectionConfig().GetName())
			}
			Expect(collectionNames).Should(ConsistOf("collection-name", "_implicit_org_first-mspid", "_implicit_org_second-mspid"))
		})

		Context("when no explicit collection config is defined", func() {
			BeforeEach(func() {
				err := resources.Serializer.Serialize(lifecycle.NamespacesName, "cc-without-explicit-collection",
					&lifecycle.ChaincodeDefinition{
						EndorsementInfo: &lb.ChaincodeEndorsementInfo{
							Version: "version",
						},
						ValidationInfo: &lb.ChaincodeValidationInfo{
							ValidationPlugin:    "validation-plugin",
							ValidationParameter: []byte("validation-parameter"),
						},
					}, fakePublicState)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns only implicit collections", func() {
				ccPkg, err := vc.AllCollectionsConfigPkg("channel-name", "cc-without-explicit-collection", fakeQueryExecutor)
				Expect(err).NotTo(HaveOccurred())
				collectionNames := []string{}
				for _, config := range ccPkg.Config {
					collectionNames = append(collectionNames, config.GetStaticCollectionConfig().GetName())
				}
				Expect(collectionNames).Should(ConsistOf("_implicit_org_first-mspid", "_implicit_org_second-mspid"))
			})
		})

		Context("when the ledger returns an error", func() {
			BeforeEach(func() {
				fakeQueryExecutor.GetStateReturns(nil, fmt.Errorf("state-error"))
			})

			It("wraps and returns the error", func() {
				_, err := vc.AllCollectionsConfigPkg("channel-name", "cc-name", fakeQueryExecutor)
				Expect(err).To(MatchError("could not get info about chaincode: could not deserialize metadata for chaincode cc-name: could not query metadata for namespace namespaces/cc-name: state-error"))
			})
		})

		Context("when there is no channel config", func() {
			BeforeEach(func() {
				fakeChannelConfigSource.GetStableChannelConfigReturns(nil)
			})

			It("returns an error", func() {
				_, err := vc.AllCollectionsConfigPkg("channel-id", "cc-name", fakeQueryExecutor)
				Expect(err).To(MatchError("could not get channelconfig for channel channel-id"))
			})
		})

		Context("when there is no application config", func() {
			BeforeEach(func() {
				fakeChannelConfig.ApplicationConfigReturns(nil, false)
			})

			It("returns an error", func() {
				_, err := vc.AllCollectionsConfigPkg("channel-id", "cc-name", fakeQueryExecutor)
				Expect(err).To(MatchError("could not get application config for channel channel-id"))
			})
		})

		Context("when the chaincode is not in the new lifecycle", func() {
			var (
				ccPkg *pb.CollectionConfigPackage
			)

			BeforeEach(func() {
				ccPkg = &pb.CollectionConfigPackage{}
				fakeLegacyProvider.ChaincodeInfoReturns(
					&ledger.DeployedChaincodeInfo{
						ExplicitCollectionConfigPkg: ccPkg,
						IsLegacy:                    true,
					},
					nil,
				)
			})

			It("passes through to the legacy impl", func() {
				res, err := vc.AllCollectionsConfigPkg("channel-name", "legacy-name", fakeQueryExecutor)
				Expect(fakeLegacyProvider.ChaincodeInfoCallCount()).To(Equal(1))
				channelID, ccName, qe := fakeLegacyProvider.ChaincodeInfoArgsForCall(0)
				Expect(channelID).To(Equal("channel-name"))
				Expect(ccName).To(Equal("legacy-name"))
				Expect(qe).To(Equal(fakeQueryExecutor))
				Expect(res).To(Equal(ccPkg))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the chaincode is not in the new lifecycle and legacy info provider returns error", func() {
			BeforeEach(func() {
				fakeLegacyProvider.ChaincodeInfoReturns(nil, fmt.Errorf("legacy-chaincode-info-error"))
			})

			It("passes through to the legacy impl", func() {
				_, err := vc.AllCollectionsConfigPkg("channel-name", "legacy-name", fakeQueryExecutor)
				Expect(err).To(MatchError("legacy-chaincode-info-error"))
			})
		})

		Context("when the data is corrupt", func() {
			BeforeEach(func() {
				fakePublicState["namespaces/fields/cc-name/ValidationInfo"] = []byte("garbage")
			})

			It("wraps and returns that error", func() {
				_, err := vc.AllCollectionsConfigPkg("channel-name", "cc-name", fakeQueryExecutor)
				Expect(err).To(MatchError("could not get info about chaincode: could not deserialize chaincode definition for chaincode cc-name: could not unmarshal state for key namespaces/fields/cc-name/ValidationInfo: proto: can't skip unknown wire type 7"))
			})
		})
	})

	Describe("LifecycleEndorsementPolicyAsBytes", func() {
		It("returns the endorsement policy for the lifecycle chaincode", func() {
			b, err := vc.LifecycleEndorsementPolicyAsBytes("channel-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(b).NotTo(BeNil())
			policy := &cb.ApplicationPolicy{}
			err = proto.Unmarshal(b, policy)
			Expect(err).NotTo(HaveOccurred())
			Expect(policy.GetChannelConfigPolicyReference()).To(Equal("/Channel/Application/LifecycleEndorsement"))
		})

		Context("when the endorsement policy reference is not found", func() {
			BeforeEach(func() {
				fakePolicyManager.GetPolicyReturns(nil, false)
			})

			It("returns an error", func() {
				b, err := vc.LifecycleEndorsementPolicyAsBytes("channel-id")
				Expect(err).NotTo(HaveOccurred())
				policy := &cb.ApplicationPolicy{}
				err = proto.Unmarshal(b, policy)
				Expect(err).NotTo(HaveOccurred())
				Expect(policy.GetSignaturePolicy()).NotTo(BeNil())
			})

			Context("when the application config cannot be retrieved", func() {
				BeforeEach(func() {
					fakeChannelConfig.ApplicationConfigReturns(nil, false)
				})

				It("returns an error", func() {
					_, err := vc.LifecycleEndorsementPolicyAsBytes("channel-id")
					Expect(err).To(MatchError("could not get application config for channel 'channel-id'"))
				})
			})
		})

		Context("when the channel config cannot be retrieved", func() {
			BeforeEach(func() {
				fakeChannelConfigSource.GetStableChannelConfigReturns(nil)
			})

			It("returns an error", func() {
				_, err := vc.LifecycleEndorsementPolicyAsBytes("channel-id")
				Expect(err).To(MatchError("could not get channel config for channel 'channel-id'"))
			})
		})
	})

	Describe("ValidationInfo", func() {
		It("returns the validation info as defined in the new lifecycle", func() {
			vPlugin, vParm, uerr, verr := vc.ValidationInfo("channel-id", "cc-name", fakeQueryExecutor)
			Expect(uerr).NotTo(HaveOccurred())
			Expect(verr).NotTo(HaveOccurred())
			Expect(vPlugin).To(Equal("validation-plugin"))
			Expect(vParm).To(Equal([]byte("validation-parameter")))
		})

		Context("when the chaincode in question is _lifecycle", func() {
			It("returns the builtin plugin and the endorsement policy", func() {
				vPlugin, vParm, uerr, verr := vc.ValidationInfo("channel-id", "_lifecycle", fakeQueryExecutor)
				Expect(uerr).NotTo(HaveOccurred())
				Expect(verr).NotTo(HaveOccurred())
				Expect(vPlugin).To(Equal("vscc"))
				Expect(vParm).NotTo(BeNil())
			})

			Context("when fetching the lifecycle endorsement policy returns an error", func() {
				BeforeEach(func() {
					fakeChannelConfigSource.GetStableChannelConfigReturns(nil)
				})

				It("treats the error as non-deterministic", func() {
					_, _, uerr, _ := vc.ValidationInfo("channel-id", "_lifecycle", fakeQueryExecutor)
					Expect(uerr).To(MatchError("unexpected failure to create lifecycle endorsement policy: could not get channel config for channel 'channel-id'"))
				})
			})
		})

		Context("when the ledger returns an error", func() {
			BeforeEach(func() {
				fakeQueryExecutor.GetStateReturns(nil, fmt.Errorf("state-error"))
			})

			It("wraps and returns the error", func() {
				_, _, uerr, _ := vc.ValidationInfo("channel-id", "cc-name", fakeQueryExecutor)
				Expect(uerr).To(MatchError("could not get chaincode: could not deserialize metadata for chaincode cc-name: could not query metadata for namespace namespaces/cc-name: state-error"))
			})
		})

		Context("when the chaincode is not in the new lifecycle", func() {
			It("passes through to the legacy impl", func() {
				vPlugin, vParm, uerr, verr := vc.ValidationInfo("channel-id", "missing-name", fakeQueryExecutor)
				Expect(vPlugin).To(BeEmpty())
				Expect(vParm).To(BeNil())
				Expect(uerr).NotTo(HaveOccurred())
				Expect(verr).NotTo(HaveOccurred())
			})
		})

		Context("when the data is corrupt", func() {
			BeforeEach(func() {
				fakePublicState["namespaces/fields/cc-name/ValidationInfo"] = []byte("garbage")
			})

			It("wraps and returns that error", func() {
				_, _, uerr, _ := vc.ValidationInfo("channel-id", "cc-name", fakeQueryExecutor)
				Expect(uerr).To(MatchError("could not get chaincode: could not deserialize chaincode definition for chaincode cc-name: could not unmarshal state for key namespaces/fields/cc-name/ValidationInfo: proto: can't skip unknown wire type 7"))
			})
		})
	})

	Describe("CollectionValidationInfo", func() {
		var (
			fakeValidationState *mock.ValidationState
		)

		BeforeEach(func() {
			fakeValidationState = &mock.ValidationState{}
			fakeValidationState.GetStateMultipleKeysStub = func(namespace string, keys []string) ([][]byte, error) {
				return [][]byte{fakePublicState[keys[0]]}, nil
			}
		})

		It("returns the endorsement policy for the collection", func() {
			ep, uErr, vErr := vc.CollectionValidationInfo("channel-id", "cc-name", "collection-name", fakeValidationState)
			Expect(uErr).NotTo(HaveOccurred())
			Expect(vErr).NotTo(HaveOccurred())
			Expect(ep).To(Equal([]byte("validation-parameter")))
		})

		Context("when the chaincode definition cannot be retrieved", func() {
			BeforeEach(func() {
				fakeValidationState.GetStateMultipleKeysReturns(nil, fmt.Errorf("state-error"))
			})

			It("returns an unexpected error", func() {
				_, uErr, vErr := vc.CollectionValidationInfo("channel-id", "cc-name", "collection-name", fakeValidationState)
				Expect(vErr).NotTo(HaveOccurred())
				Expect(uErr).To(MatchError("could not get chaincode: could not deserialize metadata for chaincode cc-name: could not query metadata for namespace namespaces/cc-name: could not get state thought validatorstate shim: state-error"))
			})
		})

		Context("when the chaincode does not exist in the new lifecycle", func() {
			It("returns nil nil nil", func() {
				ep, uErr, vErr := vc.CollectionValidationInfo("channel-id", "missing-name", "collection-name", fakeValidationState)
				Expect(uErr).NotTo(HaveOccurred())
				Expect(vErr).NotTo(HaveOccurred())
				Expect(ep).To(BeNil())
			})
		})

		Context("when the collection does not exist", func() {
			It("returns a validation error", func() {
				_, uErr, vErr := vc.CollectionValidationInfo("channel-id", "cc-name", "missing-collection-name", fakeValidationState)
				Expect(uErr).NotTo(HaveOccurred())
				Expect(vErr).To(MatchError("no such collection 'missing-collection-name'"))
			})
		})

		Context("when the collection is an implicit collection", func() {
			It("returns the implicit endorsement policy", func() {
				ep, uErr, vErr := vc.CollectionValidationInfo("channel-id", "cc-name", "_implicit_org_first-mspid", fakeValidationState)
				Expect(uErr).NotTo(HaveOccurred())
				Expect(vErr).NotTo(HaveOccurred())
				Expect(ep).NotTo(Equal([]byte("validation-parameter")))
			})

			Context("when the implicit endorsement policy returns an error", func() {
				It("returns the error", func() {
					_, _, vErr := vc.CollectionValidationInfo("channel-id", "cc-name", "_implicit_org_bad-mspid", fakeValidationState)
					Expect(vErr).To(MatchError("no org found in channel with MSPID 'bad-mspid'"))
				})
			})
		})

		Context("when the endorsement policy is specified in the collection config", func() {
			var expectedPolicy *pb.ApplicationPolicy

			BeforeEach(func() {
				expectedPolicy = &pb.ApplicationPolicy{
					Type: &pb.ApplicationPolicy_SignaturePolicy{
						SignaturePolicy: &cb.SignaturePolicyEnvelope{
							Identities: []*msp.MSPPrincipal{
								{
									Principal: []byte("test"),
								},
							},
						},
					},
				}
				err := resources.Serializer.Serialize(lifecycle.NamespacesName, "cc-name", &lifecycle.ChaincodeDefinition{
					EndorsementInfo: &lb.ChaincodeEndorsementInfo{
						Version: "version",
					},
					ValidationInfo: &lb.ChaincodeValidationInfo{
						ValidationPlugin:    "validation-plugin",
						ValidationParameter: []byte("validation-parameter"),
					},
					Collections: &pb.CollectionConfigPackage{
						Config: []*pb.CollectionConfig{
							{
								Payload: &pb.CollectionConfig_StaticCollectionConfig{
									StaticCollectionConfig: &pb.StaticCollectionConfig{
										Name:              "collection-name",
										EndorsementPolicy: expectedPolicy,
									},
								},
							},
						},
					},
				}, fakePublicState)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the endorsement policy from the collection config", func() {
				ep, uErr, vErr := vc.CollectionValidationInfo("channel-id", "cc-name", "collection-name", fakeValidationState)
				Expect(uErr).NotTo(HaveOccurred())
				Expect(vErr).NotTo(HaveOccurred())
				Expect(ep).To(Equal(protoutil.MarshalOrPanic(expectedPolicy)))
			})
		})
	})

	Describe("ImplicitCollectionEndorsementPolicyAsBytes", func() {
		It("returns the marshaled standard EP for an implicit collection", func() {
			ep, uErr, vErr := vc.ImplicitCollectionEndorsementPolicyAsBytes("channel-id", "first-mspid")
			Expect(uErr).NotTo(HaveOccurred())
			Expect(vErr).NotTo(HaveOccurred())
			policy := &cb.ApplicationPolicy{}
			err := proto.Unmarshal(ep, policy)
			Expect(err).NotTo(HaveOccurred())
			Expect(policy.GetChannelConfigPolicyReference()).To(Equal("/Channel/Application/org0/Endorsement"))
		})

		Context("when the standard channel policy for endorsement does not exist", func() {
			BeforeEach(func() {
				fakePolicyManager.GetPolicyReturns(nil, false)
			})

			It("returns a signature policy", func() {
				ep, uErr, vErr := vc.ImplicitCollectionEndorsementPolicyAsBytes("channel-id", "first-mspid")
				Expect(uErr).NotTo(HaveOccurred())
				Expect(vErr).NotTo(HaveOccurred())
				policy := &cb.ApplicationPolicy{}
				err := proto.Unmarshal(ep, policy)
				Expect(err).NotTo(HaveOccurred())
				Expect(policy.GetSignaturePolicy()).NotTo(BeNil())
			})
		})

		Context("when the channel config cannot be retrieved", func() {
			BeforeEach(func() {
				fakeChannelConfigSource.GetStableChannelConfigReturns(nil)
			})

			It("returns an unexpected error", func() {
				_, uErr, vErr := vc.ImplicitCollectionEndorsementPolicyAsBytes("channel-id", "first-mspid")
				Expect(vErr).NotTo(HaveOccurred())
				Expect(uErr).To(MatchError("could not get channel config for channel 'channel-id'"))
			})
		})

		Context("when the application config cannot be retrieved", func() {
			BeforeEach(func() {
				fakeChannelConfig.ApplicationConfigReturns(nil, false)
			})

			It("returns an unexpected error", func() {
				_, uErr, vErr := vc.ImplicitCollectionEndorsementPolicyAsBytes("channel-id", "first-mspid")
				Expect(vErr).NotTo(HaveOccurred())
				Expect(uErr).To(MatchError("could not get application config for channel 'channel-id'"))
			})
		})

		Context("when the MSPID is not for any application org", func() {
			It("returns a validation error", func() {
				_, uErr, vErr := vc.ImplicitCollectionEndorsementPolicyAsBytes("channel-id", "bad-mspid")
				Expect(uErr).NotTo(HaveOccurred())
				Expect(vErr).To(MatchError("no org found in channel with MSPID 'bad-mspid'"))
			})
		})

	})
})
