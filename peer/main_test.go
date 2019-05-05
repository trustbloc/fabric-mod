/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main_test

import (
	"fmt"
	"io/ioutil"
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
	peer, err := gexec.Build("github.com/hyperledger/fabric/peer")
	gt.Expect(err).NotTo(HaveOccurred())
	defer gexec.CleanupBuildArtifacts()

	parentDir, err := filepath.Abs("..")
	gt.Expect(err).NotTo(HaveOccurred())

	tempDir, err := ioutil.TempDir("", "plugin-failure")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	for _, plugin := range []string{
		"ENDORSERS_ESCC",
		"VALIDATORS_VSCC",
	} {
		plugin := plugin
		t.Run(plugin, func(t *testing.T) {
			cmd := exec.Command(peer, "node", "start")
			cmd.Env = []string{
				fmt.Sprintf("CORE_PEER_FILESYSTEMPATH=%s", tempDir),
				fmt.Sprintf("CORE_PEER_HANDLERS_%s_LIBRARY=testdata/invalid_plugins/invalidplugin.so", plugin),
				fmt.Sprintf("CORE_PEER_MSPCONFIGPATH=%s", "msp"),
				fmt.Sprintf("FABRIC_CFG_PATH=%s", filepath.Join(parentDir, "sampleconfig")),
				"CORE_OPERATIONS_TLS_ENABLED=false",
				fmt.Sprintf("CORE_LEDGER_STATE_COUCHDBCONFIG_COUCHDBADDRESS=%s", addr),
				fmt.Sprintf("CORE_LEDGER_STATE_COUCHDBCONFIG_USERNAME=%s", ""),
				fmt.Sprintf("CORE_LEDGER_STATE_COUCHDBCONFIG_PASSWORD=%s", ""),
				fmt.Sprintf("CORE_LEDGER_STATE_COUCHDBCONFIG_MAXRETRIES=%d", 3),
				fmt.Sprintf("CORE_LEDGER_STATE_COUCHDBCONFIG_MAXRETRIESONSTARTUP=%v", 1),
			}

			sess, err := gexec.Start(cmd, nil, nil)
			gt.Expect(err).NotTo(HaveOccurred())
			gt.Eventually(sess, time.Minute).Should(gexec.Exit(2))

			gt.Expect(sess.Err).To(gbytes.Say("panic: Error opening plugin at path testdata/invalid_plugins/invalidplugin.so"))
			gt.Expect(sess.Err).To(gbytes.Say("plugin.Open"))
		})
	}
}
