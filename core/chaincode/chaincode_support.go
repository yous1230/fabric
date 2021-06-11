/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chaincode

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/bccsp/factory"
	"github.com/hyperledger/fabric/common/metrics"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/common/viperutil"
	"github.com/hyperledger/fabric/core/chaincode/platforms"
	"github.com/hyperledger/fabric/core/common/ccprovider"
	"github.com/hyperledger/fabric/core/common/sysccprovider"
	"github.com/hyperledger/fabric/core/container/ccintf"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/peer"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/pkg/errors"
)

// Runtime is used to manage chaincode runtime instances.
type Runtime interface {
	Start(ccci *ccprovider.ChaincodeContainerInfo, codePackage []byte) error
	Stop(ccci *ccprovider.ChaincodeContainerInfo) error
	Wait(ccci *ccprovider.ChaincodeContainerInfo) (int, error)
}

// Launcher is used to launch chaincode runtimes.
type Launcher interface {
	Launch(ccci *ccprovider.ChaincodeContainerInfo) error
}

// Lifecycle provides a way to retrieve chaincode definitions and the packages necessary to run them
type Lifecycle interface {
	// ChaincodeDefinition returns the details for a chaincode by name
	ChaincodeDefinition(chaincodeName string, qe ledger.QueryExecutor) (ccprovider.ChaincodeDefinition, error)

	// ChaincodeContainerInfo returns the package necessary to launch a chaincode
	ChaincodeContainerInfo(chaincodeName string, qe ledger.QueryExecutor) (*ccprovider.ChaincodeContainerInfo, error)
}

// ChaincodeSupport responsible for providing interfacing with chaincodes from the Peer.
type ChaincodeSupport struct {
	Keepalive        time.Duration
	ExecuteTimeout   time.Duration
	UserRunsCC       bool
	Runtime          Runtime
	ACLProvider      ACLProvider
	HandlerRegistry  *HandlerRegistry
	Launcher         Launcher
	SystemCCProvider sysccprovider.SystemChaincodeProvider
	Lifecycle        Lifecycle
	appConfig        ApplicationConfigRetriever
	HandlerMetrics   *HandlerMetrics
	LaunchMetrics    *LaunchMetrics
}

// NewChaincodeSupport creates a new ChaincodeSupport instance.
func NewChaincodeSupport(
	config *Config,
	peerAddress string,
	userRunsCC bool,
	caCert []byte,
	certGenerator CertGenerator,
	packageProvider PackageProvider,
	lifecycle Lifecycle,
	aclProvider ACLProvider,
	processor Processor,
	SystemCCProvider sysccprovider.SystemChaincodeProvider,
	platformRegistry *platforms.Registry,
	appConfig ApplicationConfigRetriever,
	metricsProvider metrics.Provider,
) *ChaincodeSupport {
	cs := &ChaincodeSupport{
		UserRunsCC:       userRunsCC,
		Keepalive:        config.Keepalive,
		ExecuteTimeout:   config.ExecuteTimeout,
		HandlerRegistry:  NewHandlerRegistry(userRunsCC),
		ACLProvider:      aclProvider,
		SystemCCProvider: SystemCCProvider,
		Lifecycle:        lifecycle,
		appConfig:        appConfig,
		HandlerMetrics:   NewHandlerMetrics(metricsProvider),
		LaunchMetrics:    NewLaunchMetrics(metricsProvider),
	}

	// Keep TestQueries working
	if !config.TLSEnabled {
		certGenerator = nil
	}

	var bccspConfig *factory.FactoryOpts
	if err := viperutil.EnhancedExactUnmarshalKey("peer.BCCSP", &bccspConfig); err != nil {
		panic(fmt.Sprintf("could not parse YAML config: %v", err))
	}

	var bccspEnv []string
	switch bccspConfig.ProviderName {
	case "SW":
		bccspEnv = []string{
			"CORE_PEER_BCCSP_DEFAULT=SW",
			"CORE_PEER_BCCSP_SW_ALGORITHM=" + bccspConfig.SwOpts.Algorithm,
			"CORE_PEER_BCCSP_SW_HASH=" + bccspConfig.SwOpts.HashFamily,
			"CORE_PEER_BCCSP_SW_SECURITY=" + strconv.Itoa(bccspConfig.SwOpts.SecLevel),
		}
	case "PLUGIN":
		bccspEnv = []string{
			"CORE_PEER_BCCSP_DEFAULT=PLUGIN",
			"CORE_PEER_BCCSP_PLUGIN_LIBRARY=" + bccspConfig.PluginOpts.Library,
			"CORE_PEER_BCCSP_PLUGIN_CONFIG_ALGORITHM=" + bccspConfig.PluginOpts.Config["Algorithm"].(string),
			"CORE_PEER_BCCSP_PLUGIN_CONFIG_HASH=" + bccspConfig.PluginOpts.Config["Hash"].(string),
			"CORE_PEER_BCCSP_PLUGIN_CONFIG_SECURITY=" + bccspConfig.PluginOpts.Config["Security"].(string),
		}
	default:
		panic(fmt.Sprintf("no '%s' provider", bccspConfig.ProviderName))
	}
	commonEnv := []string{
		"CORE_CHAINCODE_LOGGING_LEVEL=" + config.LogLevel,
		"CORE_CHAINCODE_LOGGING_SHIM=" + config.ShimLogLevel,
		"CORE_CHAINCODE_LOGGING_FORMAT=" + config.LogFormat,
	}
	commonEnv = append(commonEnv, bccspEnv...)

	cs.Runtime = &ContainerRuntime{
		CertGenerator:    certGenerator,
		Processor:        processor,
		CACert:           caCert,
		PeerAddress:      peerAddress,
		PlatformRegistry: platformRegistry,
		CommonEnv:        commonEnv,
	}

	cs.Launcher = &RuntimeLauncher{
		Runtime:         cs.Runtime,
		Registry:        cs.HandlerRegistry,
		PackageProvider: packageProvider,
		StartupTimeout:  config.StartupTimeout,
		Metrics:         cs.LaunchMetrics,
	}

	return cs
}

