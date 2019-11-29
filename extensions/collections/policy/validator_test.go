/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package policy

import (
	"testing"

	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCollection(t *testing.T) {
	v := NewValidator()
	require.NotNil(t, v)

	config := &peer.CollectionConfig{
		Payload: &peer.CollectionConfig_StaticCollectionConfig{
			StaticCollectionConfig: &peer.StaticCollectionConfig{},
		},
	}

	err := v.Validate(config)
	assert.NoError(t, err)

	newCollectionConfigs := []*peer.CollectionConfig{config}
	oldCollectionConfigs := []*peer.CollectionConfig{config}

	err = v.ValidateNewCollectionConfigsAgainstOld(newCollectionConfigs, oldCollectionConfigs)
	assert.NoError(t, err)
}
