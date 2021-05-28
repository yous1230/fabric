/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chaincode

import (
	"strconv"
	"time"

	"github.com/hyperledger/fabric/core/common/ccprovider"
	"github.com/hyperledger/fabric/core/container/inproccontroller"
	"github.com/pkg/errors"
)

// LaunchRegistry tracks launching chaincode instances.
type LaunchRegistry interface {
	Launching(cname string) (launchState *LaunchState, started bool)
	Deregister(cname string) error
}

// PackageProvider gets chaincode packages from the filesystem.
type PackageProvider interface {
	GetChaincodeCodePackage(ccname string, ccversion string) ([]byte, error)
}

// RuntimeLauncher is responsible for launching chaincode runtimes.
type RuntimeLauncher struct {
	Runtime         Runtime
	Registry        LaunchRegistry
	PackageProvider PackageProvider
	StartupTimeout  time.Duration
	Metrics         *LaunchMetrics
}

func (r *RuntimeLauncher) Launch(ccci *ccprovider.ChaincodeContainerInfo) error {
	var startFailCh chan error
	var timeoutCh <-chan time.Time

	startTime := time.Now()
	cname := ccci.Name + ":" + ccci.Version
	launchState, alreadyStarted := r.Registry.Launching(cname)
	if !alreadyStarted {
		startFailCh = make(chan error, 1)
		timeoutCh = time.NewTimer(r.StartupTimeout).C

		println("launch alreadyStarted")

		codePackage, err := r.getCodePackage(ccci)
		if err != nil {
			return err
		}

		go func() {
			// 启动链码容器运行的下一个入口
			println("launch ccc -----")

			if err := r.Runtime.Start(ccci, codePackage); err != nil {
				startFailCh <- errors.WithMessage(err, "error starting container")
				return
			}
			exitCode, err := r.Runtime.Wait(ccci)
			if err != nil {
				launchState.Notify(errors.Wrap(err, "failed to wait on container exit"))
			}
			println("7777777")
			launchState.Notify(errors.Errorf("container exited with %d", exitCode))
		}()
	}

	var err error
	select {
	case <-launchState.Done():
		println("123")
		err = errors.WithMessage(launchState.Err(), "chaincode registration failed")
	case err = <-startFailCh:
		println("456")
		launchState.Notify(err)
		r.Metrics.LaunchFailures.With("chaincode", cname).Add(1)
	case <-timeoutCh:
		println("789")
		err = errors.Errorf("timeout expired while starting chaincode %s for transaction", cname)
		launchState.Notify(err)
		r.Metrics.LaunchTimeouts.With("chaincode", cname).Add(1)
	}

	success := true
	if err != nil && !alreadyStarted {
		println("err != nil, falied to Launch")
		success = false
		chaincodeLogger.Debugf("stopping due to error while launching: %+v", err)
		defer r.Registry.Deregister(cname)
		if err := r.Runtime.Stop(ccci); err != nil {
			println("runtime stop error!")
			chaincodeLogger.Debugf("stop failed: %+v", err)
		}
	}

	r.Metrics.LaunchDuration.With(
		"chaincode", cname,
		"success", strconv.FormatBool(success),
	).Observe(time.Since(startTime).Seconds())

	chaincodeLogger.Debug("launch complete")
	println("launch achieved and err ?= nil")
	return err
}

func (r *RuntimeLauncher) getCodePackage(ccci *ccprovider.ChaincodeContainerInfo) ([]byte, error) {
	if ccci.ContainerType == inproccontroller.ContainerType {
		return nil, nil
	}

	codePackage, err := r.PackageProvider.GetChaincodeCodePackage(ccci.Name, ccci.Version)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get chaincode package")
	}

	return codePackage, nil
}
