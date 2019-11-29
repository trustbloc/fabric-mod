/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package nwo_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hyperledger/fabric/integration/nwo"
	"github.com/hyperledger/fabric/integration/nwo/commands"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	yaml "gopkg.in/yaml.v2"
)

var _ = Describe("Network", func() {
	var (
		client  *docker.Client
		tempDir string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "nwo")
		Expect(err).NotTo(HaveOccurred())

		client, err = docker.NewClientFromEnv()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Describe("solo network", func() {
		var network *nwo.Network
		var process ifrit.Process

		BeforeEach(func() {
			soloBytes, err := ioutil.ReadFile("solo.yaml")
			Expect(err).NotTo(HaveOccurred())

			var config *nwo.Config
			err = yaml.Unmarshal(soloBytes, &config)
			Expect(err).NotTo(HaveOccurred())

			network = nwo.New(config, tempDir, client, StartPort(), components)

			// Generate config and bootstrap the network
			network.GenerateConfigTree()
			network.Bootstrap()

			// Start all of the fabric processes
			networkRunner := network.NetworkGroupRunner()
			process = ifrit.Invoke(networkRunner)
			Eventually(process.Ready(), network.EventuallyTimeout).Should(BeClosed())
		})

		AfterEach(func() {
			// Shutdown processes and cleanup
			process.Signal(syscall.SIGTERM)
			Eventually(process.Wait(), network.EventuallyTimeout).Should(Receive())
			network.Cleanup()
		})

		It("deploys and executes chaincode (simple) using the legacy lifecycle", func() {
			orderer := network.Orderer("orderer0")
			peer := network.Peer("org1", "peer2")

			legacyChaincode := nwo.Chaincode{
				Name:    "mycc",
				Version: "0.0",
				Path:    "github.com/hyperledger/fabric/integration/chaincode/simple/cmd",
				Ctor:    `{"Args":["init","a","100","b","200"]}`,
				Policy:  `AND ('Org1ExampleCom.member','Org2ExampleCom.member')`,
			}

			network.CreateAndJoinChannels(orderer)
			nwo.DeployChaincodeLegacy(network, "testchannel", orderer, legacyChaincode)
			RunQueryInvokeQuery(network, orderer, peer, 100)
		})

		It("deploys and executes chaincode (simple) using _lifecycle", func() {
			orderer := network.Orderer("orderer0")
			peer := network.Peer("org1", "peer2")

			chaincode := nwo.Chaincode{
				Name:            "mycc",
				Version:         "0.0",
				Path:            "github.com/hyperledger/fabric/integration/chaincode/simple/cmd",
				Lang:            "golang",
				PackageFile:     filepath.Join(tempDir, "simplecc.tar.gz"),
				Ctor:            `{"Args":["init","a","100","b","200"]}`,
				SignaturePolicy: `AND ('Org1ExampleCom.member','Org2ExampleCom.member')`,
				Sequence:        "1",
				InitRequired:    true,
				Label:           "my_simple_chaincode",
			}

			network.CreateAndJoinChannels(orderer)

			network.UpdateChannelAnchors(orderer, "testchannel")
			network.VerifyMembership(network.PeersWithChannel("testchannel"), "testchannel")

			nwo.EnableCapabilities(network, "testchannel", "Application", "V2_0", orderer, network.Peer("org1", "peer1"), network.Peer("org2", "peer1"))
			nwo.DeployChaincode(network, "testchannel", orderer, chaincode)

			RunQueryInvokeQuery(network, orderer, peer, 100)
		})
	})

	Describe("kafka network", func() {
		var (
			config    nwo.Config
			network   *nwo.Network
			processes map[string]ifrit.Process
		)

		BeforeEach(func() {
			soloBytes, err := ioutil.ReadFile("solo.yaml")
			Expect(err).NotTo(HaveOccurred())

			err = yaml.Unmarshal(soloBytes, &config)
			Expect(err).NotTo(HaveOccurred())

			// Switch from solo to kafka
			config.Consensus.Type = "kafka"
			config.Consensus.ZooKeepers = 1
			config.Consensus.Brokers = 1

			network = nwo.New(&config, tempDir, client, StartPort(), components)
			network.GenerateConfigTree()
			network.Bootstrap()
			processes = map[string]ifrit.Process{}
		})

		AfterEach(func() {
			for _, p := range processes {
				p.Signal(syscall.SIGTERM)
				Eventually(p.Wait(), network.EventuallyTimeout).Should(Receive())
			}
			network.Cleanup()
		})

		It("deploys and executes chaincode (the hard way) using the legacy lifecycle", func() {
			// This demonstrates how to control the processes that make up a network.
			// If you don't care about a collection of processes (like the brokers or
			// the orderers) use the group runner to manage those processes.
			zookeepers := []string{}
			for i := 0; i < network.Consensus.ZooKeepers; i++ {
				zk := network.ZooKeeperRunner(i)
				zookeepers = append(zookeepers, fmt.Sprintf("%s:2181", zk.Name))

				p := ifrit.Invoke(zk)
				processes[zk.Name] = p
				Eventually(p.Ready(), network.EventuallyTimeout).Should(BeClosed())
			}

			for i := 0; i < network.Consensus.Brokers; i++ {
				b := network.BrokerRunner(i, zookeepers)
				p := ifrit.Invoke(b)
				processes[b.Name] = p
				Eventually(p.Ready(), network.EventuallyTimeout).Should(BeClosed())
			}

			for _, o := range network.Orderers {
				or := network.OrdererRunner(o)
				p := ifrit.Invoke(or)
				processes[o.ID()] = p
				Eventually(p.Ready(), network.EventuallyTimeout).Should(BeClosed())
			}

			for _, peer := range network.Peers {
				pr := network.PeerRunner(peer)
				p := ifrit.Invoke(pr)
				processes[peer.ID()] = p
				Eventually(p.Ready(), network.EventuallyTimeout).Should(BeClosed())
			}

			orderer := network.Orderer("orderer0")
			testPeers := network.PeersWithChannel("testchannel")
			network.CreateChannel("testchannel", orderer, testPeers[0])
			network.JoinChannel("testchannel", orderer, testPeers...)

			legacyChaincode := nwo.Chaincode{
				Name:    "mycc",
				Version: "0.0",
				Path:    "github.com/hyperledger/fabric/integration/chaincode/simple/cmd",
				Ctor:    `{"Args":["init","a","100","b","200"]}`,
				Policy:  `AND ('Org1ExampleCom.member','Org2ExampleCom.member')`,
			}
			nwo.InstallChaincodeLegacy(network, legacyChaincode, testPeers...)
			nwo.InstantiateChaincodeLegacy(network, "testchannel", orderer, legacyChaincode, testPeers[0])
			nwo.EnsureInstantiatedLegacy(network, "testchannel", "mycc", "0.0", testPeers...)

			RunQueryInvokeQuery(network, orderer, testPeers[0], 100)
		})

		It("packages and installs chaincode (the hard way) using _lifecycle", func() {
			// This demonstrates how to control the processes that make up a network.
			// If you don't care about a collection of processes (like the brokers or
			// the orderers) use the group runner to manage those processes.
			zookeepers := []string{}
			for i := 0; i < network.Consensus.ZooKeepers; i++ {
				zk := network.ZooKeeperRunner(i)
				zookeepers = append(zookeepers, fmt.Sprintf("%s:2181", zk.Name))

				p := ifrit.Invoke(zk)
				processes[zk.Name] = p
				Eventually(p.Ready(), network.EventuallyTimeout).Should(BeClosed())
			}

			for i := 0; i < network.Consensus.Brokers; i++ {
				b := network.BrokerRunner(i, zookeepers)
				p := ifrit.Invoke(b)
				processes[b.Name] = p
				Eventually(p.Ready(), network.EventuallyTimeout).Should(BeClosed())
			}

			for _, o := range network.Orderers {
				or := network.OrdererRunner(o)
				p := ifrit.Invoke(or)
				processes[o.ID()] = p
				Eventually(p.Ready(), network.EventuallyTimeout).Should(BeClosed())
			}

			for _, peer := range network.Peers {
				pr := network.PeerRunner(peer)
				p := ifrit.Invoke(pr)
				processes[peer.ID()] = p
				Eventually(p.Ready(), network.EventuallyTimeout).Should(BeClosed())
			}

			orderer := network.Orderer("orderer0")
			testPeers := network.PeersWithChannel("testchannel")
			network.CreateChannel("testchannel", orderer, testPeers[0])
			network.JoinChannel("testchannel", orderer, testPeers...)

			network.UpdateChannelAnchors(orderer, "testchannel")
			network.VerifyMembership(testPeers, "testchannel")

			nwo.EnableCapabilities(network, "testchannel", "Application", "V2_0", orderer, network.Peer("org1", "peer1"), network.Peer("org2", "peer1"))

			chaincode := nwo.Chaincode{
				Name:              "mycc",
				Version:           "0.0",
				Path:              "github.com/hyperledger/fabric/integration/chaincode/simple/cmd",
				Lang:              "golang",
				PackageFile:       filepath.Join(tempDir, "simplecc.tar.gz"),
				Ctor:              `{"Args":["init","a","100","b","200"]}`,
				EndorsementPlugin: "escc",
				ValidationPlugin:  "vscc",
				SignaturePolicy:   `AND ('Org1ExampleCom.member','Org2ExampleCom.member')`,
				Sequence:          "1",
				InitRequired:      true,
				Label:             "my_simple_chaincode",
			}

			nwo.PackageChaincode(network, chaincode, testPeers[0])

			nwo.InstallChaincode(network, chaincode, testPeers...)

			nwo.ApproveChaincodeForMyOrg(network, "testchannel", orderer, chaincode, testPeers...)

			nwo.CheckCommitReadinessUntilReady(network, "testchannel", chaincode, network.PeerOrgs(), testPeers...)
			nwo.CommitChaincode(network, "testchannel", orderer, chaincode, testPeers[0], testPeers...)
			nwo.InitChaincode(network, "testchannel", orderer, chaincode, testPeers...)

			RunQueryInvokeQuery(network, orderer, testPeers[0], 100)
		})
	})
})

