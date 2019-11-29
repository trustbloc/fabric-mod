/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package ledger

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/integration/nwo"
	"github.com/hyperledger/fabric/integration/nwo/commands"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"gopkg.in/yaml.v2"
)

var _ = Describe("rollback, reset, pause and resume peer node commands", func() {
	// at the beginning of each test under this block, we have defined two collections:
	// 1. collectionMarbles - Org1 and Org2 have access to this collection
	// 2. collectionMarblePrivateDetails - Org2 and Org3 have access to this collection
	// when calling QueryChaincode with first arg "readMarble", it will query collectionMarbles[1]
	// when calling QueryChaincode with first arg "readMarblePrivateDetails", it will query collectionMarblePrivateDetails[2]
	var (
		setup  *setup
		helper *testHelper
	)

	BeforeEach(func() {
		setup = initThreeOrgsSetup()
		nwo.EnableCapabilities(setup.network, setup.channelID, "Application", "V2_0", setup.orderer, setup.peers...)
		helper = &testHelper{
			networkHelper: &networkHelper{
				Network:   setup.network,
				orderer:   setup.orderer,
				peers:     setup.peers,
				testDir:   setup.testDir,
				channelID: setup.channelID,
			},
		}

		By("installing and instantiating chaincode on all peers")
		chaincode := nwo.Chaincode{
			Name:              "marblesp",
			Version:           "1.0",
			Path:              components.Build("github.com/hyperledger/fabric/integration/chaincode/marbles_private/cmd"),
			Lang:              "binary",
			PackageFile:       filepath.Join(setup.testDir, "marbles-pvtdata.tar.gz"),
			Label:             "marbles-private-20",
			SignaturePolicy:   `OR ('Org1MSP.member','Org2MSP.member', 'Org3MSP.member')`,
			CollectionsConfig: filepath.Join("testdata", "collection_configs", "collections_config1.json"),
			Sequence:          "1",
		}

		helper.deployChaincode(chaincode)

		org2peer0 := setup.network.Peer("org2", "peer0")
		height := helper.getLedgerHeight(org2peer0)

		By("creating 5 blocks")
		for i := 1; i <= 5; i++ {
			helper.addMarble("marblesp", fmt.Sprintf(`{"name":"marble%d", "color":"blue", "size":35, "owner":"tom", "price":99}`, i), org2peer0)
			helper.waitUntilEqualLedgerHeight(height + i)
		}

		By("verifying marble1 to marble5 exist in collectionMarbles & collectionMarblePrivateDetails in peer0.org2")
		for i := 1; i <= 5; i++ {
			helper.assertPresentInCollectionM("marblesp", fmt.Sprintf("marble%d", i), org2peer0)
			helper.assertPresentInCollectionMPD("marblesp", fmt.Sprintf("marble%d", i), org2peer0)
		}
	})

	AfterEach(func() {
		setup.cleanup()
	})

	// This test executes the rollback, reset, pause, and resume commands on the following peerss
	// org1.peer0 - rollback
	// org2.peer0 - reset
	// org3.peer0 - pause/rollback/resume
	//
	// There are 14 blocks created in BeforeEach (before rollback/reset).
	// block 0: genesis, block 1: org1Anchor, block 2: org2Anchor, block 3: org3Anchor
	// block 4 to 8: chaincode instantiation, block 9 to 13: chaincode invoke to add marbles.
	It("pauses and resumes channels and rolls back and resets the ledger", func() {
		By("Checking ledger height on each peer")
		for _, peer := range helper.peers {
			Expect(helper.getLedgerHeight(peer)).Should(Equal(14))
		}

		org1peer0 := setup.network.Peer("org1", "peer0")
		org2peer0 := setup.network.Peer("org2", "peer0")
		org3peer0 := setup.network.Peer("org3", "peer0")

		// Negative test: rollback, reset, pause, and resume should fail when the peer is online
		expectedErrMessage := "as another peer node command is executing," +
			" wait for that command to complete its execution or terminate it before retrying"
		By("Rolling back the peer to block 6 from block 13 while the peer node is online")
		helper.rollback(org1peer0, 6, expectedErrMessage, false)
		By("Resetting the peer to the genesis block while the peer node is online")
		helper.reset(org2peer0, expectedErrMessage, false)
		By("Pausing the peer while the peer node is online")
		helper.pause(org3peer0, expectedErrMessage, false)
		By("Resuming the peer while the peer node is online")
		helper.resume(org3peer0, expectedErrMessage, false)

		By("Stopping the network to test commands")
		setup.terminateAllProcess()

		By("Rolling back the channel to block 6 from block 14 on org1peer0")
		helper.rollback(org1peer0, 6, "", true)

		By("Resetting org2peer0 to the genesis block")
		helper.reset(org2peer0, "", true)

		By("Pausing the channel on org3peer0")
		helper.pause(org3peer0, "", true)

		By("Rolling back the paused channel to block 6 from block 14 on org3peer0")
		helper.rollback(org3peer0, 6, "", true)

		By("Verifying paused channel is not found upon peer restart")
		setup.startPeer(org3peer0)
		helper.assertPausedChannel(org3peer0)

		By("Checking preResetHeightFile exists for a paused channel that is also rolled back or reset")
		setup.startBrokerAndOrderer()
		preResetHeightFile := filepath.Join(setup.network.PeerLedgerDir(org3peer0), "chains/chains", helper.channelID, "__preResetHeight")
		Expect(preResetHeightFile).To(BeARegularFile())

		setup.terminateAllProcess()

		By("Resuming the peer")
		helper.resume(org3peer0, "", true)

		By("Verifying that the endorsement is disabled when the peer has not received missing blocks")
		setup.startPeers()
		for _, peer := range setup.peers {
			helper.assertDisabledEndorser("marblesp", peer)
		}

		By("Bringing the peers to recent height by starting the orderer")
		setup.startBrokerAndOrderer()
		for _, peer := range setup.peers {
			By("Verifying endorsement is enabled and preResetHeightFile is removed on peer " + peer.ID())
			helper.waitUntilEndorserEnabled(peer)
			preResetHeightFile := filepath.Join(setup.network.PeerLedgerDir(peer), "chains/chains", helper.channelID, "__preResetHeight")
			Expect(preResetHeightFile).NotTo(BeAnExistingFile())
		}

		setup.network.VerifyMembership(setup.peers, setup.channelID, "marblesp")

		By("Verifying leger height on all peers")
		helper.waitUntilEqualLedgerHeight(14)

		// Test chaincode works correctly after the commands
		By("Creating 2 more blocks post rollback/reset")
		for i := 6; i <= 7; i++ {
			helper.addMarble("marblesp", fmt.Sprintf(`{"name":"marble%d", "color":"blue", "size":35, "owner":"tom", "price":99}`, i), org2peer0)
			helper.waitUntilEqualLedgerHeight(14 + i - 5)
		}

		By("Verifying marble1 to marble7 exist in collectionMarbles & collectionMarblePrivateDetails on org2peer0")
		for i := 1; i <= 7; i++ {
			helper.assertPresentInCollectionM("marblesp", fmt.Sprintf("marble%d", i), org2peer0)
			helper.assertPresentInCollectionMPD("marblesp", fmt.Sprintf("marble%d", i), org2peer0)
		}

		// statedb rebuild test
		By("Stopping peers and deleting the statedb folder on peer org2.peer0")
		peer := setup.network.Peer("org2", "peer0")
		setup.stopPeers()
		dbPath := filepath.Join(setup.network.PeerLedgerDir(peer), "stateLeveldb")
		Expect(os.RemoveAll(dbPath)).NotTo(HaveOccurred())
		Expect(dbPath).NotTo(BeADirectory())
		By("Restarting the peer org2.peer0")
		setup.startPeer(peer)
		Expect(dbPath).To(BeADirectory())
		helper.assertPresentInCollectionM("marblesp", "marble2", peer)
	})
})

