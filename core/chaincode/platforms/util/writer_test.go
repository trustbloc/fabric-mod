/*
Copyright London Stock Exchange 2016 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileToPackage(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "utiltest")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	buf := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	// Create a file and write it to tar writer
	filename := "test.txt"
	filecontent := "hello"
	filePath := filepath.Join(tempDir, filename)
	err = ioutil.WriteFile(filePath, bytes.NewBufferString(filecontent).Bytes(), 0600)
	require.NoError(t, err, "Error creating file %s", filePath)

	err = WriteFileToPackage(filePath, filename, tw)
	assert.NoError(t, err, "Error returned by WriteFileToPackage while writing existing file")
	tw.Close()
	gw.Close()

	// Read the file from the archive and check the name and file content
	r := bytes.NewReader(buf.Bytes())
	gr, err := gzip.NewReader(r)
	require.NoError(t, err, "Error creating a gzip reader")
	defer gr.Close()

	tr := tar.NewReader(gr)
	header, err := tr.Next()
	require.NoError(t, err, "Error getting the file from the tar")
	assert.Equal(t, filename, header.Name, "filename read from archive does not match what was added")

	b := make([]byte, 5)
	n, err := tr.Read(b)
	assert.Equal(t, 5, n)
	assert.True(t, err == nil || err == io.EOF, "Error reading file from the archive") // go1.10 returns io.EOF
	assert.Equal(t, filecontent, string(b), "file content from archive does not equal original content")

	t.Run("non existent file", func(t *testing.T) {
		tw := tar.NewWriter(&bytes.Buffer{})
		err := WriteFileToPackage("missing-file", "", tw)
		assert.Error(t, err, "expected error writing a non existent file")
		assert.Contains(t, err.Error(), "missing-file")
	})

	t.Run("closed tar writer", func(t *testing.T) {
		tw := tar.NewWriter(&bytes.Buffer{})
		tw.Close()
		err := WriteFileToPackage(filePath, "test.txt", tw)
		assert.EqualError(t, err, fmt.Sprintf("failed to write header for %s: archive/tar: write after close", filePath))
	})

	t.Run("stream write failure", func(t *testing.T) {
		failWriter := &failingWriter{failAt: 514}
		tw := tar.NewWriter(failWriter)
		err := WriteFileToPackage(filePath, "test.txt", tw)
		assert.EqualError(t, err, fmt.Sprintf("failed to write %s as test.txt: failed-the-write", filePath))
	})
}

type failingWriter struct {
	written int
	failAt  int
}

func (f *failingWriter) Write(b []byte) (int, error) {
	f.written += len(b)
	if f.written < f.failAt {
		return len(b), nil
	}
	return 0, errors.New("failed-the-write")
}

// Success case 1: with include file types and without exclude dir
func TestWriteFolderToTarPackage1(t *testing.T) {
	srcPath := filepath.Join("testdata", "sourcefiles")
	filePath := "src/src/Hello.java"
	includeFileTypes := map[string]bool{
		".java": true,
	}
	excludeFileTypes := map[string]bool{}

	tarBytes := createTestTar(t, srcPath, []string{}, includeFileTypes, excludeFileTypes)

	// Read the file from the archive and check the name
	entries := tarContents(t, tarBytes)
	assert.ElementsMatch(t, []string{filePath}, entries, "archive should only contain one file")
}

// Success case 2: with exclude dir and no include file types
func TestWriteFolderToTarPackage2(t *testing.T) {
	srcPath := filepath.Join("testdata", "sourcefiles")
	tarBytes := createTestTar(t, srcPath, []string{"src"}, nil, nil)

	entries := tarContents(t, tarBytes)
	assert.ElementsMatch(t, []string{"src/artifact.xml", "META-INF/statedb/couchdb/indexes/indexOwner.json"}, entries)
}

// Success case 3: with chaincode metadata in META-INF directory
func TestWriteFolderToTarPackage3(t *testing.T) {
	srcPath := filepath.Join("testdata", "sourcefiles")
	filePath := "META-INF/statedb/couchdb/indexes/indexOwner.json"

	tarBytes := createTestTar(t, srcPath, []string{}, nil, nil)

	// Read the files from the archive and check for the metadata index file
	entries := tarContents(t, tarBytes)
	assert.Contains(t, entries, filePath, "should have found statedb index artifact in META-INF directory")
}

// Success case 4: with chaincode metadata in META-INF directory, pass trailing slash in srcPath
func TestWriteFolderToTarPackage4(t *testing.T) {
	srcPath := filepath.Join("testdata", "sourcefiles") + string(filepath.Separator)
	filePath := "META-INF/statedb/couchdb/indexes/indexOwner.json"

	tarBytes := createTestTar(t, srcPath, []string{}, nil, nil)

	// Read the files from the archive and check for the metadata index file
	entries := tarContents(t, tarBytes)
	assert.Contains(t, entries, filePath, "should have found statedb index artifact in META-INF directory")
}

// Success case 5: with hidden files in META-INF directory (hidden files get ignored)
func TestWriteFolderToTarPackage5(t *testing.T) {
	srcPath := filepath.Join("testdata", "sourcefiles")
	filePath := "META-INF/.hiddenfile"

	assert.FileExists(t, filepath.Join(srcPath, "META-INF", ".hiddenfile"))

	tarBytes := createTestTar(t, srcPath, []string{}, nil, nil)

	// Read the files from the archive and check for the metadata index file
	entries := tarContents(t, tarBytes)
	assert.NotContains(t, entries, filePath, "should not contain .hiddenfile in META-INF directory")
}

// Failure case 1: no files in directory
func TestWriteFolderToTarPackageFailure1(t *testing.T) {
	srcPath, err := ioutil.TempDir("", "utiltest")
	require.NoError(t, err)
	defer os.RemoveAll(srcPath)

	tw := tar.NewWriter(bytes.NewBuffer(nil))
	defer tw.Close()

	err = WriteFolderToTarPackage(tw, srcPath, []string{}, nil, nil)
	assert.Contains(t, err.Error(), "no source files found")
}

// Failure case 2: with invalid chaincode metadata in META-INF directory
func TestWriteFolderToTarPackageFailure2(t *testing.T) {
	srcPath := filepath.Join("testdata", "BadMetadataInvalidIndex")
	buf := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	err := WriteFolderToTarPackage(tw, srcPath, []string{}, nil, nil)
	assert.Error(t, err, "Should have received error writing folder to package")
	assert.Contains(t, err.Error(), "Index metadata file [META-INF/statedb/couchdb/indexes/bad.json] is not a valid JSON")

	tw.Close()
	gw.Close()
}

// Failure case 3: with unexpected content in META-INF directory
func TestWriteFolderToTarPackageFailure3(t *testing.T) {
	srcPath := filepath.Join("testdata", "BadMetadataUnexpectedFolderContent")
	buf := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	err := WriteFolderToTarPackage(tw, srcPath, []string{}, nil, nil)
	assert.Error(t, err, "Should have received error writing folder to package")
	assert.Contains(t, err.Error(), "metadata file path must begin with META-INF/statedb")

	tw.Close()
	gw.Close()
}

// Failure case 4: with lstat failed
func Test_WriteFolderToTarPackageFailure4(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "WriteFolderToTarPackageFailure4BadFileMode")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	testFile := filepath.Join(tempDir, "test.java")
	err = ioutil.WriteFile(testFile, []byte("Content"), 0644)
	require.NoError(t, err, "Error creating file", testFile)
	err = os.Chmod(tempDir, 0644)
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	err = WriteFolderToTarPackage(tw, tempDir, []string{}, nil, nil)
	assert.Error(t, err, "Should have received error writing folder to package")
	assert.Contains(t, err.Error(), "permission denied")
}

func createTestTar(t *testing.T, srcPath string, excludeDir []string, includeFileTypeMap map[string]bool, excludeFileTypeMap map[string]bool) []byte {
	buf := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	err := WriteFolderToTarPackage(tw, srcPath, excludeDir, includeFileTypeMap, excludeFileTypeMap)
	assert.NoError(t, err, "Error writing folder to package")

	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func tarContents(t *testing.T, buf []byte) []string {
	br := bytes.NewReader(buf)
	gr, err := gzip.NewReader(br)
	require.NoError(t, err)
	defer gr.Close()

	tr := tar.NewReader(gr)

	var entries []string
	for {
		header, err := tr.Next()
		if err == io.EOF { // No more entries
			break
		}
		require.NoError(t, err, "failed to get next entry")
		entries = append(entries, header.Name)
	}

	return entries
}
