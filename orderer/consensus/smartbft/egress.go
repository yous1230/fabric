/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package smartbft

import (
	"sync/atomic"

	protos "github.com/SmartBFT-Go/consensus/smartbftprotos"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/orderer"
	"github.com/hyperledger/fabric/protos/utils"
)

//go:generate mockery -dir . -name RPC -case underscore -output mocks

type RPC interface {
	SendConsensus(dest uint64, msg *orderer.ConsensusRequest) error
	SendSubmit(dest uint64, request *orderer.SubmitRequest) error
}

type Logger interface {
	Warnf(template string, args ...interface{})
	Panicf(template string, args ...interface{})
}

type Egress struct {
	ConvertMessage func(m *protos.Message, channel string) *orderer.ConsensusRequest
	Channel        string
	RPC            RPC
	Logger         Logger
	RuntimeConfig  *atomic.Value
}

func (e *Egress) Nodes() []uint64 {
	nodes := e.RuntimeConfig.Load().(RuntimeConfig).Nodes
	var res []uint64
	for _, n := range nodes {
		res = append(res, n)
	}
	return res
}

func (e *Egress) SendConsensus(targetID uint64, m *protos.Message) {
	err := e.RPC.SendConsensus(targetID, e.ConvertMessage(m, e.Channel))
	if err != nil {
		e.Logger.Warnf("Failed sending to %d: %v", targetID, err)
	}
}

func (e *Egress) SendTransaction(targetID uint64, request []byte) {
	env := &common.Envelope{}
	err := proto.Unmarshal(request, env)
	if err != nil {
		e.Logger.Panicf("Failed unmarshaling request %v to envelope: %v", request, err)
	}
	msg := &orderer.SubmitRequest{
		Channel: e.Channel,
		Payload: env,
	}
	e.RPC.SendSubmit(targetID, msg)
}

func bftMsgToClusterMsg(message *protos.Message, channel string) *orderer.ConsensusRequest {
	return &orderer.ConsensusRequest{
		Payload: utils.MarshalOrPanic(message),
		Channel: channel,
	}
}
