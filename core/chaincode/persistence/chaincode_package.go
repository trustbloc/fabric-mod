/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package persistence

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	pb "github.com/hyperledger/fabric-protos-go/peer"

	"github.com/pkg/errors"
)

// The chaincode package is simply a .tar.gz file.  For the time being, we
// assume that the package contains a Chaincode-Package-Metadata.json file
// which contains a 'Type', and optionally a 'Path'.  In the future, it would
// be nice if we moved to a more buildpack type system, rather than the below
// presented JAR+manifest type system, but for expediency and incremental changes,
// moving to a tar format over the proto format for a user-inspectable artifact
// seems like a good step.

const (
	// MetadataFile is the expected location of the metadata json document
	// in the top level of the chaincode package.
	MetadataFile = "metadata.json"

	// CodePackageFile is the expected location of the code package in the
	// top level of the chaincode package
	CodePackageFile = "code.tar.gz"
)

//go:generate counterfeiter -o mock/legacy_cc_package_locator.go --fake-name LegacyCCPackageLocator . LegacyCCPackageLocator

type LegacyCCPackageLocator interface {
	GetChaincodeDepSpec(nameVersion string) (*pb.ChaincodeDeploymentSpec, error)
}

type FallbackPackageLocator struct {
	ChaincodePackageLocator *ChaincodePackageLocator
	LegacyCCPackageLocator  LegacyCCPackageLocator
}

func (fpl *FallbackPackageLocator) GetChaincodePackage(packageID string) (*ChaincodePackageMetadata, io.ReadCloser, error) {
	streamer := fpl.ChaincodePackageLocator.ChaincodePackageStreamer(packageID)
	if streamer.Exists() {
		metadata, err := streamer.Metadata()
		if err != nil {
			return nil, nil, errors.WithMessagef(err, "error retrieving chaincode package metadata '%s'", packageID)
		}

		tarStream, err := streamer.Code()
		if err != nil {
			return nil, nil, errors.WithMessagef(err, "error retrieving chaincode package code '%s'", packageID)
		}

		return metadata, tarStream, nil
	}

	cds, err := fpl.LegacyCCPackageLocator.GetChaincodeDepSpec(string(packageID))
	if err != nil {
		return nil, nil, errors.WithMessagef(err, "could not get legacy chaincode package '%s'", packageID)
	}

	return &ChaincodePackageMetadata{
			Path: cds.ChaincodeSpec.ChaincodeId.Path,
			Type: cds.ChaincodeSpec.Type.String(),
		},
		ioutil.NopCloser(bytes.NewBuffer(cds.CodePackage)),
		nil
}

type ChaincodePackageLocator struct {
	ChaincodeDir string
}

func (cpl *ChaincodePackageLocator) ChaincodePackageStreamer(packageID string) *ChaincodePackageStreamer {
	return &ChaincodePackageStreamer{
		PackagePath: filepath.Join(cpl.ChaincodeDir, CCFileName(packageID)),
	}
}

type ChaincodePackageStreamer struct {
	PackagePath string
}

func (cps *ChaincodePackageStreamer) Exists() bool {
	_, err := os.Stat(cps.PackagePath)
	return err == nil
}

func (cps *ChaincodePackageStreamer) Metadata() (*ChaincodePackageMetadata, error) {
	tarFileStream, err := cps.File(MetadataFile)
	if err != nil {
		return nil, errors.WithMessage(err, "could not get metadata file")
	}

	defer tarFileStream.Close()

	metadata := &ChaincodePackageMetadata{}
	err = json.NewDecoder(tarFileStream).Decode(metadata)
	if err != nil {
		return nil, errors.WithMessage(err, "could not parse metadata file")
	}

	return metadata, nil
}

func (cps *ChaincodePackageStreamer) Code() (*TarFileStream, error) {
	tarFileStream, err := cps.File(CodePackageFile)
	if err != nil {
		return nil, errors.WithMessage(err, "could not get code package")
	}

	return tarFileStream, nil
}

