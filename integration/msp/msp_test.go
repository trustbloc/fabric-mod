/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package msp

import (
	"fmt"
	"io"
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
)

var _ = Describe("MSP identity test on a network with mutual TLS required", func() {
	var (
		client  *docker.Client
		tempDir string
		network *nwo.Network
		process ifrit.Process
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "msp")
		Expect(err).NotTo(HaveOccurred())

		client, err = docker.NewClientFromEnv()
		Expect(err).NotTo(HaveOccurred())

		network = nwo.New(nwo.BasicSolo(), tempDir, client, StartPort(), components)
	})

	AfterEach(func() {
		// Shutdown processes and cleanup
		process.Signal(syscall.SIGTERM)
		Eventually(process.Wait(), network.EventuallyTimeout).Should(Receive())

		if network != nil {
			network.Cleanup()
		}
		os.RemoveAll(tempDir)
	})

	It("invokes chaincode on a peer that does not have a valid endorser identity", func() {
		By("setting TLS ClientAuthRequired to be true for all peers and orderers")
		network.ClientAuthRequired = true

		network.GenerateConfigTree()
		network.Bootstrap()

		By("starting all processes for fabric")
		networkRunner := network.NetworkGroupRunner()
		process = ifrit.Invoke(networkRunner)
		Eventually(process.Ready(), network.EventuallyTimeout).Should(BeClosed())

		peer := network.Peers[0]
		orderer := network.Orderer("orderer")

		By("creating and joining channels")
		network.CreateAndJoinChannels(orderer)
		By("enabling new lifecycle capabilities")
		nwo.EnableCapabilities(network, "testchannel", "Application", "V2_0", orderer, network.Peer("Org1", "peer1"), network.Peer("Org2", "peer1"))

		chaincode := nwo.Chaincode{
			Name:            "mycc",
			Version:         "0.0",
			Path:            "github.com/hyperledger/fabric/integration/chaincode/simple/cmd",
			Lang:            "golang",
			PackageFile:     filepath.Join(tempDir, "simplecc.tar.gz"),
			Ctor:            `{"Args":["init","a","100","b","200"]}`,
			SignaturePolicy: `OR ('Org1MSP.peer', 'Org2MSP.peer')`,
			Sequence:        "1",
			InitRequired:    true,
			Label:           "my_simple_chaincode",
		}

		By("deploying the chaincode")
		nwo.DeployChaincode(network, "testchannel", orderer, chaincode)

		By("querying and invoking chaincode with mutual TLS enabled")
		RunQueryInvokeQuery(network, orderer, peer, 100)

		By("replacing org2peer0's identity with a client identity")
		org2Peer0 := network.Peer("Org2", "peer0")
		org2Peer0MSPDir := network.PeerLocalMSPDir(org2Peer0)
		org2User1MSPDir := network.PeerUserMSPDir(org2Peer0, "User1")

		_, err := copyFile(filepath.Join(org2User1MSPDir, "signcerts", "User1@org2.example.com-cert.pem"), filepath.Join(org2Peer0MSPDir, "signcerts", "peer0.org2.example.com-cert.pem"))
		Expect(err).NotTo(HaveOccurred())
		_, err = copyFile(filepath.Join(org2User1MSPDir, "keystore", "priv_sk"), filepath.Join(org2Peer0MSPDir, "keystore", "priv_sk"))
		Expect(err).NotTo(HaveOccurred())

		By("restarting all fabric processes to reload MSP identities")
		process.Signal(syscall.SIGTERM)
		Eventually(process.Wait(), network.EventuallyTimeout).Should(Receive())
		networkRunner = network.NetworkGroupRunner()
		process = ifrit.Invoke(networkRunner)
		Eventually(process.Ready(), network.EventuallyTimeout).Should(BeClosed())

		By("attempting to invoke chaincode on a peer that does not have a valid endorser identity")
		sess, err := network.PeerUserSession(peer, "User1", commands.ChaincodeInvoke{
			ChannelID: "testchannel",
			Orderer:   network.OrdererAddress(orderer, nwo.ListenPort),
			Name:      "mycc",
			Ctor:      `{"Args":["invoke","a","b","10"]}`,
			PeerAddresses: []string{
				network.PeerAddress(network.Peer("Org2", "peer0"), nwo.ListenPort),
			},
			WaitForEvent: true,
			ClientAuth:   network.ClientAuthRequired,
		})
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess, network.EventuallyTimeout).Should(gexec.Exit(1))
		Expect(sess.Err).To(gbytes.Say(`(ENDORSEMENT_POLICY_FAILURE)`))

		By("reverifying the channel was not affected by the unauthorized endorsement")
		sess, err = network.PeerUserSession(peer, "User1", commands.ChaincodeQuery{
			ChannelID: "testchannel",
			Name:      "mycc",
			Ctor:      `{"Args":["query","a"]}`,
		})
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess, network.EventuallyTimeout).Should(gexec.Exit(0))
		Expect(sess).To(gbytes.Say("90"))
	})
})

func RunQueryInvokeQuery(n *nwo.Network, orderer *nwo.Orderer, peer *nwo.Peer, initialQueryResult int) {
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
			n.PeerAddress(n.Peer("Org1", "peer1"), nwo.ListenPort),
			n.PeerAddress(n.Peer("Org2", "peer1"), nwo.ListenPort),
		},
		WaitForEvent: true,
		ClientAuth:   n.ClientAuthRequired,
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

func copyFile(src, dst string) (int64, error) {
	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	err = os.Remove(dst)
	if err != nil {
		return 0, err
	}
	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}
