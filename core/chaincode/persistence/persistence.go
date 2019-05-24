/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package persistence

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/hyperledger/fabric/common/chaincode"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/common/util"
	persistence "github.com/hyperledger/fabric/core/chaincode/persistence/intf"

	"github.com/pkg/errors"
)

var logger = flogging.MustGetLogger("chaincode.persistence")

// IOReadWriter defines the interface needed for reading, writing, removing, and
// checking for existence of a specified file
type IOReadWriter interface {
	ReadDir(string) ([]os.FileInfo, error)
	ReadFile(string) ([]byte, error)
	Remove(name string) error
	WriteFile(string, string, []byte) error
	MakeDir(string, os.FileMode) error
	Exists(path string) (bool, error)
}

// FilesystemIO is the production implementation of the IOWriter interface
type FilesystemIO struct {
}

// WriteFile writes a file to the filesystem; it does so atomically
// by first writing to a temp file and then renaming the file so that
// if the operation crashes midway we're not stuck with a bad package
func (f *FilesystemIO) WriteFile(path, name string, data []byte) error {
	if path == "" {
		return errors.New("empty path not allowed")
	}
	tmpFile, err := ioutil.TempFile(path, ".ccpackage.")
	if err != nil {
		return errors.Wrapf(err, "error creating temp file in directory '%s'", path)
	}
	defer os.Remove(tmpFile.Name())

	if n, err := tmpFile.Write(data); err != nil || n != len(data) {
		if err == nil {
			err = errors.Errorf(
				"failed to write the entire content of the file, expected %d, wrote %d",
				len(data), n)
		}
		return errors.Wrapf(err, "error writing to temp file '%s'", tmpFile.Name())
	}

	if err := tmpFile.Close(); err != nil {
		return errors.Wrapf(err, "error closing temp file '%s'", tmpFile.Name())
	}

	if err := os.Rename(tmpFile.Name(), filepath.Join(path, name)); err != nil {
		return errors.Wrapf(err, "error renaming temp file '%s'", tmpFile.Name())
	}

	return nil
}

// Remove removes a file from the filesystem - used for rolling back an in-flight
// Save operation upon a failure
func (f *FilesystemIO) Remove(name string) error {
	return os.Remove(name)
}

// ReadFile reads a file from the filesystem
func (f *FilesystemIO) ReadFile(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}

// ReadDir reads a directory from the filesystem
func (f *FilesystemIO) ReadDir(dirname string) ([]os.FileInfo, error) {
	return ioutil.ReadDir(dirname)
}

// MakeDir makes a directory on the filesystem (and any
// necessary parent directories).
func (f *FilesystemIO) MakeDir(dirname string, mode os.FileMode) error {
	return os.MkdirAll(dirname, mode)
}

// Exists checks whether a file exists
func (*FilesystemIO) Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}

	return false, errors.Wrapf(err, "could not determine whether file '%s' exists", path)
}

// Store holds the information needed for persisting a chaincode install package
type Store struct {
	Path       string
	ReadWriter IOReadWriter
}

// NewStore creates a new chaincode persistence store using
// the provided path on the filesystem.
func NewStore(path string) *Store {
	store := &Store{
		Path:       path,
		ReadWriter: &FilesystemIO{},
	}
	store.Initialize()
	return store
}

// Initialize checks for the existence of the _lifecycle chaincodes
// directory and creates it if it has not yet been created.
func (s *Store) Initialize() {
	var (
		exists bool
		err    error
	)
	if exists, err = s.ReadWriter.Exists(s.Path); exists {
		return
	}
	if err != nil {
		panic(fmt.Sprintf("Initialization of chaincode store failed: %s", err))
	}
	if err = s.ReadWriter.MakeDir(s.Path, 0750); err != nil {
		panic(fmt.Sprintf("Could not create _lifecycle chaincodes install path: %s", err))
	}
}

// Save persists chaincode install package bytes. It returns
// the hash of the chaincode install package
func (s *Store) Save(label string, ccInstallPkg []byte) (persistence.PackageID, error) {
	hash := util.ComputeSHA256(ccInstallPkg)
	packageID := packageID(label, hash)

	ccInstallPkgFileName := packageID.String() + ".bin"
	ccInstallPkgFilePath := filepath.Join(s.Path, ccInstallPkgFileName)

	if exists, _ := s.ReadWriter.Exists(ccInstallPkgFilePath); exists {
		// chaincode install package was already installed
		return packageID, nil
	}

	if err := s.ReadWriter.WriteFile(s.Path, ccInstallPkgFileName, ccInstallPkg); err != nil {
		err = errors.Wrapf(err, "error writing chaincode install package to %s", ccInstallPkgFilePath)
		logger.Error(err.Error())
		return "", err
	}

	return packageID, nil
}

// Load loads a persisted chaincode install package bytes with the given hash
// and also returns the chaincode metadata (names and versions) of any chaincode
// installed with a matching hash
func (s *Store) Load(packageID persistence.PackageID) ([]byte, error) {
	ccInstallPkgPath := filepath.Join(s.Path, packageID.String()+".bin")

	exists, err := s.ReadWriter.Exists(ccInstallPkgPath)
	if err != nil {
		return nil, errors.Wrapf(err, "could not determine whether chaincode install package '%s' exists", packageID)
	}
	if !exists {
		return nil, &CodePackageNotFoundErr{
			PackageID: packageID,
		}
	}

	ccInstallPkg, err := s.ReadWriter.ReadFile(ccInstallPkgPath)
	if err != nil {
		err = errors.Wrapf(err, "error reading chaincode install package at %s", ccInstallPkgPath)
		return nil, err
	}

	return ccInstallPkg, nil
}

// CodePackageNotFoundErr is the error returned when a code package cannot
// be found in the persistence store
type CodePackageNotFoundErr struct {
	PackageID persistence.PackageID
}

func (e CodePackageNotFoundErr) Error() string {
	return fmt.Sprintf("chaincode install package '%s' not found", e.PackageID)
}

// ListInstalledChaincodes returns an array with information about the
// chaincodes installed in the persistence store
func (s *Store) ListInstalledChaincodes() ([]chaincode.InstalledChaincode, error) {
	files, err := s.ReadWriter.ReadDir(s.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading chaincode directory at %s", s.Path)
	}

	installedChaincodes := []chaincode.InstalledChaincode{}
	for _, file := range files {
		if instCC, isInstCC := installedChaincodeFromFilename(file.Name()); isInstCC {
			installedChaincodes = append(installedChaincodes, instCC)
		}
	}
	return installedChaincodes, nil
}

// GetChaincodeInstallPath returns the path where chaincodes
// are installed
func (s *Store) GetChaincodeInstallPath() string {
	return s.Path
}

func packageID(label string, hash []byte) persistence.PackageID {
	return persistence.PackageID(fmt.Sprintf("%s:%x", label, hash))
}

var packageFileMatcher = regexp.MustCompile("^(.+):([0-9abcdef]+)[.]bin$")

func installedChaincodeFromFilename(fileName string) (chaincode.InstalledChaincode, bool) {
	matches := packageFileMatcher.FindStringSubmatch(fileName)
	if len(matches) == 3 {
		label := matches[1]
		hash, _ := hex.DecodeString(matches[2])
		packageID := packageID(label, hash)

		return chaincode.InstalledChaincode{
			Label:     label,
			Hash:      hash,
			PackageID: packageID,
		}, true
	}

	return chaincode.InstalledChaincode{}, false
}
