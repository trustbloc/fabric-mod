/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package configtest

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// AddDevConfigPath adds the DevConfigDir to the viper path.
func AddDevConfigPath(v *viper.Viper) {
	devPath := GetDevConfigDir()
	if v != nil {
		v.AddConfigPath(devPath)
	} else {
		viper.AddConfigPath(devPath)
	}
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

// GetDevConfigDir gets the path to the default configuration that is
// maintained with the source tree. This should only be used in a
// test/development context.
func GetDevConfigDir() string {
	sampleConfigPath := os.Getenv("FABRIC_SAMPLECONFIG_PATH")
	if sampleConfigPath != "" {
		path, err := gopathDevConfigDir(sampleConfigPath)
		if err != nil {
			panic(err)
		}

		return path
	}

	path, err := gomodDevConfigDir()
	if err != nil {
		path, err = gopathDevConfigDir("src/github.com/hyperledger/fabric/sampleconfig")
		if err != nil {
			panic(err)
		}
	}
	return path
}

func gopathDevConfigDir(sampleConfigPath string) (string, error) {
	gopath := os.Getenv("GOPATH")

	for _, p := range filepath.SplitList(gopath) {
		devPath := filepath.Join(p, sampleConfigPath)
		if dirExists(devPath) {
			fmt.Printf("========= Using sample config dir from GOPATH: %s", devPath)
			return devPath, nil
		}
	}

	return "", fmt.Errorf("unable to find sampleconfig directory on GOPATH")
}

func gomodDevConfigDir() (string, error) {
	buf := bytes.NewBuffer(nil)
	cmd := exec.Command("go", "env", "GOMOD")
	cmd.Stdout = buf

	if err := cmd.Run(); err != nil {
		fmt.Printf("========= Error running go env GOMOD command: %s", err)
		return "", err
	}

	modFile := strings.TrimSpace(buf.String())
	if modFile == "" {
		fmt.Printf("========= Go mod file not found: %s", buf)
		return "", errors.New("not a module or not in module mode")
	}

	devPath := filepath.Join(filepath.Dir(modFile), "sampleconfig")
	if !dirExists(devPath) {
		fmt.Printf("========= Sample config dir from Go mod not found: %s", devPath)
		return "", fmt.Errorf("%s does not exist", devPath)
	}

	fmt.Printf("========= Using sample config dir from Go mod: %s", devPath)

	return devPath, nil
}

// GetDevMspDir gets the path to the sampleconfig/msp tree that is maintained
// with the source tree.  This should only be used in a test/development
// context.
func GetDevMspDir() string {
	devDir := GetDevConfigDir()
	return filepath.Join(devDir, "msp")
}

func SetDevFabricConfigPath(t *testing.T) (cleanup func()) {
	t.Helper()

	oldFabricCfgPath, resetFabricCfgPath := os.LookupEnv("FABRIC_CFG_PATH")
	devConfigDir := GetDevConfigDir()

	err := os.Setenv("FABRIC_CFG_PATH", devConfigDir)
	require.NoError(t, err, "failed to set FABRIC_CFG_PATH")
	if resetFabricCfgPath {
		return func() {
			err := os.Setenv("FABRIC_CFG_PATH", oldFabricCfgPath)
			require.NoError(t, err)
		}
	}

	return func() {
		err := os.Unsetenv("FABRIC_CFG_PATH")
		require.NoError(t, err)
	}
}
