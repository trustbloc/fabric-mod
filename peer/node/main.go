/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package node

// Start starts the peer
func Start() error {
	return serve([]string{})
}