type setup struct {
	testDir        string
	channelID      string
	network        *nwo.Network
	peers          []*nwo.Peer
	peerProcess    []ifrit.Process
	orderer        *nwo.Orderer
	ordererProcess ifrit.Process
	brokerProcess  ifrit.Process
}

func initThreeOrgsSetup() *setup {
	var err error
	testDir, err := ioutil.TempDir("", "reset-rollback")
	Expect(err).NotTo(HaveOccurred())

	client, err := docker.NewClientFromEnv()
	Expect(err).NotTo(HaveOccurred())

	configBytes, err := ioutil.ReadFile(filepath.Join("testdata", "network.yaml"))
	Expect(err).NotTo(HaveOccurred())

	var networkConfig *nwo.Config
	err = yaml.Unmarshal(configBytes, &networkConfig)
	Expect(err).NotTo(HaveOccurred())

	n := nwo.New(networkConfig, testDir, client, StartPort(), components)
	n.GenerateConfigTree()
	n.Bootstrap()

	peers := []*nwo.Peer{
		n.Peer("org1", "peer0"),
		n.Peer("org2", "peer0"),
		n.Peer("org3", "peer0"),
	}

	setup := &setup{
		testDir:   testDir,
		network:   n,
		peers:     peers,
		channelID: "testchannel",
	}

	setup.startBrokerAndOrderer()

	setup.startPeer(peers[0])
	setup.startPeer(peers[1])
	setup.startPeer(peers[2])

	orderer := n.Orderer("orderer")
	n.CreateAndJoinChannel(orderer, "testchannel")
	n.UpdateChannelAnchors(orderer, "testchannel")
	setup.orderer = orderer

	By("verifying membership")
	setup.network.VerifyMembership(setup.peers, setup.channelID)

	return setup
}

