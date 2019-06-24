/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package service

import (
	"testing"

	"github.com/hyperledger/fabric/protos/gossip"
	"github.com/stretchr/testify/require"
)

func TestHandleGossip(t *testing.T) {

	doneCh := make(chan bool, 1)
	handle := func(msg *gossip.GossipMessage) {
		doneCh <- true
	}
	HandleGossip(handle)(nil)

	select {
	case ok := <-doneCh:
		require.True(t, ok)
	default:
		t.Fatal("handler supposed to be executed")
	}
}
