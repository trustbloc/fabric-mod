/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blockpublisher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	channelID = "testchannel"
)

func TestProvider(t *testing.T) {
	p := NewProvider()
	require.NotNil(t, p)

	publisher := p.ForChannel(channelID)
	require.NotNil(t, publisher)

	assert.NotPanics(t, func() {
		publisher.AddConfigUpdateHandler(nil)
		publisher.AddWriteHandler(nil)
		publisher.AddReadHandler(nil)
		publisher.AddCCUpgradeHandler(nil)
		publisher.AddCCEventHandler(nil)
		publisher.Publish(nil)
	})

	p.Close()
}
