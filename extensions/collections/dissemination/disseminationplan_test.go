/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package dissemination

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDisseminationPlan(t *testing.T) {
	assert.PanicsWithValue(t, "not implemented",
		func() {
			ComputeDisseminationPlan("testchannel", "ns1", nil, nil, nil, nil, nil)
		},
	)
}
