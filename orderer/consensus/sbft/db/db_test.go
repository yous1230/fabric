/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package db

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/tecbot/gorocksdb"
)

func TestMain(m *testing.M) {
	setupTestConfig()
	os.Exit(m.Run())
}

func TestGetDBPathEmptyPath(t *testing.T) {
	originalSetting := viper.GetString("peer.fileSystemPath")
	viper.Set("peer.fileSystemPath", "")
	defer func() {
		x := recover()
		if x == nil {
			t.Fatal("A panic should have been caused here.")
		}
	}()
	defer viper.Set("peer.fileSystemPath", originalSetting)
	Start()
	GetDBHandle()
}

func TestStartDB_DirDoesNotExist(t *testing.T) {
	deleteTestDBPath()

	defer deleteTestDBPath()
	defer Stop()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Failed to open DB: %s", r)
		}
	}()
	Start()
}

func TestStartDB_NonEmptyDirExists(t *testing.T) {
	deleteTestDBPath()
	createNonEmptyTestDBPath()

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("dbPath is already exists. DB open should throw error")
		}
	}()
	Start()
}

func TestWriteAndRead(t *testing.T) {
	deleteTestDBPath()
	Start()
	defer deleteTestDBPath()
	defer Stop()
}

// This test verifies that when a new column family is added to the DB
// users at an older level of the DB will still be able to open it with new code
func TestDBColumnUpgrade(t *testing.T) {
	deleteTestDBPath()
	Start()
	Stop()

	oldcfs := columnfamilies
	columnfamilies = append([]string{"Testing"}, columnfamilies...)
	defer func() {
		columnfamilies = oldcfs
	}()

	defer deleteTestDBPath()
	defer Stop()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Error re-opening DB with upgraded columnFamilies")
		}
	}()
	Start()
}

// db helper functions
func testIterator(t *testing.T, itr *gorocksdb.Iterator, expectedValues map[string][]byte) {
	itrResults := make(map[string][]byte)
	itr.SeekToFirst()
	for ; itr.Valid(); itr.Next() {
		key := itr.Key()
		value := itr.Value()
		k := makeCopy(key.Data())
		v := makeCopy(value.Data())
		itrResults[string(k)] = v
	}
	if len(itrResults) != len(expectedValues) {
		t.Fatalf("Expected [%d] results from iterator, found [%d]", len(expectedValues), len(itrResults))
	}
	for k, v := range expectedValues {
		if !bytes.Equal(itrResults[k], v) {
			t.Fatalf("Wrong value for key [%s]. Expected [%s], found [%s]", k, itrResults[k], v)
		}
	}
}

func createNonEmptyTestDBPath() {
	dbPath := viper.GetString("peer.fileSystemPath")
	os.MkdirAll(dbPath+"/db/tmpFile", 0775)
}

func deleteTestDBPath() {
	dbPath := viper.GetString("peer.fileSystemPath")
	os.RemoveAll(dbPath)
}

func setupTestConfig() {
	tempDir, err := ioutil.TempDir("", "fabric-db-test")
	if err != nil {
		panic(err)
	}
	viper.Set("peer.fileSystemPath", tempDir)
	deleteTestDBPath()
}
