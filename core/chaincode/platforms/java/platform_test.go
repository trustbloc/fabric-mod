/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package java_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	pb "github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/core/chaincode/platforms/java"
	"github.com/hyperledger/fabric/core/chaincode/platforms/util"
	"github.com/hyperledger/fabric/core/config/configtest"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const chaincodePathFolderGradle = "testdata/gradle"

var spec = &pb.ChaincodeSpec{
	Type: pb.ChaincodeSpec_JAVA,
	ChaincodeId: &pb.ChaincodeID{
		Name: "ssample",
		Path: chaincodePathFolderGradle,
	},
	Input: &pb.ChaincodeInput{
		Args: [][]byte{
			[]byte("f"),
		},
	},
}

func TestMain(m *testing.M) {
	viper.SetConfigName("core")
	viper.SetEnvPrefix("CORE")
	configtest.AddDevConfigPath(nil)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("could not read config %s\n", err)
		os.Exit(-1)
	}
	os.Exit(m.Run())
}

func TestValidatePath(t *testing.T) {
	platform := java.Platform{}

	err := platform.ValidatePath(spec.ChaincodeId.Path)
	assert.NoError(t, err)
}

func TestValidateCodePackage(t *testing.T) {
	platform := java.Platform{}
	b, _ := generateMockPackegeBytes("src/pom.xml", 0100400)
	assert.NoError(t, platform.ValidateCodePackage(b))

	b, _ = generateMockPackegeBytes("src/pom.xml", 0100555)
	assert.Error(t, platform.ValidateCodePackage(b))

	b, _ = generateMockPackegeBytes("src/build.gradle", 0100400)
	assert.NoError(t, platform.ValidateCodePackage(b))

	b, _ = generateMockPackegeBytes("src/build.xml", 0100400)
	assert.Error(t, platform.ValidateCodePackage(b))

	b, _ = generateMockPackegeBytes("src/src/Main.java", 0100400)
	assert.NoError(t, platform.ValidateCodePackage(b))

	b, _ = generateMockPackegeBytes("src/build/Main.java", 0100400)
	assert.Error(t, platform.ValidateCodePackage(b))

	b, _ = generateMockPackegeBytes("src/src/xyz/main.java", 0100400)
	assert.NoError(t, platform.ValidateCodePackage(b))

	b, _ = generateMockPackegeBytes("src/src/xyz/main.class", 0100400)
	assert.Error(t, platform.ValidateCodePackage(b))

	b, _ = platform.GetDeploymentPayload(chaincodePathFolderGradle)
	assert.NoError(t, platform.ValidateCodePackage(b))
}

func TestGetDeploymentPayload(t *testing.T) {
	platform := java.Platform{}

	_, err := platform.GetDeploymentPayload("")
	assert.Contains(t, err.Error(), "ChaincodeSpec's path cannot be empty")

	spec.ChaincodeId.Path = chaincodePathFolderGradle

	payload, err := platform.GetDeploymentPayload(chaincodePathFolderGradle)
	assert.NoError(t, err)
	assert.NotZero(t, len(payload))

	is := bytes.NewReader(payload)
	gr, err := gzip.NewReader(is)
	require.NoError(t, err, "failed to open zip stream")
	defer gr.Close()

	tr := tar.NewReader(gr)

	contents := map[string]bool{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)

		if strings.Contains(header.Name, ".class") {
			assert.Fail(t, "Result package can't contain class file")
		}
		if strings.Contains(header.Name, "target/") {
			assert.Fail(t, "Result package can't contain target folder")
		}
		if strings.Contains(header.Name, "build/") {
			assert.Fail(t, "Result package can't contain build folder")
		}
		contents[header.Name] = true
	}

	// generated from observed behavior
	assert.Contains(t, contents, "src/build.gradle")
	assert.Contains(t, contents, "src/pom.xml")
	assert.Contains(t, contents, "src/settings.gradle")
	assert.Contains(t, contents, "src/src/main/java/example/ExampleCC.java")
}

func TestGenerateDockerfile(t *testing.T) {
	platform := java.Platform{}

	spec.ChaincodeId.Path = chaincodePathFolderGradle
	_, err := platform.GetDeploymentPayload(spec.ChaincodeId.Path)
	if err != nil {
		t.Fatalf("failed to get Java CC payload: %s", err)
	}

	dockerfile, err := platform.GenerateDockerfile()
	assert.NoError(t, err)

	var buf []string

	buf = append(buf, "FROM "+util.GetDockerImageFromConfig("chaincode.java.runtime"))
	buf = append(buf, "ADD binpackage.tar /root/chaincode-java/chaincode")

	dockerFileContents := strings.Join(buf, "\n")

	assert.Equal(t, dockerFileContents, dockerfile)
}

func TestDockerBuildOptions(t *testing.T) {
	platform := java.Platform{}

	opts, err := platform.DockerBuildOptions("path")
	assert.NoError(t, err, "unexpected error from DockerBuildOptions")

	expectedOpts := util.DockerBuildOptions{
		Image: "hyperledger/fabric-javaenv:latest",
		Cmd:   "./build.sh",
	}
	assert.Equal(t, expectedOpts, opts)
}

func generateMockPackegeBytes(fileName string, mode int64) ([]byte, error) {
	var zeroTime time.Time
	codePackage := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(codePackage)
	tw := tar.NewWriter(gw)
	payload := make([]byte, 25, 25)
	err := tw.WriteHeader(&tar.Header{Name: fileName, Size: int64(len(payload)), ModTime: zeroTime, AccessTime: zeroTime, ChangeTime: zeroTime, Mode: mode})
	if err != nil {
		return nil, err
	}
	_, err = tw.Write(payload)
	if err != nil {
		return nil, err
	}
	tw.Close()
	gw.Close()
	return codePackage.Bytes(), nil
}
