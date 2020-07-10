/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	xtestutil "github.com/hyperledger/fabric/extensions/testutil"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

func TestPluginLoadingFailure(t *testing.T) {
	//setup extension test environment
	addr, _, destroy := xtestutil.SetupExtTestEnv()
	defer destroy()
	gt := NewGomegaWithT(t)
	peer, err := gexec.Build("github.com/hyperledger/fabric/cmd/peer", "-tags", "testing")
	gt.Expect(err).NotTo(HaveOccurred())
	defer gexec.CleanupBuildArtifacts()

	parentDir, err := filepath.Abs("../..")
	gt.Expect(err).NotTo(HaveOccurred())

	tempDir, err := ioutil.TempDir("", "plugin-failure")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	peerListener, err := net.Listen("tcp", "localhost:0")
	gt.Expect(err).NotTo(HaveOccurred())
	peerListenAddress := peerListener.Addr()

	chaincodeListener, err := net.Listen("tcp", "localhost:0")
	gt.Expect(err).NotTo(HaveOccurred())
	chaincodeListenAddress := chaincodeListener.Addr()

	operationsListener, err := net.Listen("tcp", "localhost:0")
	gt.Expect(err).NotTo(HaveOccurred())
	operationsListenAddress := operationsListener.Addr()

	err = peerListener.Close()
	gt.Expect(err).NotTo(HaveOccurred())
	err = chaincodeListener.Close()
	gt.Expect(err).NotTo(HaveOccurred())
	err = operationsListener.Close()
	gt.Expect(err).NotTo(HaveOccurred())

	for _, plugin := range []string{
		"ENDORSERS_ESCC",
		"VALIDATORS_VSCC",
	} {
		plugin := plugin
		t.Run(plugin, func(t *testing.T) {
			cmd := exec.Command(peer, "node", "start")
			cmd.Env = []string{
				fmt.Sprintf("CORE_PEER_FILESYSTEMPATH=%s", tempDir),
				fmt.Sprintf("CORE_PEER_HANDLERS_%s_LIBRARY=%s", plugin, filepath.Join(parentDir, "internal/peer/testdata/invalid_plugins/invalidplugin.so")),
				fmt.Sprintf("CORE_PEER_LISTENADDRESS=%s", peerListenAddress),
				fmt.Sprintf("CORE_PEER_CHAINCODELISTENADDRESS=%s", chaincodeListenAddress),
				fmt.Sprintf("CORE_PEER_MSPCONFIGPATH=%s", "msp"),
				fmt.Sprintf("CORE_OPERATIONS_LISTENADDRESS=%s", operationsListenAddress),
				"CORE_OPERATIONS_TLS_ENABLED=false",
				fmt.Sprintf("FABRIC_CFG_PATH=%s", filepath.Join(parentDir, "sampleconfig")),
				"CORE_OPERATIONS_TLS_ENABLED=false",
				fmt.Sprintf("CORE_LEDGER_STATE_STATEDATABASE=%s", xtestutil.TestLedgerConf().StateDBConfig.StateDatabase),
				fmt.Sprintf("CORE_LEDGER_STATE_COUCHDBCONFIG_COUCHDBADDRESS=%s", addr),
				fmt.Sprintf("CORE_LEDGER_STATE_COUCHDBCONFIG_USERNAME=%s", "admin"),
				fmt.Sprintf("CORE_LEDGER_STATE_COUCHDBCONFIG_PASSWORD=%s", "adminpw"),
				fmt.Sprintf("CORE_LEDGER_STATE_COUCHDBCONFIG_MAXRETRIES=%d", 3),
				fmt.Sprintf("CORE_LEDGER_STATE_COUCHDBCONFIG_MAXRETRIESONSTARTUP=%v", 1),
			}
			sess, err := gexec.Start(cmd, nil, nil)
			gt.Expect(err).NotTo(HaveOccurred())
			gt.Eventually(sess, time.Minute).Should(gexec.Exit(2))

			gt.Expect(sess.Err).To(gbytes.Say(fmt.Sprintf("panic: Error opening plugin at path %s", filepath.Join(parentDir, "internal/peer/testdata/invalid_plugins/invalidplugin.so"))))
			gt.Expect(sess.Err).To(gbytes.Say("plugin.Open"))
		})
	}
}
