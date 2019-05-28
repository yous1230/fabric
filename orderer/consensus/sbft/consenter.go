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

package sbft

import (
	"strconv"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/comm"
	"github.com/hyperledger/fabric/orderer/common/localconfig"
	"github.com/hyperledger/fabric/orderer/consensus"
	"github.com/hyperledger/fabric/orderer/consensus/migration"
	"github.com/hyperledger/fabric/orderer/consensus/sbft/backend"
	"github.com/hyperledger/fabric/orderer/consensus/sbft/connection"
	"github.com/hyperledger/fabric/orderer/consensus/sbft/persist"
	cb "github.com/hyperledger/fabric/protos/common"
	sb "github.com/hyperledger/fabric/protos/orderer/sbft"
	"github.com/pkg/errors"
)

var once sync.Once

// Consenter interface implementation for new main application
type consenter struct {
	cert        []byte
	mspId       string
	config      *sb.ConsensusConfig
	sbftConfig  *localconfig.SbftLocal
	persistence *persist.Persist
	backend     *backend.Backend
	logger      *flogging.FabricLogger
}

type chain struct {
	chainID         string
	exitChan        chan struct{}
	backend         *backend.Backend
	migrationStatus migration.Status
}

// New creates a new consenter for the SBFT consensus scheme.
// It accepts messages being delivered via Enqueue, orders them, and then uses the blockcutter to form the messages
// into blocks before writing to the given ledger.
func New(conf *localconfig.TopLevel, srvConf comm.ServerConfig) consensus.Consenter {
	logger := flogging.MustGetLogger("orderer.consensus.sbft")
	persistence := persist.New(conf.SbftLocal.DataDir, conf.SbftLocal.Db.LogLevel,
		conf.SbftLocal.Db.MaxLogFileSize, conf.SbftLocal.Db.KeepLogFileNum)
	persistence.Start()
	return &consenter{
		cert:        srvConf.SecOpts.Certificate,
		mspId:       conf.General.LocalMSPDir,
		sbftConfig:  &conf.SbftLocal,
		persistence: persistence,
		logger:      logger}
}

func (c *consenter) HandleChain(support consensus.ConsenterSupport, metadata *cb.Metadata) (consensus.Chain, error) {
	c.logger.Infof("Starting a chain: %d", support.ChainID())

	m := &sb.ConfigMetadata{}
	if err := proto.Unmarshal(support.SharedConfig().ConsensusMetadata(), m); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal consensus metadata")
	}
	if m.Options == nil {
		return nil, errors.New("Sbft options have not been provided")
	}
	if len(m.Consenters) == 0 {
		return nil, errors.New("Sbft consenters have not been provided")
	}
	peers := make(map[string]*sb.Consenter)
	for _, consenter := range m.Consenters {
		endpoint := consenter.Host + ":" + strconv.FormatUint(uint64(consenter.Port), 10)
		peers[endpoint] = consenter
	}

	c.config = &sb.ConsensusConfig{
		Consensus: m.Options,
		Peers:     peers,
	}

	if c.backend == nil {
		conn, err := connection.New(c.sbftConfig.PeerCommAddr, c.sbftConfig.CertFile, c.sbftConfig.KeyFile)
		if err != nil {
			c.logger.Errorf("Error when trying to connect: %s", err)
			panic(err)
		}

		pBackend, err := backend.NewBackend(c.config.Peers, conn, c.cert, c.mspId)
		if err != nil {
			c.logger.Errorf("Backend instantiation error: %v", err)
			panic(err)
		}
		c.backend = pBackend
	}
	c.backend.InitSbftPeer(c.persistence, c.config, support)

	return &chain{
		chainID:         support.ChainID(),
		exitChan:        make(chan struct{}),
		backend:         c.backend,
		migrationStatus: migration.NewStatusStepper(support.IsSystemChannel(), support.ChainID()), // Needed by consensus-type migration
	}, nil
}

// Chain interface implementation:

// Start allocates the necessary resources for staying up to date with this Chain.
// It implements the multichain.Chain interface. It is called by multichain.NewManagerImpl()
// which is invoked when the ordering process is launched, before the call to NewServer().
func (ch *chain) Start() {
	once.Do(ch.backend.StartAndConnectWorkers)
}

// Halt frees the resources which were allocated for this Chain
func (ch *chain) Halt() {
	// panic("There is no way to halt SBFT")
	select {
	case <-ch.exitChan:
		// Allow multiple halts without panic
	default:
		close(ch.exitChan)
	}
}

func (ch *chain) WaitReady() error {
	return nil
}

// Order accepts normal messages for ordering
func (ch *chain) Order(env *cb.Envelope, configSeq uint64) error {
	return ch.backend.Enqueue(ch.chainID, env)
}

// Configure accepts configuration update messages for ordering
func (ch *chain) Configure(config *cb.Envelope, configSeq uint64) error {
	return ch.backend.Enqueue(ch.chainID, config)
}

// Errored only closes on exit
func (ch *chain) Errored() <-chan struct{} {
	return ch.exitChan
}

func (ch *chain) MigrationStatus() migration.Status {
	return ch.migrationStatus
}
