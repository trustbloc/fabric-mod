/*
Copyright IBM Corp. 2017 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package msgprocessor

import (
	"fmt"
	"testing"

	"github.com/hyperledger/fabric/common/channelconfig"
	mockconfig "github.com/hyperledger/fabric/common/mocks/config"
	"github.com/hyperledger/fabric/internal/pkg/identity"
	cb "github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/orderer"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/stretchr/testify/assert"
)

const testChannelID = "foo"

type mockSystemChannelFilterSupport struct {
	ProposeConfigUpdateVal *cb.ConfigEnvelope
	ProposeConfigUpdateErr error
	SequenceVal            uint64
	ConsensusTypeStateVal  orderer.ConsensusType_State
}

func (ms *mockSystemChannelFilterSupport) ProposeConfigUpdate(env *cb.Envelope) (*cb.ConfigEnvelope, error) {
	return ms.ProposeConfigUpdateVal, ms.ProposeConfigUpdateErr
}

func (ms *mockSystemChannelFilterSupport) Sequence() uint64 {
	return ms.SequenceVal
}

func (ms *mockSystemChannelFilterSupport) Signer() identity.SignerSerializer {
	return nil
}

func (ms *mockSystemChannelFilterSupport) ChainID() string {
	return testChannelID
}

func (ms *mockSystemChannelFilterSupport) OrdererConfig() (channelconfig.Orderer, bool) {
	return &mockconfig.Orderer{
			CapabilitiesVal:       &mockconfig.OrdererCapabilities{ConsensusTypeMigrationVal: true},
			ConsensusTypeStateVal: ms.ConsensusTypeStateVal,
		},
		true
}

func TestClassifyMsg(t *testing.T) {
	t.Run("ConfigUpdate", func(t *testing.T) {
		class := (&StandardChannel{}).ClassifyMsg(&cb.ChannelHeader{Type: int32(cb.HeaderType_CONFIG_UPDATE)})
		assert.Equal(t, class, ConfigUpdateMsg)
	})
	t.Run("OrdererTx", func(t *testing.T) {
		class := (&StandardChannel{}).ClassifyMsg(&cb.ChannelHeader{Type: int32(cb.HeaderType_ORDERER_TRANSACTION)})
		assert.Equal(t, class, ConfigMsg)
	})
	t.Run("ConfigTx", func(t *testing.T) {
		class := (&StandardChannel{}).ClassifyMsg(&cb.ChannelHeader{Type: int32(cb.HeaderType_CONFIG)})
		assert.Equal(t, class, ConfigMsg)
	})
	t.Run("EndorserTx", func(t *testing.T) {
		class := (&StandardChannel{}).ClassifyMsg(&cb.ChannelHeader{Type: int32(cb.HeaderType_ENDORSER_TRANSACTION)})
		assert.Equal(t, class, NormalMsg)
	})
}

func TestProcessNormalMsg(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		ms := &mockSystemChannelFilterSupport{
			SequenceVal:           7,
			ConsensusTypeStateVal: orderer.ConsensusType_STATE_NORMAL,
		}
		cs, err := NewStandardChannel(ms, NewRuleSet([]Rule{AcceptRule})).ProcessNormalMsg(nil)
		assert.Equal(t, cs, ms.SequenceVal)
		assert.Nil(t, err)
	})
	t.Run("Maintenance", func(t *testing.T) {
		ms := &mockSystemChannelFilterSupport{
			SequenceVal:           7,
			ConsensusTypeStateVal: orderer.ConsensusType_STATE_MAINTENANCE,
		}
		_, err := NewStandardChannel(ms, NewRuleSet([]Rule{AcceptRule})).ProcessNormalMsg(nil)
		assert.EqualError(t, err, "normal transactions are rejected: maintenance mode")
	})
}

func TestConfigUpdateMsg(t *testing.T) {
	t.Run("BadUpdate", func(t *testing.T) {
		ms := &mockSystemChannelFilterSupport{
			ProposeConfigUpdateVal: &cb.ConfigEnvelope{},
			ProposeConfigUpdateErr: fmt.Errorf("An error"),
		}
		config, cs, err := NewStandardChannel(ms, NewRuleSet(nil)).ProcessConfigUpdateMsg(&cb.Envelope{})
		assert.Nil(t, config)
		assert.Equal(t, uint64(0), cs)
		assert.EqualError(t, err, "error applying config update to existing channel 'foo': An error")
	})
	t.Run("BadMsg", func(t *testing.T) {
		ms := &mockSystemChannelFilterSupport{
			ProposeConfigUpdateVal: &cb.ConfigEnvelope{},
			ProposeConfigUpdateErr: fmt.Errorf("An error"),
		}
		config, cs, err := NewStandardChannel(ms, NewRuleSet([]Rule{EmptyRejectRule})).ProcessConfigUpdateMsg(&cb.Envelope{})
		assert.Nil(t, config)
		assert.Equal(t, uint64(0), cs)
		assert.NotNil(t, err)
	})
	t.Run("SignedEnvelopeFailure", func(t *testing.T) {
		ms := &mockSystemChannelFilterSupport{}
		config, cs, err := NewStandardChannel(ms, NewRuleSet([]Rule{AcceptRule})).ProcessConfigUpdateMsg(nil)
		assert.Nil(t, config)
		assert.Equal(t, uint64(0), cs)
		assert.NotNil(t, err)
		assert.Regexp(t, "Marshal called with nil", err)
	})
	t.Run("Success", func(t *testing.T) {
		ms := &mockSystemChannelFilterSupport{
			SequenceVal:            7,
			ProposeConfigUpdateVal: &cb.ConfigEnvelope{},
		}
		config, cs, err := NewStandardChannel(ms, NewRuleSet([]Rule{AcceptRule})).ProcessConfigUpdateMsg(nil)
		assert.NotNil(t, config)
		assert.Equal(t, cs, ms.SequenceVal)
		assert.Nil(t, err)
	})
}

func TestProcessConfigMsg(t *testing.T) {
	t.Run("WrongType", func(t *testing.T) {
		ms := &mockSystemChannelFilterSupport{
			SequenceVal:            7,
			ProposeConfigUpdateVal: &cb.ConfigEnvelope{},
		}
		_, _, err := NewStandardChannel(ms, NewRuleSet([]Rule{AcceptRule})).ProcessConfigMsg(&cb.Envelope{
			Payload: protoutil.MarshalOrPanic(&cb.Payload{
				Header: &cb.Header{
					ChannelHeader: protoutil.MarshalOrPanic(&cb.ChannelHeader{
						ChannelId: testChannelID,
						Type:      int32(cb.HeaderType_ORDERER_TRANSACTION),
					}),
				},
			}),
		})
		assert.Error(t, err)
	})

	t.Run("Success", func(t *testing.T) {
		ms := &mockSystemChannelFilterSupport{
			SequenceVal:            7,
			ProposeConfigUpdateVal: &cb.ConfigEnvelope{},
		}
		config, cs, err := NewStandardChannel(ms, NewRuleSet([]Rule{AcceptRule})).ProcessConfigMsg(&cb.Envelope{
			Payload: protoutil.MarshalOrPanic(&cb.Payload{
				Header: &cb.Header{
					ChannelHeader: protoutil.MarshalOrPanic(&cb.ChannelHeader{
						ChannelId: testChannelID,
						Type:      int32(cb.HeaderType_CONFIG),
					}),
				},
			}),
		})
		assert.NotNil(t, config)
		assert.Equal(t, cs, ms.SequenceVal)
		assert.Nil(t, err)
		hdr, err := protoutil.ChannelHeader(config)
		assert.Equal(
			t,
			int32(cb.HeaderType_CONFIG),
			hdr.Type,
			"Expect type of returned envelope to be %d, but got %d", cb.HeaderType_CONFIG, hdr.Type)
	})
}