func RunQueryInvokeQuery(n *nwo.Network, orderer *nwo.Orderer, peer *nwo.Peer, initialQueryResult int) {
	By("querying the chaincode")
	sess, err := n.PeerUserSession(peer, "User1", commands.ChaincodeQuery{
		ChannelID: "testchannel",
		Name:      "mycc",
		Ctor:      `{"Args":["query","a"]}`,
	})
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess, n.EventuallyTimeout).Should(gexec.Exit(0))
	Expect(sess).To(gbytes.Say(fmt.Sprint(initialQueryResult)))

	sess, err = n.PeerUserSession(peer, "User1", commands.ChaincodeInvoke{
		ChannelID: "testchannel",
		Orderer:   n.OrdererAddress(orderer, nwo.ListenPort),
		Name:      "mycc",
		Ctor:      `{"Args":["invoke","a","b","10"]}`,
		PeerAddresses: []string{
			n.PeerAddress(n.Peer("org1", "peer1"), nwo.ListenPort),
			n.PeerAddress(n.Peer("org2", "peer2"), nwo.ListenPort),
		},
		WaitForEvent: true,
	})
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess, n.EventuallyTimeout).Should(gexec.Exit(0))
	Expect(sess.Err).To(gbytes.Say("Chaincode invoke successful. result: status:200"))

	sess, err = n.PeerUserSession(peer, "User1", commands.ChaincodeQuery{
		ChannelID: "testchannel",
		Name:      "mycc",
		Ctor:      `{"Args":["query","a"]}`,
	})
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess, n.EventuallyTimeout).Should(gexec.Exit(0))
	Expect(sess).To(gbytes.Say(fmt.Sprint(initialQueryResult - 10)))
}
