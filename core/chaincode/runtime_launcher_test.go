/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chaincode_test

import (
	"time"

	"github.com/hyperledger/fabric/common/metrics/metricsfakes"
	"github.com/hyperledger/fabric/core/chaincode"
	"github.com/hyperledger/fabric/core/chaincode/accesscontrol"
	"github.com/hyperledger/fabric/core/chaincode/fake"
	"github.com/hyperledger/fabric/core/chaincode/mock"
	"github.com/hyperledger/fabric/core/container/ccintf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

var _ = Describe("RuntimeLauncher", func() {
	var (
		fakeRuntime        *mock.Runtime
		fakeRegistry       *fake.LaunchRegistry
		launchState        *chaincode.LaunchState
		fakeLaunchDuration *metricsfakes.Histogram
		fakeLaunchFailures *metricsfakes.Counter
		fakeLaunchTimeouts *metricsfakes.Counter
		fakeCertGenerator  *mock.CertGenerator
		exitedCh           chan int

		runtimeLauncher *chaincode.RuntimeLauncher
	)

	BeforeEach(func() {
		launchState = chaincode.NewLaunchState()
		fakeRegistry = &fake.LaunchRegistry{}
		fakeRegistry.LaunchingReturns(launchState, false)

		fakeRuntime = &mock.Runtime{}
		fakeRuntime.StartStub = func(string, *ccintf.PeerConnection) error {
			launchState.Notify(nil)
			return nil
		}
		exitedCh = make(chan int)
		waitExitCh := exitedCh // shadow to avoid race
		fakeRuntime.WaitStub = func(string) (int, error) {
			return <-waitExitCh, nil
		}

		fakeLaunchDuration = &metricsfakes.Histogram{}
		fakeLaunchDuration.WithReturns(fakeLaunchDuration)
		fakeLaunchFailures = &metricsfakes.Counter{}
		fakeLaunchFailures.WithReturns(fakeLaunchFailures)
		fakeLaunchTimeouts = &metricsfakes.Counter{}
		fakeLaunchTimeouts.WithReturns(fakeLaunchTimeouts)

		launchMetrics := &chaincode.LaunchMetrics{
			LaunchDuration: fakeLaunchDuration,
			LaunchFailures: fakeLaunchFailures,
			LaunchTimeouts: fakeLaunchTimeouts,
		}
		fakeCertGenerator = &mock.CertGenerator{}
		fakeCertGenerator.GenerateReturns(&accesscontrol.CertAndPrivKeyPair{Cert: []byte("cert"), Key: []byte("key")}, nil)
		runtimeLauncher = &chaincode.RuntimeLauncher{
			Runtime:        fakeRuntime,
			Registry:       fakeRegistry,
			StartupTimeout: 5 * time.Second,
			Metrics:        launchMetrics,
			PeerAddress:    "peer-address",
			CertGenerator:  fakeCertGenerator,
		}
	})

	AfterEach(func() {
		close(exitedCh)
	})

	It("registers the chaincode as launching", func() {
		err := runtimeLauncher.Launch("chaincode-name:chaincode-version")
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeRegistry.LaunchingCallCount()).To(Equal(1))
		cname := fakeRegistry.LaunchingArgsForCall(0)
		Expect(cname).To(Equal("chaincode-name:chaincode-version"))
	})

	Context("build does not return external chaincode info", func() {
		BeforeEach(func() {
			fakeRuntime.BuildReturns(nil, nil)
		})

		It("chaincode is launched", func() {
			err := runtimeLauncher.Launch("chaincode-name:chaincode-version")
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeRuntime.BuildCallCount()).To(Equal(1))
			ccciArg := fakeRuntime.BuildArgsForCall(0)
			Expect(ccciArg).To(Equal("chaincode-name:chaincode-version"))

			Expect(fakeRuntime.StartCallCount()).To(Equal(1))
		})
	})

	Context("build returns external chaincode info", func() {
		BeforeEach(func() {
			fakeRuntime.BuildReturns(&ccintf.ChaincodeServerInfo{Address: "ccaddress:12345"}, nil)
		})

		It("chaincode is not launched", func() {
			err := runtimeLauncher.Launch("chaincode-name:chaincode-version")
			Expect(err).To(MatchError("peer as client to be implemented"))

			Expect(fakeRuntime.BuildCallCount()).To(Equal(1))
			ccciArg := fakeRuntime.BuildArgsForCall(0)
			Expect(ccciArg).To(Equal("chaincode-name:chaincode-version"))

			Expect(fakeRuntime.StartCallCount()).To(Equal(0))
		})
	})

	It("starts the runtime for the chaincode", func() {
		err := runtimeLauncher.Launch("chaincode-name:chaincode-version")
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeRuntime.BuildCallCount()).To(Equal(1))
		ccciArg := fakeRuntime.BuildArgsForCall(0)
		Expect(ccciArg).To(Equal("chaincode-name:chaincode-version"))
		Expect(fakeRuntime.StartCallCount()).To(Equal(1))
		ccciArg, ccinfoArg := fakeRuntime.StartArgsForCall(0)
		Expect(ccciArg).To(Equal("chaincode-name:chaincode-version"))

		Expect(ccinfoArg).To(Equal(&ccintf.PeerConnection{Address: "peer-address", TLSConfig: &ccintf.TLSConfig{ClientCert: []byte("cert"), ClientKey: []byte("key"), RootCert: nil}}))
	})

	Context("tls is not enabled", func() {
		BeforeEach(func() {
			runtimeLauncher.CertGenerator = nil
		})

		It("starts the runtime for the chaincode with no TLS", func() {

			err := runtimeLauncher.Launch("chaincode-name:chaincode-version")
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeRuntime.BuildCallCount()).To(Equal(1))
			ccciArg := fakeRuntime.BuildArgsForCall(0)
			Expect(ccciArg).To(Equal("chaincode-name:chaincode-version"))
			Expect(fakeRuntime.StartCallCount()).To(Equal(1))
			ccciArg, ccinfoArg := fakeRuntime.StartArgsForCall(0)
			Expect(ccciArg).To(Equal("chaincode-name:chaincode-version"))

			Expect(ccinfoArg).To(Equal(&ccintf.PeerConnection{Address: "peer-address"}))
		})
	})

	It("waits for the launch to complete", func() {
		fakeRuntime.StartReturns(nil)

		errCh := make(chan error, 1)
		go func() { errCh <- runtimeLauncher.Launch("chaincode-name:chaincode-version") }()

		Consistently(errCh).ShouldNot(Receive())
		launchState.Notify(nil)
		Eventually(errCh).Should(Receive(BeNil()))
	})

	It("does not deregister the chaincode", func() {
		err := runtimeLauncher.Launch("chaincode-name:chaincode-version")
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeRegistry.DeregisterCallCount()).To(Equal(0))
	})

	It("records launch duration", func() {
		err := runtimeLauncher.Launch("chaincode-name:chaincode-version")
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeLaunchDuration.WithCallCount()).To(Equal(1))
		labelValues := fakeLaunchDuration.WithArgsForCall(0)
		Expect(labelValues).To(Equal([]string{
			"chaincode", "chaincode-name:chaincode-version",
			"success", "true",
		}))
		Expect(fakeLaunchDuration.ObserveArgsForCall(0)).NotTo(BeZero())
		Expect(fakeLaunchDuration.ObserveArgsForCall(0)).To(BeNumerically("<", 1.0))
	})

	Context("when starting the runtime fails", func() {
		BeforeEach(func() {
			fakeRuntime.StartReturns(errors.New("banana"))
		})

		It("returns a wrapped error", func() {
			err := runtimeLauncher.Launch("chaincode-name:chaincode-version")
			Expect(err).To(MatchError("error starting container: banana"))
		})

		It("notifies the LaunchState", func() {
			runtimeLauncher.Launch("chaincode-name:chaincode-version")
			Eventually(launchState.Done()).Should(BeClosed())
			Expect(launchState.Err()).To(MatchError("error starting container: banana"))
		})

		It("records chaincode launch failures", func() {
			runtimeLauncher.Launch("chaincode-name:chaincode-version")
			Expect(fakeLaunchFailures.WithCallCount()).To(Equal(1))
			labelValues := fakeLaunchFailures.WithArgsForCall(0)
			Expect(labelValues).To(Equal([]string{
				"chaincode", "chaincode-name:chaincode-version",
			}))
			Expect(fakeLaunchFailures.AddCallCount()).To(Equal(1))
			Expect(fakeLaunchFailures.AddArgsForCall(0)).To(BeNumerically("~", 1.0))
		})

		It("stops the runtime", func() {
			runtimeLauncher.Launch("chaincode-name:chaincode-version")

			Expect(fakeRuntime.StopCallCount()).To(Equal(1))
			ccciArg := fakeRuntime.StopArgsForCall(0)
			Expect(ccciArg).To(Equal("chaincode-name:chaincode-version"))
		})

		It("deregisters the chaincode", func() {
			runtimeLauncher.Launch("chaincode-name:chaincode-version")

			Expect(fakeRegistry.DeregisterCallCount()).To(Equal(1))
			cname := fakeRegistry.DeregisterArgsForCall(0)
			Expect(cname).To(Equal("chaincode-name:chaincode-version"))
		})
	})

	Context("when the contaienr terminates before registration", func() {
		BeforeEach(func() {
			fakeRuntime.StartReturns(nil)
			fakeRuntime.WaitReturns(-99, nil)
		})

		It("returns an error", func() {
			err := runtimeLauncher.Launch("chaincode-name:chaincode-version")
			Expect(err).To(MatchError("chaincode registration failed: container exited with -99"))
		})

		It("attempts to stop the runtime", func() {
			runtimeLauncher.Launch("chaincode-name:chaincode-version")

			Expect(fakeRuntime.StopCallCount()).To(Equal(1))
			ccciArg := fakeRuntime.StopArgsForCall(0)
			Expect(ccciArg).To(Equal("chaincode-name:chaincode-version"))
		})

		It("deregisters the chaincode", func() {
			runtimeLauncher.Launch("chaincode-name:chaincode-version")

			Expect(fakeRegistry.DeregisterCallCount()).To(Equal(1))
			cname := fakeRegistry.DeregisterArgsForCall(0)
			Expect(cname).To(Equal("chaincode-name:chaincode-version"))
		})
	})

	Context("when handler registration fails", func() {
		BeforeEach(func() {
			fakeRuntime.StartStub = func(string, *ccintf.PeerConnection) error {
				launchState.Notify(errors.New("papaya"))
				return nil
			}
		})

		It("returns an error", func() {
			err := runtimeLauncher.Launch("chaincode-name:chaincode-version")
			Expect(err).To(MatchError("chaincode registration failed: papaya"))
		})

		It("stops the runtime", func() {
			runtimeLauncher.Launch("chaincode-name:chaincode-version")

			Expect(fakeRuntime.StopCallCount()).To(Equal(1))
			ccciArg := fakeRuntime.StopArgsForCall(0)
			Expect(ccciArg).To(Equal("chaincode-name:chaincode-version"))
		})

		It("deregisters the chaincode", func() {
			runtimeLauncher.Launch("chaincode-name:chaincode-version")

			Expect(fakeRegistry.DeregisterCallCount()).To(Equal(1))
			cname := fakeRegistry.DeregisterArgsForCall(0)
			Expect(cname).To(Equal("chaincode-name:chaincode-version"))
		})
	})

	Context("when the runtime startup times out", func() {
		BeforeEach(func() {
			fakeRuntime.StartReturns(nil)
			runtimeLauncher.StartupTimeout = 250 * time.Millisecond
		})

		It("returns a meaningful error", func() {
			err := runtimeLauncher.Launch("chaincode-name:chaincode-version")
			Expect(err).To(MatchError("timeout expired while starting chaincode chaincode-name:chaincode-version for transaction"))
		})

		It("notifies the LaunchState", func() {
			runtimeLauncher.Launch("chaincode-name:chaincode-version")
			Eventually(launchState.Done()).Should(BeClosed())
			Expect(launchState.Err()).To(MatchError("timeout expired while starting chaincode chaincode-name:chaincode-version for transaction"))
		})

		It("records chaincode launch timeouts", func() {
			runtimeLauncher.Launch("chaincode-name:chaincode-version")
			Expect(fakeLaunchTimeouts.WithCallCount()).To(Equal(1))
			labelValues := fakeLaunchTimeouts.WithArgsForCall(0)
			Expect(labelValues).To(Equal([]string{
				"chaincode", "chaincode-name:chaincode-version",
			}))
			Expect(fakeLaunchTimeouts.AddCallCount()).To(Equal(1))
			Expect(fakeLaunchTimeouts.AddArgsForCall(0)).To(BeNumerically("~", 1.0))
		})

		It("stops the runtime", func() {
			runtimeLauncher.Launch("chaincode-name:chaincode-version")

			Expect(fakeRuntime.StopCallCount()).To(Equal(1))
			ccciArg := fakeRuntime.StopArgsForCall(0)
			Expect(ccciArg).To(Equal("chaincode-name:chaincode-version"))
		})

		It("deregisters the chaincode", func() {
			runtimeLauncher.Launch("chaincode-name:chaincode-version")

			Expect(fakeRegistry.DeregisterCallCount()).To(Equal(1))
			cname := fakeRegistry.DeregisterArgsForCall(0)
			Expect(cname).To(Equal("chaincode-name:chaincode-version"))
		})
	})

	Context("when the registry indicates the chaincode has already been started", func() {
		BeforeEach(func() {
			fakeRegistry.LaunchingReturns(launchState, true)
		})

		It("does not start the runtime for the chaincode", func() {
			launchState.Notify(nil)

			err := runtimeLauncher.Launch("chaincode-name:chaincode-version")
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeRuntime.StartCallCount()).To(Equal(0))
		})

		It("waits for the launch to complete", func() {
			fakeRuntime.StartReturns(nil)

			errCh := make(chan error, 1)
			go func() { errCh <- runtimeLauncher.Launch("chaincode-name:chaincode-version") }()

			Consistently(errCh).ShouldNot(Receive())
			launchState.Notify(nil)
			Eventually(errCh).Should(Receive(BeNil()))
		})

		Context("when the launch fails", func() {
			BeforeEach(func() {
				launchState.Notify(errors.New("gooey-guac"))
			})

			It("does not deregister the chaincode", func() {
				err := runtimeLauncher.Launch("chaincode-name:chaincode-version")
				Expect(err).To(MatchError("chaincode registration failed: gooey-guac"))
				Expect(fakeRegistry.DeregisterCallCount()).To(Equal(0))
			})

			It("does not stop the runtime", func() {
				err := runtimeLauncher.Launch("chaincode-name:chaincode-version")
				Expect(err).To(MatchError("chaincode registration failed: gooey-guac"))
				Expect(fakeRuntime.StopCallCount()).To(Equal(0))
			})
		})
	})

	Context("when stopping the runtime fails", func() {
		BeforeEach(func() {
			fakeRuntime.StartReturns(errors.New("whirled-peas"))
			fakeRuntime.StopReturns(errors.New("applesauce"))
		})

		It("preserves the initial error", func() {
			err := runtimeLauncher.Launch("chaincode-name:chaincode-version")
			Expect(err).To(MatchError("error starting container: whirled-peas"))
			Expect(fakeRuntime.StopCallCount()).To(Equal(1))
		})
	})
})