// LaunchInit bypasses getting the chaincode spec from the LSCC table
// as in the case of v1.0-v1.2 lifecycle, the chaincode will not yet be
// defined in the LSCC table
func (cs *ChaincodeSupport) LaunchInit(ccci *ccprovider.ChaincodeContainerInfo) error {
	cname := ccci.Name + ":" + ccci.Version
	if cs.HandlerRegistry.Handler(cname) != nil {
		println("Already exit!")
		return nil
	}
	println("Launch registry --------")
	return cs.Launcher.Launch(ccci)
}

// Launch starts executing chaincode if it is not already running. This method
// blocks until the peer side handler gets into ready state or encounters a fatal
// error. If the chaincode is already running, it simply returns.
func (cs *ChaincodeSupport) Launch(chainID, chaincodeName, chaincodeVersion string, qe ledger.QueryExecutor) (*Handler, error) {
	println("enter ChaincodeSupport.Launch -------")
	cname := chaincodeName + ":" + chaincodeVersion
	if h := cs.HandlerRegistry.Handler(cname); h != nil {
		return h, nil
	}

	ccci, err := cs.Lifecycle.ChaincodeContainerInfo(chaincodeName, qe)
	if err != nil {
		// TODO: There has to be a better way to do this...
		if cs.UserRunsCC {
			chaincodeLogger.Error(
				"You are attempting to perform an action other than Deploy on Chaincode that is not ready and you are in developer mode. Did you forget to Deploy your chaincode?",
			)
		}

		return nil, errors.Wrapf(err, "[channel %s] failed to get chaincode container info for %s", chainID, cname)
	}

	if err := cs.Launcher.Launch(ccci); err != nil {
		return nil, errors.Wrapf(err, "[channel %s] could not launch chaincode %s", chainID, cname)
	}

	h := cs.HandlerRegistry.Handler(cname)
	if h == nil {
		return nil, errors.Wrapf(err, "[channel %s] claimed to start chaincode container for %s but could not find handler", chainID, cname)
	}

	return h, nil
}

// Stop stops a chaincode if running.
func (cs *ChaincodeSupport) Stop(ccci *ccprovider.ChaincodeContainerInfo) error {
	return cs.Runtime.Stop(ccci)
}

