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
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"github.com/tecbot/gorocksdb"
)

var dbLogger = logging.MustGetLogger("db")
var rocksDBLogLevelMap = map[string]gorocksdb.InfoLogLevel{
	"debug": gorocksdb.DebugInfoLogLevel,
	"info":  gorocksdb.InfoInfoLogLevel,
	"warn":  gorocksdb.WarnInfoLogLevel,
	"error": gorocksdb.ErrorInfoLogLevel,
	"fatal": gorocksdb.FatalInfoLogLevel}

const persistCF = "persistCF"

var columnfamilies = []string{
	persistCF, // persistent per-peer state (consensus)
}

// RocksDB encapsulates rocksdb's structures
type RocksDB struct {
	DB        *gorocksdb.DB
	PersistCF *gorocksdb.ColumnFamilyHandle

}

var openchainDB = create()

// Create create an openchainDB instance
func create() *RocksDB {
	return &RocksDB{}
}

// GetDBHandle gets an opened openchainDB singleton. Note that method Start must always be invoked before this method.
func GetDBHandle() *RocksDB {
	return openchainDB
}

// Start the db, init the openchainDB instance and open the db. Note this method has no guarantee correct behavior concurrent invocation.
func Start() {
	openchainDB.open()
}

// Stop the db. Note this method has no guarantee correct behavior concurrent invocation.
func Stop() {
	openchainDB.close()
}

// GetSnapshot returns a point-in-time view of the DB. You MUST call snapshot.Release()
// when you are done with the snapshot.
func (openchainDB *RocksDB) GetSnapshot() *gorocksdb.Snapshot {
	return openchainDB.DB.NewSnapshot()
}

func getDBPath() string {
	dbPath := viper.GetString("peer.fileSystemPath")
	if dbPath == "" {
		panic("DB path not specified in configuration file. Please check that property 'peer.fileSystemPath' is set")
	}
	if !strings.HasSuffix(dbPath, "/") {
		dbPath = dbPath + "/"
	}
	return dbPath + "db"
}

// Open open underlying rocksdb
func (openchainDB *RocksDB) open() {
	dbPath := getDBPath()
	missing, err := dirMissingOrEmpty(dbPath)
	if err != nil {
		panic(fmt.Sprintf("Error while trying to open DB: %s", err))
	}
	dbLogger.Debugf("Is db path [%s] empty [%t]", dbPath, missing)

	if missing {
		err = os.MkdirAll(path.Dir(dbPath), 0755)
		if err != nil {
			panic(fmt.Sprintf("Error making directory path [%s]: %s", dbPath, err))
		}
	}

	opts := gorocksdb.NewDefaultOptions()
	defer opts.Destroy()

	maxLogFileSize := viper.GetInt("peer.db.maxLogFileSize")
	if maxLogFileSize > 0 {
		dbLogger.Infof("Setting rocksdb maxLogFileSize to %d", maxLogFileSize)
		opts.SetMaxLogFileSize(maxLogFileSize)
	}

	keepLogFileNum := viper.GetInt("peer.db.keepLogFileNum")
	if keepLogFileNum > 0 {
		dbLogger.Infof("Setting rocksdb keepLogFileNum to %d", keepLogFileNum)
		opts.SetKeepLogFileNum(keepLogFileNum)
	}

	logLevelStr := viper.GetString("peer.db.loglevel")
	logLevel, ok := rocksDBLogLevelMap[logLevelStr]

	if ok {
		dbLogger.Infof("Setting rocks db InfoLogLevel to %d", logLevel)
		opts.SetInfoLogLevel(logLevel)
	}

	opts.SetCreateIfMissing(missing)
	opts.SetCreateIfMissingColumnFamilies(true)

	cfNames := []string{"default"}
	cfNames = append(cfNames, columnfamilies...)
	var cfOpts []*gorocksdb.Options
	for range cfNames {
		cfOpts = append(cfOpts, opts)
	}

	db, cfHandlers, err := gorocksdb.OpenDbColumnFamilies(opts, dbPath, cfNames, cfOpts)

	if err != nil {
		panic(fmt.Sprintf("Error opening DB: %s", err))
	}

	openchainDB.DB = db
	openchainDB.PersistCF = cfHandlers[1]
}