func (s *setup) cleanup() {
	s.terminateAllProcess()
	s.network.Cleanup()
	os.RemoveAll(s.testDir)
}

func (s *setup) terminateAllProcess() {
	s.ordererProcess.Signal(syscall.SIGTERM)
	Eventually(s.ordererProcess.Wait(), s.network.EventuallyTimeout).Should(Receive())
	s.ordererProcess = nil

	s.brokerProcess.Signal(syscall.SIGTERM)
	Eventually(s.brokerProcess.Wait(), s.network.EventuallyTimeout).Should(Receive())
	s.brokerProcess = nil

	for _, p := range s.peerProcess {
		p.Signal(syscall.SIGTERM)
		Eventually(p.Wait(), s.network.EventuallyTimeout).Should(Receive())
	}
	s.peerProcess = nil
}

func (s *setup) startPeers() {
	for _, peer := range s.peers {
		s.startPeer(peer)
	}
}

func (s *setup) stopPeers() {
	for _, p := range s.peerProcess {
		p.Signal(syscall.SIGTERM)
		Eventually(p.Wait(), s.network.EventuallyTimeout).Should(Receive())
	}
	s.peerProcess = nil
}

func (s *setup) startPeer(peer *nwo.Peer) {
	peerRunner := s.network.PeerRunner(peer)
	peerProcess := ifrit.Invoke(peerRunner)
	Eventually(peerProcess.Ready(), s.network.EventuallyTimeout).Should(BeClosed())
	s.peerProcess = append(s.peerProcess, peerProcess)
}

func (s *setup) startBrokerAndOrderer() {

	brokerRunner := s.network.BrokerGroupRunner()
	brokerProcess := ifrit.Invoke(brokerRunner)
	Eventually(brokerProcess.Ready(), s.network.EventuallyTimeout).Should(BeClosed())
	s.brokerProcess = brokerProcess

	ordererRunner := s.network.OrdererGroupRunner()
	ordererProcess := ifrit.Invoke(ordererRunner)
	Eventually(ordererProcess.Ready(), s.network.EventuallyTimeout).Should(BeClosed())
	s.ordererProcess = ordererProcess
}

type networkHelper struct {
	*nwo.Network
	orderer   *nwo.Orderer
	peers     []*nwo.Peer
	channelID string
	testDir   string
}

