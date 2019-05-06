/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package policy

import (
	"testing"

	"github.com/hyperledger/fabric/protos/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCollection(t *testing.T) {
	v := NewValidator()
	require.NotNil(t, v)

	config := &common.CollectionConfig{
		Payload: &common.CollectionConfig_StaticCollectionConfig{
			StaticCollectionConfig: &common.StaticCollectionConfig{},
		},
	}

	err := v.Validate(config)
	assert.NoError(t, err)

	newCollectionConfigs := []*common.CollectionConfig{config}
	oldCollectionConfigs := []*common.CollectionConfig{config}

	err = v.ValidateNewCollectionConfigsAgainstOld(newCollectionConfigs, oldCollectionConfigs)
	assert.NoError(t, err)
}
