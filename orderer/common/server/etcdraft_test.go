/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package server_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var basePort = int32(8000)

func nextPort() int32 {
	return atomic.AddInt32(&basePort, 1)
}

func TestSpawnEtcdRaft(t *testing.T) {
	gt := NewGomegaWithT(t)

	// Set the fabric root folder for easy navigation to sampleconfig folder
	fabricRootDir, err := filepath.Abs(filepath.Join("..", "..", ".."))
	gt.Expect(err).NotTo(HaveOccurred())

	// Build the configtxgen binary
	configtxgen, err := gexec.Build("github.com/hyperledger/fabric/cmd/configtxgen")
	gt.Expect(err).NotTo(HaveOccurred())

	// Build the orderer binary
	orderer, err := gexec.Build("github.com/hyperledger/fabric/cmd/orderer")
	gt.Expect(err).NotTo(HaveOccurred())

	defer gexec.CleanupBuildArtifacts()

	t.Run("Bad", func(t *testing.T) {
		gt = NewGomegaWithT(t)
		tempDir, err := ioutil.TempDir("", "etcdraft-orderer-launch")
		gt.Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tempDir)

		t.Run("Invalid bootstrap block", func(t *testing.T) {
			testEtcdRaftOSNFailureInvalidBootstrapBlock(NewGomegaWithT(t), tempDir, orderer, fabricRootDir, configtxgen)
		})

		t.Run("TLS disabled single listener", func(t *testing.T) {
			testEtcdRaftOSNNoTLSSingleListener(NewGomegaWithT(t), tempDir, orderer, fabricRootDir, configtxgen)
		})
	})

	t.Run("Good", func(t *testing.T) {
		// tests in this suite actually launch process with success, hence we need to avoid
		// conflicts in listening port, opening files.
		t.Run("TLS disabled dual listener", func(t *testing.T) {
			gt = NewGomegaWithT(t)
			tempDir, err := ioutil.TempDir("", "etcdraft-orderer-launch")
			gt.Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll(tempDir)

			testEtcdRaftOSNNoTLSDualListener(gt, tempDir, orderer, fabricRootDir, configtxgen)
		})

		t.Run("TLS enabled single listener", func(t *testing.T) {
			gt = NewGomegaWithT(t)
			tempDir, err := ioutil.TempDir("", "etcdraft-orderer-launch")
			gt.Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll(tempDir)

			testEtcdRaftOSNSuccess(gt, tempDir, configtxgen, orderer, fabricRootDir)
		})
	})
}

func createBootstrapBlock(gt *GomegaWithT, tempDir, configtxgen, channel, profile string) string {
	// create a genesis block for the specified channel and profile
	genesisBlockPath := filepath.Join(tempDir, "genesis.block")
	cmd := exec.Command(configtxgen, "-channelID", channel, "-profile", profile,
		"-outputBlock", genesisBlockPath)
	cmd.Env = append(cmd.Env, "FABRIC_CFG_PATH=testdata")
	configtxgenProcess, err := gexec.Start(cmd, nil, nil)
	gt.Expect(err).NotTo(HaveOccurred())
	gt.Eventually(configtxgenProcess, time.Minute).Should(gexec.Exit(0))
	gt.Expect(configtxgenProcess.Err).To(gbytes.Say("Writing genesis block"))

	return genesisBlockPath
}

func testEtcdRaftOSNSuccess(gt *GomegaWithT, tempDir, configtxgen, orderer, fabricRootDir string) {
	genesisBlockPath := createBootstrapBlock(gt, tempDir, configtxgen, "system", "SampleEtcdRaftSystemChannel")

	// Launch the OSN
	ordererProcess := launchOrderer(gt, orderer, tempDir, genesisBlockPath, fabricRootDir)
	defer ordererProcess.Kill()
	// The following configuration parameters are not specified in the orderer.yaml, so let's ensure
	// they are really configured autonomously via the localconfig code.
	gt.Eventually(ordererProcess.Err, time.Minute).Should(gbytes.Say("General.Cluster.DialTimeout = 5s"))
	gt.Eventually(ordererProcess.Err, time.Minute).Should(gbytes.Say("General.Cluster.RPCTimeout = 7s"))
	gt.Eventually(ordererProcess.Err, time.Minute).Should(gbytes.Say("General.Cluster.ReplicationBufferSize = 20971520"))
	gt.Eventually(ordererProcess.Err, time.Minute).Should(gbytes.Say("General.Cluster.ReplicationPullTimeout = 5s"))
	gt.Eventually(ordererProcess.Err, time.Minute).Should(gbytes.Say("General.Cluster.ReplicationRetryTimeout = 5s"))
	gt.Eventually(ordererProcess.Err, time.Minute).Should(gbytes.Say("General.Cluster.ReplicationBackgroundRefreshInterval = 5m0s"))
	gt.Eventually(ordererProcess.Err, time.Minute).Should(gbytes.Say("General.Cluster.ReplicationMaxRetries = 12"))
	gt.Eventually(ordererProcess.Err, time.Minute).Should(gbytes.Say("General.Cluster.SendBufferSize = 10"))
	gt.Eventually(ordererProcess.Err, time.Minute).Should(gbytes.Say("General.Cluster.CertExpirationWarningThreshold = 168h0m0s"))

	// Consensus.EvictionSuspicion is not specified in orderer.yaml, so let's ensure
	// it is really configured autonomously via the etcdraft chain itself.
	gt.Eventually(ordererProcess.Err, time.Minute).Should(gbytes.Say("EvictionSuspicion not set, defaulting to 10m"))
	// Wait until the the node starts up and elects itself as a single leader in a single node cluster.
	gt.Eventually(ordererProcess.Err, time.Minute).Should(gbytes.Say("Beginning to serve requests"))
	gt.Eventually(ordererProcess.Err, time.Minute).Should(gbytes.Say("becomeLeader"))
}

