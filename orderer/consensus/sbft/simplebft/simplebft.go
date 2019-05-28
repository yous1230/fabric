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

package simplebft

import (
	"fmt"
	"math"
	"reflect"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/orderer/common/localconfig"
	"github.com/hyperledger/fabric/orderer/consensus"
	"github.com/hyperledger/fabric/orderer/consensus/sbft/persist"
	sb "github.com/hyperledger/fabric/protos/orderer/sbft"
)

const preprepared string = "preprepared"
const prepared string = "prepared"
const committed string = "committed"
const viewchange string = "viewchange"

// Receiver defines the API that is exposed by SBFT to the system.
type Receiver interface {
	Receive(msg *sb.Msg, src uint64)
	Request(req []byte)
	Connection(replica uint64)
	GetChainId() string
}

// System defines the API that needs to be provided for SBFT.
type System interface {
	Send(chainId string, msg *sb.Msg, dest uint64)
	Timer(d time.Duration, f func()) Canceller
	Deliver(chainId string, batch *sb.Batch)
	AddReceiver(chainId string, receiver Receiver)
	LastBatch(chainId string) *sb.Batch
	Sign(data []byte) []byte
	CheckSig(data []byte, src uint64, sig []byte) error
	Reconnect(chainId string, replica uint64)
	Validate(chainID string, req *sb.Request) ([][]*sb.Request, bool)
	Cut(chainID string) []*sb.Request
}

// Canceller allows cancelling of a scheduled timer event.
type Canceller interface {
	Cancel()
}

// SBFT is a simplified PBFT implementation.
type SBFT struct {
	sys         System
	support     consensus.ConsenterSupport
	logger      *flogging.FabricLogger
	persistence *persist.Persist

	config            sb.Options
	id                uint64
	view              uint64
	blocks            [][]*sb.Request
	batchTimer        Canceller
	cur               reqInfo
	activeView        bool
	lastNewViewSent   *sb.NewView
	viewChangeTimeout time.Duration
	viewChangeTimer   Canceller
	replicaState      []replicaInfo
	pending           map[string]*sb.Request
	validated         map[string]bool
	chainId           string
}

type reqInfo struct {
	subject        sb.Subject
	timeout        Canceller
	preprep        *sb.Preprepare
	prep           map[uint64]*sb.Subject
	commit         map[uint64]*sb.Subject
	checkpoint     map[uint64]*sb.Checkpoint
	prepared       bool
	committed      bool
	checkpointDone bool
}

type replicaInfo struct {
	backLog          []*sb.Msg
	hello            *sb.Hello
	signedViewchange *sb.Signed
	viewchange       *sb.ViewChange
}

type dummyCanceller struct{}

func (d dummyCanceller) Cancel() {}

// New creates a new SBFT instance.
func New(id uint64, sc *localconfig.SbftLocal, support consensus.ConsenterSupport, config *sb.Options, sys System) (*SBFT, error) {
	chainID := support.ChainID()
	if config.F*3+1 > config.N {
		return nil, fmt.Errorf("invalid combination of N (%d) and F (%d)", config.N, config.F)
	}

	s := &SBFT{
		sys:             sys,
		support:         support,
		logger:          flogging.MustGetLogger("orderer.consensus.sbft.simplebft"),
		persistence:     persist.New(sc.DataDir, sc.Db.LogLevel, sc.Db.MaxLogFileSize, sc.Db.KeepLogFileNum),
		config:          *config,
		id:              id,
		chainId:         chainID,
		viewChangeTimer: dummyCanceller{},
		replicaState:    make([]replicaInfo, config.N),
		pending:         make(map[string]*sb.Request),
		validated:       make(map[string]bool),
		blocks:          make([][]*sb.Request, 0, 3),
	}
	s.sys.AddReceiver(chainID, s)

	s.view = 0
	s.cur.subject.Seq = &sb.SeqView{}
	s.cur.prepared = true
	s.cur.committed = true
	s.cur.checkpointDone = true
	s.cur.timeout = dummyCanceller{}
	s.activeView = true

	svc := &sb.Signed{}
	if s.Restore(s.chainId, viewchange, svc) {
		vc := &sb.ViewChange{}
		err := proto.Unmarshal(svc.Data, vc)
		if err != nil {
			return nil, err
		}
		fmt.Println(fmt.Sprintf("rep %d VIEW %d   %d", s.id, s.view, vc.View))
		s.view = vc.View
		s.replicaState[s.id].signedViewchange = svc
		s.activeView = false
	}

	pp := &sb.Preprepare{}
	if s.Restore(s.chainId, preprepared, pp) && pp.Seq.View >= s.view {
		s.view = pp.Seq.View
		s.activeView = true
		if pp.Seq.Seq > s.seq() {
			// TODO double add to BC?
			//_, committers := s.getCommittersFromBatch(pp.Batch)
			s.acceptPreprepare(pp)
		}
	}
	c := &sb.Subject{}
	if s.Restore(s.chainId, prepared, c) && reflect.DeepEqual(c, &s.cur.subject) && c.Seq.View >= s.view {
		s.cur.prepared = true
	}
	ex := &sb.Subject{}
	if s.Restore(s.chainId, committed, ex) && reflect.DeepEqual(c, &s.cur.subject) && ex.Seq.View >= s.view {
		s.cur.committed = true
	}

	s.cancelViewChangeTimer()
	return s, nil
}