func (nh *networkHelper) deployChaincode(chaincode nwo.Chaincode) {
	nwo.DeployChaincode(nh.Network, nh.channelID, nh.orderer, chaincode)
	nh.waitUntilEqualLedgerHeight(nh.getLedgerHeight(nh.peers[0]))
}

func (nh *networkHelper) waitUntilEqualLedgerHeight(height int) {
	for _, peer := range nh.peers {
		Eventually(func() int {
			return nh.getLedgerHeight(peer)
		}, nh.EventuallyTimeout).Should(Equal(height))
	}
}

func (nh *networkHelper) getLedgerHeight(peer *nwo.Peer) int {
	sess, err := nh.PeerUserSession(peer, "User1", commands.ChannelInfo{
		ChannelID: nh.channelID,
	})
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess, nh.EventuallyTimeout).Should(gexec.Exit(0))

	channelInfoStr := strings.TrimPrefix(string(sess.Buffer().Contents()[:]), "Blockchain info:")
	var channelInfo = common.BlockchainInfo{}
	json.Unmarshal([]byte(channelInfoStr), &channelInfo)
	return int(channelInfo.Height)
}

func (nh *networkHelper) queryChaincode(peer *nwo.Peer, command commands.ChaincodeQuery, expectedMessage string, expectSuccess bool) {
	sess, err := nh.PeerUserSession(peer, "User1", command)
	Expect(err).NotTo(HaveOccurred())
	if expectSuccess {
		Eventually(sess, nh.EventuallyTimeout).Should(gexec.Exit(0))
		Expect(sess).To(gbytes.Say(expectedMessage))
	} else {
		Eventually(sess, nh.EventuallyTimeout).Should(gexec.Exit(1))
		Expect(sess.Err).To(gbytes.Say(expectedMessage))
	}
}

func (nh *networkHelper) invokeChaincode(peer *nwo.Peer, command commands.ChaincodeInvoke) {
	sess, err := nh.PeerUserSession(peer, "User1", command)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess, nh.EventuallyTimeout).Should(gexec.Exit(0))
	Expect(sess.Err).To(gbytes.Say("Chaincode invoke successful."))
}

func (nh *networkHelper) rollback(peer *nwo.Peer, blockNumber int, expectedErrMessage string, expectSuccess bool) {
	rollbackCmd := commands.NodeRollback{ChannelID: nh.channelID, BlockNumber: blockNumber}
	sess, err := nh.PeerUserSession(peer, "User1", rollbackCmd)
	Expect(err).NotTo(HaveOccurred())
	if expectSuccess {
		Eventually(sess, nh.EventuallyTimeout).Should(gexec.Exit(0))
	} else {
		Eventually(sess, nh.EventuallyTimeout).Should(gexec.Exit(1))
		Expect(sess.Err).To(gbytes.Say(expectedErrMessage))
	}
}

func (nh *networkHelper) reset(peer *nwo.Peer, expectedErrMessage string, expectSuccess bool) {
	resetCmd := commands.NodeReset{}
	sess, err := nh.PeerUserSession(peer, "User1", resetCmd)
	Expect(err).NotTo(HaveOccurred())
	if expectSuccess {
		Eventually(sess, nh.EventuallyTimeout).Should(gexec.Exit(0))
	} else {
		Eventually(sess, nh.EventuallyTimeout).Should(gexec.Exit(1))
		Expect(sess.Err).To(gbytes.Say(expectedErrMessage))
	}
}

func (nh *networkHelper) pause(peer *nwo.Peer, expectedErrMessage string, expectSuccess bool) {
	pauseCmd := commands.NodePause{ChannelID: nh.channelID}
	sess, err := nh.PeerUserSession(peer, "User1", pauseCmd)
	Expect(err).NotTo(HaveOccurred())
	if expectSuccess {
		Eventually(sess, nh.EventuallyTimeout).Should(gexec.Exit(0))
	} else {
		Eventually(sess, nh.EventuallyTimeout).Should(gexec.Exit(1))
		Expect(sess.Err).To(gbytes.Say(expectedErrMessage))
	}
}