func testEtcdRaftOSNFailureInvalidBootstrapBlock(gt *GomegaWithT, tempDir, orderer, fabricRootDir, configtxgen string) {
	// create an application channel genesis block
	genesisBlockPath := createBootstrapBlock(gt, tempDir, configtxgen, "mychannel", "SampleOrgChannel")
	genesisBlockBytes, err := ioutil.ReadFile(genesisBlockPath)
	gt.Expect(err).NotTo(HaveOccurred())

	// Copy it to the designated location in the temporary folder
	genesisBlockPath = filepath.Join(tempDir, "genesis.block")
	err = ioutil.WriteFile(genesisBlockPath, genesisBlockBytes, 0644)
	gt.Expect(err).NotTo(HaveOccurred())

	// Launch the OSN
	ordererProcess := launchOrderer(gt, orderer, tempDir, genesisBlockPath, fabricRootDir)
	defer ordererProcess.Kill()

	expectedErr := "Failed validating bootstrap block: the block isn't a system channel block because it lacks ConsortiumsConfig"
	gt.Eventually(ordererProcess.Err, time.Minute).Should(gbytes.Say(expectedErr))
}

func testEtcdRaftOSNNoTLSSingleListener(gt *GomegaWithT, tempDir, orderer, fabricRootDir string, configtxgen string) {
	genesisBlockPath := createBootstrapBlock(gt, tempDir, configtxgen, "system", "SampleEtcdRaftSystemChannel")

	cmd := exec.Command(orderer)
	cmd.Env = []string{
		fmt.Sprintf("ORDERER_GENERAL_LISTENPORT=%d", nextPort()),
		"ORDERER_GENERAL_GENESISMETHOD=file",
		"ORDERER_GENERAL_SYSTEMCHANNEL=system",
		fmt.Sprintf("ORDERER_FILELEDGER_LOCATION=%s", filepath.Join(tempDir, "ledger")),
		fmt.Sprintf("ORDERER_GENERAL_BOOTSTRAPFILE=%s", genesisBlockPath),
		fmt.Sprintf("FABRIC_CFG_PATH=%s", filepath.Join(fabricRootDir, "sampleconfig")),
	}
	ordererProcess, err := gexec.Start(cmd, nil, nil)
	gt.Expect(err).NotTo(HaveOccurred())
	defer ordererProcess.Kill()

	expectedErr := "TLS is required for running ordering nodes of type etcdraft."
	gt.Eventually(ordererProcess.Err, time.Minute).Should(gbytes.Say(expectedErr))
}

