/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"github.com/hyperledger/fabric/internal/peer/common"
	"github.com/spf13/cobra"
)

const CmdRoot = common.CmdRoot

// InitCmd initiated the fabric peer
func InitCmd(cmd *cobra.Command, args []string) {
	common.InitCmd(cmd, args)
}
