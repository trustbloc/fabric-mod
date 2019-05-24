/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package server_test

import (
	"github.com/hyperledger/fabric/protos/token"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/hyperledger/fabric/token/server"
	"github.com/hyperledger/fabric/token/server/mock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

var _ = Describe("AccessControl", func() {
	var (
		fakeACLProvider *mock.ACLProvider
		aclResources    *server.ACLResources
		pbac            *server.PolicyBasedAccessControl

		header        *token.Header
		command       *token.Command
		signedCommand *token.SignedCommand
	)

	BeforeEach(func() {
		fakeACLProvider = &mock.ACLProvider{}
		aclResources = &server.ACLResources{IssueTokens: "pineapple"}
		pbac = &server.PolicyBasedAccessControl{
			ACLProvider:  fakeACLProvider,
			ACLResources: aclResources,
		}

		header = &token.Header{
			ChannelId: "channel-id",
			Creator:   []byte("creator"),
		}
		command = &token.Command{
			Header: header,
			Payload: &token.Command_IssueRequest{
				IssueRequest: &token.IssueRequest{},
			},
		}

		signedCommand = &token.SignedCommand{
			Command:   ProtoMarshal(command),
			Signature: []byte("signature"),
		}
	})

	It("validates the policy for import command", func() {
		err := pbac.Check(signedCommand, command)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeACLProvider.CheckACLCallCount()).To(Equal(1))
		resourceName, channelID, signedData := fakeACLProvider.CheckACLArgsForCall(0)
		Expect(resourceName).To(Equal(aclResources.IssueTokens))
		Expect(channelID).To(Equal("channel-id"))
		Expect(signedData).To(ConsistOf(&protoutil.SignedData{
			Data:      signedCommand.Command,
			Identity:  []byte("creator"),
			Signature: []byte("signature"),
		}))
	})

	It("validates the policy for transfer command", func() {
		transferCommand := &token.Command{
			Header: header,
			Payload: &token.Command_TransferRequest{
				TransferRequest: &token.TransferRequest{},
			},
		}
		signedTransferCommand := &token.SignedCommand{
			Command:   ProtoMarshal(transferCommand),
			Signature: []byte("signature"),
		}
		err := pbac.Check(signedTransferCommand, command)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeACLProvider.CheckACLCallCount()).To(Equal(1))
		resourceName, channelID, signedData := fakeACLProvider.CheckACLArgsForCall(0)
		Expect(resourceName).To(Equal(aclResources.IssueTokens))
		Expect(channelID).To(Equal("channel-id"))
		Expect(signedData).To(ConsistOf(&protoutil.SignedData{
			Data:      signedTransferCommand.Command,
			Identity:  []byte("creator"),
			Signature: []byte("signature"),
		}))
	})

	It("validates the policy for redeem command", func() {
		redeemCommand := &token.Command{
			Header: header,
			Payload: &token.Command_RedeemRequest{
				RedeemRequest: &token.RedeemRequest{},
			},
		}
		signedRedeemCommand := &token.SignedCommand{
			Command:   ProtoMarshal(redeemCommand),
			Signature: []byte("signature"),
		}
		err := pbac.Check(signedRedeemCommand, command)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeACLProvider.CheckACLCallCount()).To(Equal(1))
		resourceName, channelID, signedData := fakeACLProvider.CheckACLArgsForCall(0)
		Expect(resourceName).To(Equal(aclResources.IssueTokens))
		Expect(channelID).To(Equal("channel-id"))
		Expect(signedData).To(ConsistOf(&protoutil.SignedData{
			Data:      signedRedeemCommand.Command,
			Identity:  []byte("creator"),
			Signature: []byte("signature"),
		}))
	})

	Context("when the policy checker returns an error", func() {
		BeforeEach(func() {
			fakeACLProvider.CheckACLReturns(errors.New("wild-banana"))
		})

		It("returns the error", func() {
			err := pbac.Check(signedCommand, command)
			Expect(err).To(MatchError("wild-banana"))
		})
	})

	Context("when the command payload is nil", func() {
		BeforeEach(func() {
			command.Payload = nil
		})

		It("skips the access control check", func() {
			pbac.Check(signedCommand, command)
			Expect(fakeACLProvider.CheckACLCallCount()).To(Equal(0))
		})

		It("returns a error", func() {
			err := pbac.Check(signedCommand, command)
			Expect(err).To(MatchError("command type not recognized: <nil>"))
		})
	})

	It("validates the policy for operation issue command", func() {
		issueOpRequest := &token.TokenOperationRequest{
			Credential: []byte("credential"),
			Operations: []*token.TokenOperation{{
				Operation: &token.TokenOperation_Action{
					Action: &token.TokenOperationAction{
						Payload: &token.TokenOperationAction_Issue{
							Issue: &token.TokenActionTerms{},
						},
					},
				},
			},
			},
		}

		tokenOperationCommand := &token.Command{
			Header: header,
			Payload: &token.Command_TokenOperationRequest{
				TokenOperationRequest: issueOpRequest,
			},
		}
		signedExpectationCommand := &token.SignedCommand{
			Command:   ProtoMarshal(tokenOperationCommand),
			Signature: []byte("signature"),
		}
		err := pbac.Check(signedExpectationCommand, command)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeACLProvider.CheckACLCallCount()).To(Equal(1))
		resourceName, channelID, signedData := fakeACLProvider.CheckACLArgsForCall(0)
		Expect(resourceName).To(Equal(aclResources.IssueTokens))
		Expect(channelID).To(Equal("channel-id"))
		Expect(signedData).To(ConsistOf(&protoutil.SignedData{
			Data:      signedExpectationCommand.Command,
			Identity:  []byte("creator"),
			Signature: []byte("signature"),
		}))
	})

	It("validates the policy for operation transfer command", func() {
		transferOpRequest := &token.TokenOperationRequest{
			Credential: []byte("credential"),
			Operations: []*token.TokenOperation{{
				Operation: &token.TokenOperation_Action{
					Action: &token.TokenOperationAction{
						Payload: &token.TokenOperationAction_Transfer{
							Transfer: &token.TokenActionTerms{}},
					},
				},
			},
			},
		}
		tokenOperationCommand := &token.Command{
			Header: header,
			Payload: &token.Command_TokenOperationRequest{
				TokenOperationRequest: transferOpRequest,
			},
		}
		signedExpectationCommand := &token.SignedCommand{
			Command:   ProtoMarshal(tokenOperationCommand),
			Signature: []byte("signature"),
		}
		err := pbac.Check(signedExpectationCommand, command)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeACLProvider.CheckACLCallCount()).To(Equal(1))
		resourceName, channelID, signedData := fakeACLProvider.CheckACLArgsForCall(0)
		Expect(resourceName).To(Equal(aclResources.IssueTokens))
		Expect(channelID).To(Equal("channel-id"))
		Expect(signedData).To(ConsistOf(&protoutil.SignedData{
			Data:      signedExpectationCommand.Command,
			Identity:  []byte("creator"),
			Signature: []byte("signature"),
		}))
	})

	Context("when Expectationrequest has nil Expectation", func() {
		BeforeEach(func() {
			issueOpRequest := &token.TokenOperationRequest{
				Credential: []byte("credential"),
			}
			command = &token.Command{
				Header: header,
				Payload: &token.Command_TokenOperationRequest{
					TokenOperationRequest: issueOpRequest,
				},
			}
		})

		It("returns the error", func() {
			signedCommand := &token.SignedCommand{
				Command:   ProtoMarshal(command),
				Signature: []byte("signature"),
			}
			err := pbac.Check(signedCommand, command)
			Expect(err).To(MatchError("TokenOperationRequest has no operations"))
		})
	})

	Context("when Expectationrequest has nil PlainExpectation", func() {
		BeforeEach(func() {
			issueOpRequest := &token.TokenOperationRequest{
				Credential: []byte("credential"),
				Operations: []*token.TokenOperation{},
			}
			command = &token.Command{
				Header: header,
				Payload: &token.Command_TokenOperationRequest{
					TokenOperationRequest: issueOpRequest,
				},
			}
		})

		It("returns the error", func() {
			signedCommand := &token.SignedCommand{
				Command:   ProtoMarshal(command),
				Signature: []byte("signature"),
			}
			err := pbac.Check(signedCommand, command)
			Expect(err).To(MatchError("TokenOperationRequest has no operations"))
		})
	})
})