func testEtcdRaftOSNNoTLSDualListener(gt *GomegaWithT, tempDir, orderer, fabricRootDir string, configtxgen string) {
	cwd, err := os.Getwd()
	gt.Expect(err).NotTo(HaveOccurred())

	genesisBlockPath := createBootstrapBlock(gt, tempDir, configtxgen, "system", "SampleEtcdRaftSystemChannel")

	cmd := exec.Command(orderer)
	cmd.Env = []string{
		fmt.Sprintf("ORDERER_GENERAL_LISTENPORT=%d", nextPort()),
		"ORDERER_GENERAL_GENESISMETHOD=file",
		"ORDERER_GENERAL_SYSTEMCHANNEL=system",
		"ORDERER_GENERAL_TLS_ENABLED=false",
		"ORDERER_OPERATIONS_TLS_ENABLED=false",
		fmt.Sprintf("ORDERER_FILELEDGER_LOCATION=%s", filepath.Join(tempDir, "ledger")),
		fmt.Sprintf("ORDERER_GENERAL_BOOTSTRAPFILE=%s", genesisBlockPath),
		fmt.Sprintf("ORDERER_GENERAL_CLUSTER_LISTENPORT=%d", nextPort()),
		"ORDERER_GENERAL_CLUSTER_LISTENADDRESS=127.0.0.1",
		fmt.Sprintf("ORDERER_GENERAL_CLUSTER_SERVERCERTIFICATE=%s", filepath.Join(cwd, "testdata", "example.com", "tls", "server.crt")),
		fmt.Sprintf("ORDERER_GENERAL_CLUSTER_SERVERPRIVATEKEY=%s", filepath.Join(cwd, "testdata", "example.com", "tls", "server.key")),
		fmt.Sprintf("ORDERER_GENERAL_CLUSTER_CLIENTCERTIFICATE=%s", filepath.Join(cwd, "testdata", "example.com", "tls", "server.crt")),
		fmt.Sprintf("ORDERER_GENERAL_CLUSTER_CLIENTPRIVATEKEY=%s", filepath.Join(cwd, "testdata", "example.com", "tls", "server.key")),
		fmt.Sprintf("ORDERER_GENERAL_CLUSTER_ROOTCAS=[%s]", filepath.Join(cwd, "testdata", "example.com", "tls", "ca.crt")),
		fmt.Sprintf("ORDERER_CONSENSUS_WALDIR=%s", filepath.Join(tempDir, "wal")),
		fmt.Sprintf("ORDERER_CONSENSUS_SNAPDIR=%s", filepath.Join(tempDir, "snapshot")),
		fmt.Sprintf("FABRIC_CFG_PATH=%s", filepath.Join(fabricRootDir, "sampleconfig")),
		"ORDERER_OPERATIONS_LISTENADDRESS=127.0.0.1:0",
	}
	ordererProcess, err := gexec.Start(cmd, nil, nil)
	gt.Expect(err).NotTo(HaveOccurred())
	defer ordererProcess.Kill()

	gt.Eventually(ordererProcess.Err, time.Minute).Should(gbytes.Say("Beginning to serve requests"))
	gt.Eventually(ordererProcess.Err, time.Minute).Should(gbytes.Say("becomeLeader"))
}

func launchOrderer(gt *GomegaWithT, orderer, tempDir, genesisBlockPath, fabricRootDir string) *gexec.Session {
	cwd, err := os.Getwd()
	gt.Expect(err).NotTo(HaveOccurred())

	// Launch the orderer process
	cmd := exec.Command(orderer)
	cmd.Env = []string{
		fmt.Sprintf("ORDERER_GENERAL_LISTENPORT=%d", nextPort()),
		"ORDERER_GENERAL_GENESISMETHOD=file",
		"ORDERER_GENERAL_SYSTEMCHANNEL=system",
		"ORDERER_GENERAL_TLS_CLIENTAUTHREQUIRED=true",
		"ORDERER_GENERAL_TLS_ENABLED=true",
		"ORDERER_OPERATIONS_TLS_ENABLED=false",
		fmt.Sprintf("ORDERER_FILELEDGER_LOCATION=%s", filepath.Join(tempDir, "ledger")),
		fmt.Sprintf("ORDERER_GENERAL_BOOTSTRAPFILE=%s", genesisBlockPath),
		fmt.Sprintf("ORDERER_GENERAL_CLUSTER_LISTENPORT=%d", nextPort()),
		"ORDERER_GENERAL_CLUSTER_LISTENADDRESS=127.0.0.1",
		fmt.Sprintf("ORDERER_GENERAL_CLUSTER_SERVERCERTIFICATE=%s", filepath.Join(cwd, "testdata", "example.com", "tls", "server.crt")),
		fmt.Sprintf("ORDERER_GENERAL_CLUSTER_SERVERPRIVATEKEY=%s", filepath.Join(cwd, "testdata", "example.com", "tls", "server.key")),
		fmt.Sprintf("ORDERER_GENERAL_CLUSTER_CLIENTCERTIFICATE=%s", filepath.Join(cwd, "testdata", "example.com", "tls", "server.crt")),
		fmt.Sprintf("ORDERER_GENERAL_CLUSTER_CLIENTPRIVATEKEY=%s", filepath.Join(cwd, "testdata", "example.com", "tls", "server.key")),
		fmt.Sprintf("ORDERER_GENERAL_CLUSTER_ROOTCAS=[%s]", filepath.Join(cwd, "testdata", "example.com", "tls", "ca.crt")),
		fmt.Sprintf("ORDERER_GENERAL_TLS_ROOTCAS=[%s]", filepath.Join(cwd, "testdata", "example.com", "tls", "ca.crt")),
		fmt.Sprintf("ORDERER_GENERAL_TLS_CERTIFICATE=%s", filepath.Join(cwd, "testdata", "example.com", "tls", "server.crt")),
		fmt.Sprintf("ORDERER_GENERAL_TLS_PRIVATEKEY=%s", filepath.Join(cwd, "testdata", "example.com", "tls", "server.key")),
		fmt.Sprintf("ORDERER_CONSENSUS_WALDIR=%s", filepath.Join(tempDir, "wal")),
		fmt.Sprintf("ORDERER_CONSENSUS_SNAPDIR=%s", filepath.Join(tempDir, "snapshot")),
		fmt.Sprintf("FABRIC_CFG_PATH=%s", filepath.Join(fabricRootDir, "sampleconfig")),
	}
	sess, err := gexec.Start(cmd, nil, nil)
	gt.Expect(err).NotTo(HaveOccurred())
	return sess
}