// HandleChaincodeStream implements ccintf.HandleChaincodeStream for all vms to call with appropriate stream
func (cs *ChaincodeSupport) HandleChaincodeStream(stream ccintf.ChaincodeStream) error {
	handler := &Handler{
		Invoker:                    cs,
		DefinitionGetter:           cs.Lifecycle,
		Keepalive:                  cs.Keepalive,
		Registry:                   cs.HandlerRegistry,
		ACLProvider:                cs.ACLProvider,
		TXContexts:                 NewTransactionContexts(),
		ActiveTransactions:         NewActiveTransactions(),
		SystemCCProvider:           cs.SystemCCProvider,
		SystemCCVersion:            util.GetSysCCVersion(),
		InstantiationPolicyChecker: CheckInstantiationPolicyFunc(ccprovider.CheckInstantiationPolicy),
		QueryResponseBuilder:       &QueryResponseGenerator{MaxResultLimit: 100},
		UUIDGenerator:              UUIDGeneratorFunc(util.GenerateUUID),
		LedgerGetter:               peer.Default,
		AppConfig:                  cs.appConfig,
		Metrics:                    cs.HandlerMetrics,
	}

	return handler.ProcessStream(stream)
}

// Register the bidi stream entry point called by chaincode to register with the Peer.
func (cs *ChaincodeSupport) Register(stream pb.ChaincodeSupport_RegisterServer) error {
	return cs.HandleChaincodeStream(stream)
}

// createCCMessage creates a transaction message.
func createCCMessage(messageType pb.ChaincodeMessage_Type, cid string, txid string, cMsg *pb.ChaincodeInput) (*pb.ChaincodeMessage, error) {
	payload, err := proto.Marshal(cMsg)
	if err != nil {
		return nil, err
	}
	ccmsg := &pb.ChaincodeMessage{
		Type:      messageType,
		Payload:   payload,
		Txid:      txid,
		ChannelId: cid,
	}
	return ccmsg, nil
}

// ExecuteLegacyInit is a temporary method which should be removed once the old style lifecycle
// is entirely deprecated.  Ideally one release after the introduction of the new lifecycle.
// It does not attempt to start the chaincode based on the information from lifecycle, but instead
// accepts the container information directly in the form of a ChaincodeDeploymentSpec.
func (cs *ChaincodeSupport) ExecuteLegacyInit(txParams *ccprovider.TransactionParams, cccid *ccprovider.CCContext, spec *pb.ChaincodeDeploymentSpec) (*pb.Response, *pb.ChaincodeEvent, error) {
	print("Enter LaunchInit ----------")
	ccci := ccprovider.DeploymentSpecToChaincodeContainerInfo(spec)
	ccci.Version = cccid.Version
	// 检索是否已经存在，存在直接返回，不存在就进行启动注册
	// 启动链码
	err := cs.LaunchInit(ccci)
	if err != nil {
		println("LaunchInit err!!")
		return nil, nil, err
	}
	println("LaunchInit succeed -------")
	cname := ccci.Name + ":" + ccci.Version
	// 注册链码，启动链码容器
	h := cs.HandlerRegistry.Handler(cname)
	if h == nil {
		return nil, nil, errors.Wrapf(err, "[channel %s] claimed to start chaincode container for %s but could not find handler", txParams.ChannelID, cname)
	}
	println("chaincode docker has launched")

	// 调用链码Init
	resp, err := cs.execute(pb.ChaincodeMessage_INIT, txParams, cccid, spec.GetChaincodeSpec().Input, h)
	if err != nil {
		fmt.Printf("调用链码Init错误 %s", err.Error())
		println("调用链码Init错误 %s", err.Error())
	}
	println("invoke chaincode ", cccid.Name)
	return processChaincodeExecutionResult(txParams.TxID, cccid.Name, resp, err)
}

// Execute invokes chaincode and returns the original response.
func (cs *ChaincodeSupport) Execute(txParams *ccprovider.TransactionParams, cccid *ccprovider.CCContext, input *pb.ChaincodeInput) (*pb.Response, *pb.ChaincodeEvent, error) {
	resp, err := cs.Invoke(txParams, cccid, input)
	return processChaincodeExecutionResult(txParams.TxID, cccid.Name, resp, err)
}