////////////////////////////////////////////////

func (s *SBFT) GetChainId() string {
	return s.chainId
}

func (s *SBFT) primaryIDView(v uint64) uint64 {
	return v % s.config.N
}

func (s *SBFT) primaryID() uint64 {
	return s.primaryIDView(s.view)
}

func (s *SBFT) isPrimary() bool {
	return s.primaryID() == s.id
}

func (s *SBFT) seq() uint64 {
	return s.sys.LastBatch(s.chainId).DecodeHeader().Seq
}

func (s *SBFT) nextSeq() sb.SeqView {
	return sb.SeqView{Seq: s.seq() + 1, View: s.view}
}

func (s *SBFT) nextView() uint64 {
	return s.view + 1
}

func (s *SBFT) commonCaseQuorum() int {
	//When N=3F+1 this should be 2F+1 (N-F)
	//More generally, we need every two common case quorums of size X to intersect in at least F+1 orderers,
	//hence 2X>=N+F+1, or X is:
	return int(math.Ceil(float64(s.config.N+s.config.F+1) / float64(2)))
}

func (s *SBFT) viewChangeQuorum() int {
	//When N=3F+1 this should be 2F+1 (N-F)
	//More generally, we need every view change quorum to intersect with every common case quorum at least F+1 orderers, hence:
	//Y >= N-X+F+1
	return int(s.config.N+s.config.F+1) - s.commonCaseQuorum()
}

func (s *SBFT) oneCorrectQuorum() int {
	return int(s.config.F + 1)
}

func (s *SBFT) broadcast(m *sb.Msg) {
	for i := uint64(0); i < s.config.N; i++ {
		s.sys.Send(s.chainId, m, i)
	}
}

////////////////////////////////////////////////

// Receive is the ingress method for SBFT messages.
func (s *SBFT) Receive(m *sb.Msg, src uint64) {
	s.logger.Debugf("replica %d: received message from %d: %s", s.id, src, m.Type)

	if h := m.GetHello(); h != nil {
		s.handleHello(h, src)
		return
	} else if req := m.GetRequest(); req != nil {
		s.handleRequest(req, src)
		return
	} else if vs := m.GetViewChange(); vs != nil {
		s.handleViewChange(vs, src)
		return
	} else if nv := m.GetNewView(); nv != nil {
		s.handleNewView(nv, src)
		return
	}

	if s.testBacklogMessage(m, src) {
		s.logger.Debugf("replica %d: message for future seq, storing for later", s.id)
		s.recordBacklogMsg(m, src)
		return
	}

	s.handleQueueableMessage(m, src)
}

func (s *SBFT) handleQueueableMessage(m *sb.Msg, src uint64) {
	if pp := m.GetPreprepare(); pp != nil {
		s.handlePreprepare(pp, src)
		return
	} else if p := m.GetPrepare(); p != nil {
		s.handlePrepare(p, src)
		return
	} else if c := m.GetCommit(); c != nil {
		s.handleCommit(c, src)
		return
	} else if c := m.GetCheckpoint(); c != nil {
		s.handleCheckpoint(c, src)
		return
	}

	s.logger.Warningf("replica %d: received invalid message from %d", s.id, src)
}

func (s *SBFT) deliverBatch(batch *sb.Batch) {
	//if committers == nil {
	//	logger.Warningf("replica %d: commiter is nil", s.id)
	//	panic("Committer is nil.")
	//}
	s.cur.checkpointDone = true
	s.cur.timeout.Cancel()
	// s.primarycommitters[0]
	s.sys.Deliver(s.chainId, batch)
	// s.primarycommitters = s.primarycommitters[1:]

	for _, req := range batch.Payloads {
		key := hash2str(hash(req))
		s.logger.Infof("replica %d: attempting to remove %x from pending", s.id, key)
		delete(s.pending, key)
		delete(s.validated, key)
	}
}

////////////////////////////////////////////////
func (s *SBFT) StartRocksDb() {
	s.persistence.Start()
}

func (s *SBFT) StopRocksDb() {
	s.persistence.Stop()
}

// Persist persists data identified by a chainId and a key
func (s *SBFT) Persist(chainId string, key string, data proto.Message) {
	compk := fmt.Sprintf("%s-%s", chainId, key)
	if data == nil {
		s.persistence.DelState(compk)
	} else {
		bytes, err := proto.Marshal(data)
		if err != nil {
			panic(err)
		}
		s.persistence.StoreState(compk, bytes)
	}
}

// Restore loads persisted data identified by chainId and key
func (s *SBFT) Restore(chainId string, key string, out proto.Message) bool {
	compk := fmt.Sprintf("%s-%s", chainId, key)
	val, err := s.persistence.ReadState(compk)
	if err != nil {
		return false
	}
	err = proto.Unmarshal(val, out)
	return err == nil
}

// Delete persisted data identified by chainId and key
func (s *SBFT) Delete(chainId string, key string) {
	compk := fmt.Sprintf("%s-%s", chainId, key)
	s.persistence.DelState(compk)
}
