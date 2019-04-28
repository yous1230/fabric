/*
Copyright IBM Corp. 2017 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package server

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hyperledger/fabric/common/ledger/blkstorage/fsblkstorage"
	"github.com/hyperledger/fabric/common/ledger/blockledger"
	"github.com/hyperledger/fabric/common/ledger/blockledger/file"
	"github.com/hyperledger/fabric/common/ledger/blockledger/json"
	"github.com/hyperledger/fabric/common/ledger/blockledger/ram"
	config "github.com/hyperledger/fabric/orderer/common/localconfig"
	"github.com/hyperledger/fabric/orderer/consensus/sbft/backend"
	sbftcrypto "github.com/hyperledger/fabric/orderer/consensus/sbft/crypto"
	sb "github.com/hyperledger/fabric/protos/orderer/sbft"
)

func createLedgerFactory(conf *config.TopLevel) (blockledger.Factory, string) {
	var lf blockledger.Factory
	var ld string
	switch conf.General.LedgerType {
	case "file":
		ld = conf.FileLedger.Location
		if ld == "" {
			ld = createTempDir(conf.FileLedger.Prefix)
		}
		logger.Debug("Ledger dir:", ld)
		lf = fileledger.New(ld)
		// The file-based ledger stores the blocks for each channel
		// in a fsblkstorage.ChainsDir sub-directory that we have
		// to create separately. Otherwise the call to the ledger
		// Factory's ChainIDs below will fail (dir won't exist).
		createSubDir(ld, fsblkstorage.ChainsDir)
	case "json":
		ld = conf.FileLedger.Location
		if ld == "" {
			ld = createTempDir(conf.FileLedger.Prefix)
		}
		logger.Debug("Ledger dir:", ld)
		lf = jsonledger.New(ld)
	case "ram":
		fallthrough
	default:
		lf = ramledger.New(int(conf.RAMLedger.HistorySize))
	}
	return lf, ld
}

func createTempDir(dirPrefix string) string {
	dirPath, err := ioutil.TempDir("", dirPrefix)
	if err != nil {
		logger.Panic("Error creating temp dir:", err)
	}
	return dirPath
}

func createSubDir(parentDirPath string, subDir string) (string, bool) {
	var created bool
	subDirPath := filepath.Join(parentDirPath, subDir)
	if _, err := os.Stat(subDirPath); err != nil {
		if os.IsNotExist(err) {
			if err = os.Mkdir(subDirPath, 0755); err != nil {
				logger.Panic("Error creating sub dir:", err)
			}
			created = true
		}
	} else {
		logger.Debugf("Found %s sub-dir and using it", fsblkstorage.ChainsDir)
	}
	return subDirPath, created
}

// XXX The functions below need to be moved to the SBFT package ASAP
func makeSbftConsensusConfig(conf *config.TopLevel) *sb.ConsensusConfig {
	cfg := sb.Config{N: conf.Genesis.SbftShared.N, F: conf.Genesis.SbftShared.F,
		BatchDurationNsec:  uint64(conf.Genesis.DeprecatedBatchTimeout),
		BatchSizeBytes:     uint64(conf.Genesis.DeprecatedBatchSize),
		RequestTimeoutNsec: conf.Genesis.SbftShared.RequestTimeoutNsec}
	peers := make(map[string][]byte)
	for addr, cert := range conf.Genesis.SbftShared.Peers {
		var err error
		peers[addr], err = sbftcrypto.ParseCertPEM(cert)
		if err != nil {
			logger.Error("MakeSbftConsensusConfig failed:", err)
		}
	}
	return &sb.ConsensusConfig{Consensus: &cfg, Peers: peers}
}

func makeSbftStackConfig(conf *config.TopLevel) *backend.StackConfig {
	return &backend.StackConfig{ListenAddr: conf.SbftLocal.PeerCommAddr,
		CertFile: conf.SbftLocal.CertFile,
		KeyFile:  conf.SbftLocal.KeyFile,
		DataDir:  conf.SbftLocal.DataDir}
}
