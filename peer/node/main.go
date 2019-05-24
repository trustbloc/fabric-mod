/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package node

import "github.com/hyperledger/fabric/internal/peer/node"

// Start starts the peer
func Start() error {
	return node.Serve([]string{})
}
