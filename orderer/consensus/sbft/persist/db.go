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

package persist

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/op/go-logging"
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
	LogDir         string
	LogLevel       string
	MaxLogFileSize uint64
	KeepLogFileNum uint32

	DB        *gorocksdb.DB
	PersistCF *gorocksdb.ColumnFamilyHandle
}

// Open open underlying rocksdb
func (r *RocksDB) Start() {
	dbPath := r.LogDir
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

	maxLogFileSize := r.MaxLogFileSize
	if maxLogFileSize > 0 {
		dbLogger.Infof("Setting rocksdb maxLogFileSize to %d", maxLogFileSize)
		opts.SetMaxLogFileSize(int(maxLogFileSize))
	}

	keepLogFileNum := r.KeepLogFileNum
	if keepLogFileNum > 0 {
		dbLogger.Infof("Setting rocksdb keepLogFileNum to %d", keepLogFileNum)
		opts.SetKeepLogFileNum(int(keepLogFileNum))
	}

	logLevelStr := r.LogLevel
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

	r.DB = db
	r.PersistCF = cfHandlers[1]
}

// Close releases all column family handles and closes rocksdb
func (r *RocksDB) Stop() {
	r.PersistCF.Destroy()
	r.DB.Close()
}

// Get returns the valud for the given column family and key
func (r *RocksDB) Get(cfHandler *gorocksdb.ColumnFamilyHandle, key []byte) ([]byte, error) {
	opt := gorocksdb.NewDefaultReadOptions()
	defer opt.Destroy()
	slice, err := r.DB.GetCF(opt, cfHandler, key)
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
func (r *RocksDB) Put(cfHandler *gorocksdb.ColumnFamilyHandle, key []byte, value []byte) error {
	opt := gorocksdb.NewDefaultWriteOptions()
	defer opt.Destroy()
	err := r.DB.PutCF(opt, cfHandler, key, value)
	if err != nil {
		dbLogger.Errorf("Error while trying to write key: %s", key)
		return err
	}
	return nil
}

// Delete delets the given key in the specified column family
func (r *RocksDB) Delete(cfHandler *gorocksdb.ColumnFamilyHandle, key []byte) error {
	opt := gorocksdb.NewDefaultWriteOptions()
	defer opt.Destroy()
	err := r.DB.DeleteCF(opt, cfHandler, key)
	if err != nil {
		dbLogger.Errorf("Error while trying to delete key: %s", key)
		return err
	}
	return nil
}

// GetSnapshot returns a point-in-time view of the DB. You MUST call snapshot.Release()
// when you are done with the snapshot.
func (r *RocksDB) GetSnapshot() *gorocksdb.Snapshot {
	return r.DB.NewSnapshot()
}

func (r *RocksDB) getFromSnapshot(snapshot *gorocksdb.Snapshot, cfHandler *gorocksdb.ColumnFamilyHandle, key []byte) ([]byte, error) {
	opt := gorocksdb.NewDefaultReadOptions()
	defer opt.Destroy()
	opt.SetSnapshot(snapshot)
	slice, err := r.DB.GetCF(opt, cfHandler, key)
	if err != nil {
		dbLogger.Errorf("Error while trying to retrieve key: %s", key)
		return nil, err
	}
	defer slice.Free()
	data := append([]byte(nil), slice.Data()...)
	return data, nil
}

// GetIterator returns an iterator for the given column family
func (r *RocksDB) GetIterator(cfHandler *gorocksdb.ColumnFamilyHandle) *gorocksdb.Iterator {
	opt := gorocksdb.NewDefaultReadOptions()
	opt.SetFillCache(true)
	defer opt.Destroy()
	return r.DB.NewIteratorCF(opt, cfHandler)
}

func (r *RocksDB) getSnapshotIterator(snapshot *gorocksdb.Snapshot, cfHandler *gorocksdb.ColumnFamilyHandle) *gorocksdb.Iterator {
	opt := gorocksdb.NewDefaultReadOptions()
	defer opt.Destroy()
	opt.SetSnapshot(snapshot)
	iter := r.DB.NewIteratorCF(opt, cfHandler)
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