func (nh *networkHelper) resume(peer *nwo.Peer, expectedErrMessage string, expectSuccess bool) {
	resumeCmd := commands.NodeResume{ChannelID: nh.channelID}
	sess, err := nh.PeerUserSession(peer, "User1", resumeCmd)
	Expect(err).NotTo(HaveOccurred())
	if expectSuccess {
		Eventually(sess, nh.EventuallyTimeout).Should(gexec.Exit(0))
	} else {
		Eventually(sess, nh.EventuallyTimeout).Should(gexec.Exit(1))
		Expect(sess.Err).To(gbytes.Say(expectedErrMessage))
	}
}

func (nh *networkHelper) waitUntilEndorserEnabled(peer *nwo.Peer) {
	Eventually(func() *gbytes.Buffer {
		sess, err := nh.PeerUserSession(peer, "User1", commands.ChannelInfo{
			ChannelID: nh.channelID,
		})
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess, nh.EventuallyTimeout).Should(gexec.Exit())
		return sess.Buffer()
	}, nh.EventuallyTimeout).Should(gbytes.Say("Blockchain info"))
}

type testHelper struct {
	*networkHelper
}

func (th *testHelper) addMarble(chaincodeName, marbleDetails string, peer *nwo.Peer) {
	marbleDetailsBase64 := base64.StdEncoding.EncodeToString([]byte(marbleDetails))

	command := commands.ChaincodeInvoke{
		ChannelID: th.channelID,
		Orderer:   th.OrdererAddress(th.orderer, nwo.ListenPort),
		Name:      chaincodeName,
		Ctor:      fmt.Sprintf(`{"Args":["initMarble"]}`),
		Transient: fmt.Sprintf(`{"marble":"%s"}`, marbleDetailsBase64),
		PeerAddresses: []string{
			th.PeerAddress(peer, nwo.ListenPort),
		},
		WaitForEvent: true,
	}
	th.invokeChaincode(peer, command)
}

// assertPresentInCollectionM asserts that the private data for given marble is present in collection
// 'readMarble' at the given peers
func (th *testHelper) assertPresentInCollectionM(chaincodeName, marbleName string, peerList ...*nwo.Peer) {
	command := commands.ChaincodeQuery{
		ChannelID: th.channelID,
		Name:      chaincodeName,
		Ctor:      fmt.Sprintf(`{"Args":["readMarble","%s"]}`, marbleName),
	}
	expectedMsg := fmt.Sprintf(`{"docType":"marble","name":"%s"`, marbleName)
	for _, peer := range peerList {
		th.queryChaincode(peer, command, expectedMsg, true)
	}
}

// assertPresentInCollectionMPD asserts that the private data for given marble is present
// in collection 'readMarblePrivateDetails' at the given peers
func (th *testHelper) assertPresentInCollectionMPD(chaincodeName, marbleName string, peerList ...*nwo.Peer) {
	command := commands.ChaincodeQuery{
		ChannelID: th.channelID,
		Name:      chaincodeName,
		Ctor:      fmt.Sprintf(`{"Args":["readMarblePrivateDetails","%s"]}`, marbleName),
	}
	expectedMsg := fmt.Sprintf(`{"docType":"marblePrivateDetails","name":"%s"`, marbleName)
	for _, peer := range peerList {
		th.queryChaincode(peer, command, expectedMsg, true)
	}
}

func (th *testHelper) assertDisabledEndorser(chaincodeName string, peer *nwo.Peer) {
	command := commands.ChaincodeQuery{
		ChannelID: th.channelID,
		Name:      chaincodeName,
		Ctor:      `{"Args":["readMarble","marble1"]}`,
	}
	expectedMsg := "endorse requests are blocked while ledgers are being rebuilt"
	th.queryChaincode(peer, command, expectedMsg, false)
}

func (th *testHelper) assertPausedChannel(peer *nwo.Peer) {
	sess, err := th.PeerUserSession(peer, "User1", commands.ChannelInfo{
		ChannelID: th.channelID,
	})
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess, th.EventuallyTimeout).Should(gexec.Exit(1))
	Expect(sess.Err).To(gbytes.Say("Invalid chain ID"))
}
