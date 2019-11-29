/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chaincode_test

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/hyperledger/fabric/internal/peer/lifecycle/chaincode"
	"github.com/hyperledger/fabric/internal/peer/lifecycle/chaincode/mock"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Package", func() {
	Describe("Packager", func() {
		var (
			mockPlatformRegistry *mock.PlatformRegistry
			mockWriter           *mock.Writer
			input                *chaincode.PackageInput
			packager             *chaincode.Packager
		)

		BeforeEach(func() {
			mockPlatformRegistry = &mock.PlatformRegistry{}
			mockPlatformRegistry.NormalizePathReturns("normalizedPath", nil)

			input = &chaincode.PackageInput{
				OutputFile: "testDir/testPackage",
				Path:       "testPath",
				Type:       "testType",
				Label:      "testLabel",
			}

			mockWriter = &mock.Writer{}

			packager = &chaincode.Packager{
				PlatformRegistry: mockPlatformRegistry,
				Writer:           mockWriter,
				Input:            input,
			}
		})

		It("packages chaincodes", func() {
			err := packager.Package()
			Expect(err).NotTo(HaveOccurred())

			Expect(mockPlatformRegistry.NormalizePathCallCount()).To(Equal(1))
			ccType, path := mockPlatformRegistry.NormalizePathArgsForCall(0)
			Expect(ccType).To(Equal("TESTTYPE"))
			Expect(path).To(Equal("testPath"))

			Expect(mockPlatformRegistry.GetDeploymentPayloadCallCount()).To(Equal(1))
			ccType, path = mockPlatformRegistry.GetDeploymentPayloadArgsForCall(0)
			Expect(ccType).To(Equal("TESTTYPE"))
			Expect(path).To(Equal("testPath"))

			Expect(mockWriter.WriteFileCallCount()).To(Equal(1))
			dir, name, pkgTarGzBytes := mockWriter.WriteFileArgsForCall(0)
			wd, err := os.Getwd()
			Expect(err).NotTo(HaveOccurred())
			Expect(dir).To(Equal(filepath.Join(wd, "testDir")))
			Expect(name).To(Equal("testPackage"))
			Expect(pkgTarGzBytes).NotTo(BeNil())

			metadata, err := readMetadataFromBytes(pkgTarGzBytes)
			Expect(err).NotTo(HaveOccurred())
			Expect(metadata).To(Equal(&chaincode.PackageMetadata{
				Path:  "normalizedPath",
				Type:  "testType",
				Label: "testLabel",
			}))
		})

		Context("when the path is not provided", func() {
			BeforeEach(func() {
				input.Path = ""
			})

			It("returns an error", func() {
				err := packager.Package()
				Expect(err).To(MatchError("chaincode path must be specified"))
			})
		})

		Context("when the type is not provided", func() {
			BeforeEach(func() {
				input.Type = ""
			})

			It("returns an error", func() {
				err := packager.Package()
				Expect(err).To(MatchError("chaincode language must be specified"))
			})
		})

		Context("when the output file is not provided", func() {
			BeforeEach(func() {
				input.OutputFile = ""
			})

			It("returns an error", func() {
				err := packager.Package()
				Expect(err).To(MatchError("output file must be specified"))
			})
		})

		Context("when the label is not provided", func() {
			BeforeEach(func() {
				input.Label = ""
			})

			It("returns an error", func() {
				err := packager.Package()
				Expect(err).To(MatchError("package label must be specified"))
			})
		})

		Context("when the platform registry fails to normalize the path", func() {
			BeforeEach(func() {
				mockPlatformRegistry.NormalizePathReturns("", errors.New("cortado"))
			})

			It("returns an error", func() {
				err := packager.Package()
				Expect(err).To(MatchError("failed to normalize chaincode path: cortado"))
			})
		})

		Context("when the platform registry fails to get the deployment payload", func() {
			BeforeEach(func() {
				mockPlatformRegistry.GetDeploymentPayloadReturns(nil, errors.New("americano"))
			})

			It("returns an error", func() {
				err := packager.Package()
				Expect(err).To(MatchError("error getting chaincode bytes: americano"))
			})
		})

		Context("when writing the file fails", func() {
			BeforeEach(func() {
				mockWriter.WriteFileReturns(errors.New("espresso"))
			})

			It("returns an error", func() {
				err := packager.Package()
				Expect(err).To(MatchError("error writing chaincode package to testDir/testPackage: espresso"))
			})
		})
	})

	Describe("PackageCmd", func() {
		var (
			packageCmd *cobra.Command
		)

		BeforeEach(func() {
			packageCmd = chaincode.PackageCmd(nil)
			packageCmd.SetArgs([]string{
				"testPackage",
				"--path=testPath",
				"--lang=golang",
				"--label=testLabel",
			})
		})

		It("sets up the packager and attempts to package the chaincode", func() {
			err := packageCmd.Execute()
			Expect(err).To(MatchError(ContainSubstring("error getting chaincode bytes")))
		})

		Context("when more than one argument is provided", func() {
			BeforeEach(func() {
				packageCmd = chaincode.PackageCmd(nil)
				packageCmd.SetArgs([]string{
					"testPackage",
					"whatthe",
				})
			})

			It("returns an error", func() {
				err := packageCmd.Execute()
				Expect(err).To(MatchError("invalid number of args. expected only the output file"))
			})
		})

		Context("when no argument is provided", func() {
			BeforeEach(func() {
				packageCmd = chaincode.PackageCmd(nil)
				packageCmd.SetArgs([]string{})
			})

			It("returns an error", func() {
				err := packageCmd.Execute()
				Expect(err).To(MatchError("invalid number of args. expected only the output file"))
			})
		})
	})
})

func readMetadataFromBytes(pkgTarGzBytes []byte) (*chaincode.PackageMetadata, error) {
	buffer := gbytes.BufferWithBytes(pkgTarGzBytes)
	gzr, err := gzip.NewReader(buffer)
	Expect(err).NotTo(HaveOccurred())
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Name == "metadata.json" {
			jsonDecoder := json.NewDecoder(tr)
			metadata := &chaincode.PackageMetadata{}
			err := jsonDecoder.Decode(metadata)
			return metadata, err
		}
	}
	return nil, errors.New("metadata.json not found")
}
