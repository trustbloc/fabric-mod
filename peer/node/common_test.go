/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package node

import (
	"os"
	"testing"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/config/configtest"
	"github.com/hyperledger/fabric/internal/peer/common"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestInitCmd(t *testing.T) {
	cleanup := configtest.SetDevFabricConfigPath(t)
	defer cleanup()
	defer viper.Reset()

	// test that InitCmd doesn't remove existing loggers from the logger levels map
	flogging.MustGetLogger("test")
	flogging.ActivateSpec("test=error")
	assert.Equal(t, "error", flogging.Global.Level("test").String())
	flogging.MustGetLogger("chaincode")
	assert.Equal(t, flogging.Global.DefaultLevel().String(), flogging.Global.Level("chaincode").String())
	flogging.MustGetLogger("test.test2")
	flogging.ActivateSpec("test.test2=warn")
	assert.Equal(t, "warn", flogging.Global.Level("test.test2").String())

	origEnvValue := os.Getenv("FABRIC_LOGGING_SPEC")
	os.Setenv("FABRIC_LOGGING_SPEC", "chaincode=debug:test.test2=fatal:abc=error")
	common.InitCmd(nil, nil)
	assert.Equal(t, "debug", flogging.Global.Level("chaincode").String())
	assert.Equal(t, "info", flogging.Global.Level("test").String())
	assert.Equal(t, "fatal", flogging.Global.Level("test.test2").String())
	assert.Equal(t, "error", flogging.Global.Level("abc").String())
	os.Setenv("FABRIC_LOGGING_SPEC", origEnvValue)
}