func processChaincodeExecutionResult(txid, ccName string, resp *pb.ChaincodeMessage, err error) (*pb.Response, *pb.ChaincodeEvent, error) {
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to execute transaction %s", txid)
	}
	if resp == nil {
		return nil, nil, errors.Errorf("nil response from transaction %s", txid)
	}

	if resp.ChaincodeEvent != nil {
		resp.ChaincodeEvent.ChaincodeId = ccName
		resp.ChaincodeEvent.TxId = txid
	}

	println("resp is not nil -------")
	switch resp.Type {
	case pb.ChaincodeMessage_COMPLETED:
		//fmt.Printf("%s chaincode excute success ChaincodeMessage_COMPLETED", ccName)
		println("chaincode excute success ChaincodeMessage_COMPLETED ", ccName)
		res := &pb.Response{}
		println("Response.Status = ", res.Status, ", Response.Message = ", res.Message, ", Response.Payload = ", res.Payload)
		err := proto.Unmarshal(resp.Payload, res)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to unmarshal response for transaction %s", txid)
		}
		println("no error!")
		println("ChaincodeMessage_COMPLETED res is ", res)

		return res, resp.ChaincodeEvent, nil

	case pb.ChaincodeMessage_ERROR:
		//fmt.Printf("%s chaincode excute failed ChaincodeMessage_ERROR", ccName)
		fmt.Printf("error: %s", resp.Payload)
		println("%s chaincode excute failed ChaincodeMessage_ERROR ", ccName)
		return nil, resp.ChaincodeEvent, errors.Errorf("transaction returned with failure: %s", resp.Payload)

	default:
		println("%s chaincode excute failed ChaincodeMessage_ERROR default ", ccName)
		return nil, nil, errors.Errorf("unexpected response type %d for transaction %s", resp.Type, txid)
	}
}

func (cs *ChaincodeSupport) InvokeInit(txParams *ccprovider.TransactionParams, cccid *ccprovider.CCContext, input *pb.ChaincodeInput) (*pb.ChaincodeMessage, error) {
	h, err := cs.Launch(txParams.ChannelID, cccid.Name, cccid.Version, txParams.TXSimulator)
	if err != nil {
		return nil, err
	}

	println("cs.Launch execute success")
	return cs.execute(pb.ChaincodeMessage_INIT, txParams, cccid, input, h)
}

// Invoke will invoke chaincode and return the message containing the response.
// The chaincode will be launched if it is not already running.
func (cs *ChaincodeSupport) Invoke(txParams *ccprovider.TransactionParams, cccid *ccprovider.CCContext, input *pb.ChaincodeInput) (*pb.ChaincodeMessage, error) {
	println("enter chaincode.invoke -------")
	h, err := cs.Launch(txParams.ChannelID, cccid.Name, cccid.Version, txParams.TXSimulator)
	if err != nil {
		return nil, err
	}

	// TODO add Init exactly once semantics here once new lifecycle
	// is available.  Enforced if the target channel is using the new lifecycle
	//
	// First, the function name of the chaincode to invoke should be checked.  If it is
	// "init", then consider this invocation to be of type pb.ChaincodeMessage_INIT,
	// otherwise consider it to be of type pb.ChaincodeMessage_TRANSACTION,
	//
	// Secondly, A check should be made whether the chaincode has been
	// inited, then, if true, only allow cctyp pb.ChaincodeMessage_TRANSACTION,
	// otherwise, only allow cctype pb.ChaincodeMessage_INIT,
	cctype := pb.ChaincodeMessage_TRANSACTION
	println("execute chaincodesupport ------")

	return cs.execute(cctype, txParams, cccid, input, h)
}

// execute executes a transaction and waits for it to complete until a timeout value.
func (cs *ChaincodeSupport) execute(cctyp pb.ChaincodeMessage_Type, txParams *ccprovider.TransactionParams, cccid *ccprovider.CCContext, input *pb.ChaincodeInput, h *Handler) (*pb.ChaincodeMessage, error) {
	input.Decorations = txParams.ProposalDecorations
	ccMsg, err := createCCMessage(cctyp, txParams.ChannelID, txParams.TxID, input)
	if err != nil {
		fmt.Printf("extuce createCCMessage error: %s", err.Error())
		return nil, errors.WithMessage(err, "failed to create chaincode message")
	}

	ccresp, err := h.Execute(txParams, cccid, ccMsg, cs.ExecuteTimeout)
	if err != nil {
		fmt.Printf("extuce h.Execute error: %s", err.Error())
		return nil, errors.WithMessage(err, fmt.Sprintf("error sending"))
	}

	return ccresp, nil
}
