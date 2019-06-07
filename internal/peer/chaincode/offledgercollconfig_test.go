/*
Copyright Digital Asset Holdings, LLC. All Rights Reserved.
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chaincode

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/cauthdsl"
	"github.com/stretchr/testify/require"
)

const sampleCollectionConfigOffLedger = `[
	{
		"name": "foo",
		"policy": "OR('A.member', 'B.member')",
		"requiredPeerCount": 3,
		"maxPeerCount": 5,
		"type": "OFFLEDGER",
		"timeToLive": "2m"
	}
]`

const sampleCollectionConfigDCAS = `[
	{
		"name": "foo",
		"policy": "OR('A.member', 'B.member')",
		"requiredPeerCount": 3,
		"maxPeerCount": 5,
		"type": "DCAS",
		"timeToLive": "2m"
	}
]`

func TestOffLedgerCollectionTypeParsing(t *testing.T) {
	pol, _ := cauthdsl.FromString("OR('A.member', 'B.member')")

	t.Run("OffLedger Collection Config", func(t *testing.T) {
		ccp, _, err := getCollectionConfigFromBytes([]byte(sampleCollectionConfigOffLedger))
		require.NoError(t, err)
		require.NotNil(t, ccp)
		conf := ccp.Config[0].GetStaticCollectionConfig()
		require.NotNil(t, conf)
		require.Equal(t, "foo", conf.Name)
		require.Equal(t, int32(3), conf.RequiredPeerCount)
		require.Equal(t, int32(5), conf.MaximumPeerCount)
		require.True(t, proto.Equal(pol, conf.MemberOrgsPolicy.GetSignaturePolicy()))
		require.Equal(t, "2m", conf.TimeToLive)
	})

	t.Run("DCAS Collection Config", func(t *testing.T) {
		ccp, _, err := getCollectionConfigFromBytes([]byte(sampleCollectionConfigDCAS))
		require.NoError(t, err)
		require.NotNil(t, ccp)
		conf := ccp.Config[0].GetStaticCollectionConfig()
		require.NotNil(t, conf)
		require.Equal(t, "foo", conf.Name)
		require.Equal(t, int32(3), conf.RequiredPeerCount)
		require.Equal(t, int32(5), conf.MaximumPeerCount)
		require.True(t, proto.Equal(pol, conf.MemberOrgsPolicy.GetSignaturePolicy()))
		require.Equal(t, "2m", conf.TimeToLive)
	})
}
