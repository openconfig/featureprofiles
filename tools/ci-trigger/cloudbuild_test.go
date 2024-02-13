// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"
)

func TestCreateTGZArchive(t *testing.T) {
	f := fstest.MapFS{
		"file1.txt":      {Data: []byte("Hello World"), Mode: 0644},
		"file2.txt":      {Mode: 0644},
		"dir1":           {Mode: fs.ModeDir | 0755},
		"dir1/file3.txt": {Data: []byte("Inside dir1"), Mode: 0644},
		"dir2":           {Mode: fs.ModeDir | 0755},
		"dir2/file4.txt": {Data: []byte("Inside dir2"), Mode: 0644},
		"dir2/file5.txt": {Data: []byte("Executable Inside dir2"), Mode: 0755},
	}

	buf, err := createTGZArchive(f)
	if err != nil {
		t.Fatal(err)
	}

	gzReader, err := gzip.NewReader(buf)
	if err != nil {
		t.Fatal(err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}

		_, ok := f[header.Name]
		if !ok {
			t.Errorf("createTGZArchive(): unexpected entry found in tarball: %s", header.Name)
		}

		origEntry := f[header.Name]
		expectedMode := origEntry.Mode & os.ModePerm
		actualMode := header.FileInfo().Mode() & os.ModePerm
		if expectedMode != actualMode {
			t.Errorf("createTGZArchive(): unexpected mode for %s: got %v, want %v", header.Name, actualMode, expectedMode)
		}

		if header.Typeflag == tar.TypeReg {
			origFile, err := f.Open(header.Name)
			if err != nil {
				t.Fatal(err)
			}
			defer origFile.Close()

			origData, err := io.ReadAll(origFile)
			if err != nil {
				t.Fatal(err)
			}

			tarData, err := io.ReadAll(tarReader)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(origData, tarData) {
				t.Errorf("createTGZArchive(): original and compressed contents do not match for %s", header.Name)
			}
		}
	}
}