// Close releases all column family handles and closes rocksdb
func (openchainDB *RocksDB) close() {
	openchainDB.PersistCF.Destroy()
	openchainDB.DB.Close()
}

// Get returns the valud for the given column family and key
func (openchainDB *RocksDB) Get(cfHandler *gorocksdb.ColumnFamilyHandle, key []byte) ([]byte, error) {
	opt := gorocksdb.NewDefaultReadOptions()
	defer opt.Destroy()
	slice, err := openchainDB.DB.GetCF(opt, cfHandler, key)
	if err != nil {
		dbLogger.Errorf("Error while trying to retrieve key: %s", key)
		return nil, err
	}
	defer slice.Free()
	if slice.Data() == nil {
		return nil, nil
	}
	data := makeCopy(slice.Data())
	return data, nil
}

// Put saves the key/value in the given column family
func (openchainDB *RocksDB) Put(cfHandler *gorocksdb.ColumnFamilyHandle, key []byte, value []byte) error {
	opt := gorocksdb.NewDefaultWriteOptions()
	defer opt.Destroy()
	err := openchainDB.DB.PutCF(opt, cfHandler, key, value)
	if err != nil {
		dbLogger.Errorf("Error while trying to write key: %s", key)
		return err
	}
	return nil
}

// Delete delets the given key in the specified column family
func (openchainDB *RocksDB) Delete(cfHandler *gorocksdb.ColumnFamilyHandle, key []byte) error {
	opt := gorocksdb.NewDefaultWriteOptions()
	defer opt.Destroy()
	err := openchainDB.DB.DeleteCF(opt, cfHandler, key)
	if err != nil {
		dbLogger.Errorf("Error while trying to delete key: %s", key)
		return err
	}
	return nil
}

func (openchainDB *RocksDB) getFromSnapshot(snapshot *gorocksdb.Snapshot, cfHandler *gorocksdb.ColumnFamilyHandle, key []byte) ([]byte, error) {
	opt := gorocksdb.NewDefaultReadOptions()
	defer opt.Destroy()
	opt.SetSnapshot(snapshot)
	slice, err := openchainDB.DB.GetCF(opt, cfHandler, key)
	if err != nil {
		dbLogger.Errorf("Error while trying to retrieve key: %s", key)
		return nil, err
	}
	defer slice.Free()
	data := append([]byte(nil), slice.Data()...)
	return data, nil
}

// GetIterator returns an iterator for the given column family
func (openchainDB *RocksDB) GetIterator(cfHandler *gorocksdb.ColumnFamilyHandle) *gorocksdb.Iterator {
	opt := gorocksdb.NewDefaultReadOptions()
	opt.SetFillCache(true)
	defer opt.Destroy()
	return openchainDB.DB.NewIteratorCF(opt, cfHandler)
}

func (openchainDB *RocksDB) getSnapshotIterator(snapshot *gorocksdb.Snapshot, cfHandler *gorocksdb.ColumnFamilyHandle) *gorocksdb.Iterator {
	opt := gorocksdb.NewDefaultReadOptions()
	defer opt.Destroy()
	opt.SetSnapshot(snapshot)
	iter := openchainDB.DB.NewIteratorCF(opt, cfHandler)
	return iter
}

func dirMissingOrEmpty(path string) (bool, error) {
	dirExists, err := dirExists(path)
	if err != nil {
		return false, err
	}
	if !dirExists {
		return true, nil
	}

	dirEmpty, err := dirEmpty(path)
	if err != nil {
		return false, err
	}
	if dirEmpty {
		return true, nil
	}
	return false, nil
}

func dirExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func dirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func makeCopy(src []byte) []byte {
	dest := make([]byte, len(src))
	copy(dest, src)
	return dest
}