func (cps *ChaincodePackageStreamer) File(name string) (tarFileStream *TarFileStream, err error) {
	file, err := os.Open(cps.PackagePath)
	if err != nil {
		return nil, errors.WithMessagef(err, "could not open chaincode package at '%s'", cps.PackagePath)
	}

	defer func() {
		if err != nil {
			file.Close()
		}
	}()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading as gzip stream")
	}

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, errors.Wrapf(err, "error inspecting next tar header")
		}

		if header.Name != name {
			continue
		}

		if header.Typeflag != tar.TypeReg {
			return nil, errors.Errorf("tar entry %s is not a regular file, type %v", header.Name, header.Typeflag)
		}

		return &TarFileStream{
			TarFile:    tarReader,
			FileStream: file,
		}, nil
	}

	return nil, errors.Errorf("did not find file '%s' in package", name)
}

type TarFileStream struct {
	TarFile    io.Reader
	FileStream io.Closer
}

func (tfs *TarFileStream) Read(p []byte) (int, error) {
	return tfs.TarFile.Read(p)
}

func (tfs *TarFileStream) Close() error {
	return tfs.FileStream.Close()
}

// ChaincodePackage represents the un-tar-ed format of the chaincode package.
type ChaincodePackage struct {
	Metadata    *ChaincodePackageMetadata
	CodePackage []byte
	DBArtifacts []byte
}

// ChaincodePackageMetadata contains the information necessary to understand
// the embedded code package.
type ChaincodePackageMetadata struct {
	Type  string `json:"type"`
	Path  string `json:"path"`
	Label string `json:"label"`
}

// MetadataProvider provides the means to retrieve metadata
// information (for instance the DB indexes) from a code package.
type MetadataProvider interface {
	GetDBArtifacts(codePackage []byte) ([]byte, error)
}

// ChaincodePackageParser provides the ability to parse chaincode packages.
type ChaincodePackageParser struct {
	MetadataProvider MetadataProvider
}

var (
	// LabelRegexp is the regular expression controlling
	// the allowed characters for the package label
	LabelRegexp = regexp.MustCompile("^[a-zA-Z0-9]+([.+-_][a-zA-Z0-9]+)*$")
)

func validateLabel(label string) error {
	if !LabelRegexp.MatchString(label) {
		return errors.Errorf("invalid label '%s'. Label must be non-empty, can only consist of alphanumerics, symbols from '.+-_', and can only begin with alphanumerics", label)
	}

	return nil
}

// Parse parses a set of bytes as a chaincode package
// and returns the parsed package as a struct
func (ccpp ChaincodePackageParser) Parse(source []byte) (*ChaincodePackage, error) {
	gzReader, err := gzip.NewReader(bytes.NewBuffer(source))
	if err != nil {
		return nil, errors.Wrapf(err, "error reading as gzip stream")
	}

	tarReader := tar.NewReader(gzReader)

	var codePackage []byte
	var ccPackageMetadata *ChaincodePackageMetadata
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, errors.Wrapf(err, "error inspecting next tar header")
		}

		if header.Typeflag != tar.TypeReg {
			return nil, errors.Errorf("tar entry %s is not a regular file, type %v", header.Name, header.Typeflag)
		}

		fileBytes, err := ioutil.ReadAll(tarReader)
		if err != nil {
			return nil, errors.Wrapf(err, "could not read %s from tar", header.Name)
		}

		switch header.Name {

		case MetadataFile:
			ccPackageMetadata = &ChaincodePackageMetadata{}
			err := json.Unmarshal(fileBytes, ccPackageMetadata)
			if err != nil {
				return nil, errors.Wrapf(err, "could not unmarshal %s as json", MetadataFile)
			}

		case CodePackageFile:
			codePackage = fileBytes
		default:
			logger.Warningf("Encountered unexpected file '%s' in top level of chaincode package", header.Name)
		}
	}

	if codePackage == nil {
		return nil, errors.Errorf("did not find a code package inside the package")
	}

	if ccPackageMetadata == nil {
		return nil, errors.Errorf("did not find any package metadata (missing %s)", MetadataFile)
	}

	if err := validateLabel(ccPackageMetadata.Label); err != nil {
		return nil, err
	}

	dbArtifacts, err := ccpp.MetadataProvider.GetDBArtifacts(codePackage)
	if err != nil {
		return nil, errors.WithMessage(err, "error retrieving DB artifacts from code package")
	}

	return &ChaincodePackage{
		Metadata:    ccPackageMetadata,
		CodePackage: codePackage,
		DBArtifacts: dbArtifacts,
	}, nil
}
