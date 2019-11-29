/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package scc_test

import (
	"os"
	"testing"

	"github.com/hyperledger/fabric/core/chaincode/lifecycle"
	"github.com/hyperledger/fabric/core/scc"
	"github.com/hyperledger/fabric/core/scc/mock"
	xtestutil "github.com/hyperledger/fabric/extensions/testutil"
	"github.com/onsi/gomega"
)

//go:generate counterfeiter -o mock/chaincode_stream_handler.go --fake-name ChaincodeStreamHandler . chaincodeStreamHandler
type chaincodeStreamHandler interface {
	scc.ChaincodeStreamHandler
}

func TestMain(m *testing.M) {
	//setup extension test environment
	_, _, destroy := xtestutil.SetupExtTestEnv()

	code := m.Run()
	destroy()
	os.Exit(code)
}

func TestDeploy(t *testing.T) {
	gt := gomega.NewGomegaWithT(t)

	csh := &mock.ChaincodeStreamHandler{}
	doneC := make(chan struct{})
	close(doneC)
	csh.LaunchInProcReturns(doneC)
	scc.DeploySysCC(&lifecycle.SCC{}, csh)
	gt.Expect(csh.LaunchInProcCallCount()).To(gomega.Equal(1))
	gt.Expect(csh.LaunchInProcArgsForCall(0)).To(gomega.Equal("_lifecycle.syscc"))
	gt.Eventually(csh.HandleChaincodeStreamCallCount).Should(gomega.Equal(1))
}
