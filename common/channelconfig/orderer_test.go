/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package channelconfig

import (
	"testing"

	ab "github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/stretchr/testify/require"
)

func TestBatchSize(t *testing.T) {
	validMaxMessageCount := uint32(10)
	validAbsoluteMaxBytes := uint32(1000)
	validPreferredMaxBytes := uint32(500)

	oc := &OrdererConfig{protos: &OrdererProtos{BatchSize: &ab.BatchSize{MaxMessageCount: validMaxMessageCount, AbsoluteMaxBytes: validAbsoluteMaxBytes, PreferredMaxBytes: validPreferredMaxBytes}}}
	require.NoError(t, oc.validateBatchSize(), "BatchSize was valid")

	oc = &OrdererConfig{protos: &OrdererProtos{BatchSize: &ab.BatchSize{MaxMessageCount: 0, AbsoluteMaxBytes: validAbsoluteMaxBytes, PreferredMaxBytes: validPreferredMaxBytes}}}
	require.Error(t, oc.validateBatchSize(), "MaxMessageCount was zero")

	oc = &OrdererConfig{protos: &OrdererProtos{BatchSize: &ab.BatchSize{MaxMessageCount: validMaxMessageCount, AbsoluteMaxBytes: 0, PreferredMaxBytes: validPreferredMaxBytes}}}
	require.Error(t, oc.validateBatchSize(), "AbsoluteMaxBytes was zero")

	oc = &OrdererConfig{protos: &OrdererProtos{BatchSize: &ab.BatchSize{MaxMessageCount: validMaxMessageCount, AbsoluteMaxBytes: validAbsoluteMaxBytes, PreferredMaxBytes: validAbsoluteMaxBytes + 1}}}
	require.Error(t, oc.validateBatchSize(), "PreferredMaxBytes larger to AbsoluteMaxBytes")
}

func TestBatchTimeout(t *testing.T) {
	oc := &OrdererConfig{protos: &OrdererProtos{BatchTimeout: &ab.BatchTimeout{Timeout: "1s"}}}
	require.NoError(t, oc.validateBatchTimeout(), "Valid batch timeout")

	oc = &OrdererConfig{protos: &OrdererProtos{BatchTimeout: &ab.BatchTimeout{Timeout: "-1s"}}}
	require.Error(t, oc.validateBatchTimeout(), "Negative batch timeout")

	oc = &OrdererConfig{protos: &OrdererProtos{BatchTimeout: &ab.BatchTimeout{Timeout: "0s"}}}
	require.Error(t, oc.validateBatchTimeout(), "Zero batch timeout")
}

func TestKafkaBrokers(t *testing.T) {
	oc := &OrdererConfig{protos: &OrdererProtos{KafkaBrokers: &ab.KafkaBrokers{Brokers: []string{"127.0.0.1:9092", "foo.bar:9092"}}}}
	require.NoError(t, oc.validateKafkaBrokers(), "Valid kafka brokers")

	oc = &OrdererConfig{protos: &OrdererProtos{KafkaBrokers: &ab.KafkaBrokers{Brokers: []string{"127.0.0.1", "foo.bar", "127.0.0.1:-1", "localhost:65536", "foo.bar.:9092", ".127.0.0.1:9092", "-foo.bar:9092"}}}}
	require.Error(t, oc.validateKafkaBrokers(), "Invalid kafka brokers")
}
