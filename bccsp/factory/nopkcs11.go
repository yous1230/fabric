// +build !pkcs11

/*
Copyright IBM Corp. All Rights Reserved.

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
package factory

import (
	"strconv"

	"github.com/hyperledger/fabric/bccsp"
	"github.com/pkg/errors"
	gcx "github.com/zhigui-projects/gm-crypto/x509"
)

const pkcs11Enabled = false

// FactoryOpts holds configuration information used to initialize factory implementations
type FactoryOpts struct {
	ProviderName string      `mapstructure:"default" json:"default" yaml:"Default"`
	SwOpts       *SwOpts     `mapstructure:"SW,omitempty" json:"SW,omitempty" yaml:"SwOpts"`
	PluginOpts   *PluginOpts `mapstructure:"PLUGIN,omitempty" json:"PLUGIN,omitempty" yaml:"PluginOpts"`
}

// InitFactories must be called before using factory interfaces
// It is acceptable to call with config = nil, in which case
// some defaults will get used
// Error is returned only if defaultBCCSP cannot be found
func InitFactories(config *FactoryOpts) error {
	factoriesInitOnce.Do(func() {
		factoriesInitError = initFactories(config)
	})

	return factoriesInitError
}

func initFactories(config *FactoryOpts) error {
	// Take some precautions on default opts
	if config == nil {
		config = GetDefaultOpts()
	}

	if config.ProviderName == "" {
		config.ProviderName = "SW"
	}

	if config.SwOpts == nil {
		config.SwOpts = GetDefaultOpts().SwOpts
	}

	// Initialize factories map
	bccspMap = make(map[string]bccsp.BCCSP)

	// Software-Based BCCSP
	if config.ProviderName == "SW" && config.SwOpts != nil {
		f := &SWFactory{}
		err := initBCCSP(f, config)
		if err != nil {
			return errors.Wrapf(err, "Failed initializing BCCSP")
		}
	}

	// BCCSP Plugin
	if config.ProviderName == "PLUGIN" && config.PluginOpts != nil {
		f := &PluginFactory{}
		err := initBCCSP(f, config)
		if err != nil {
			return errors.Wrapf(err, "Failed initializing PLUGIN.BCCSP")
		}
	}

	var ok bool
	defaultBCCSP, ok = bccspMap[config.ProviderName]
	if !ok {
		return errors.Errorf("Could not find default `%s` BCCSP", config.ProviderName)
	}
	gcx.InitX509(defaultAlgorithm)
	return nil
}

// GetBCCSPFromOpts returns a BCCSP created according to the options passed in input.
func GetBCCSPFromOpts(config *FactoryOpts) (bccsp.BCCSP, error) {
	var f BCCSPFactory
	switch config.ProviderName {
	case "SW":
		f = &SWFactory{}
	case "PLUGIN":
		f = &PluginFactory{}
	default:
		return nil, errors.Errorf("Could not find BCCSP, no '%s' provider", config.ProviderName)
	}

	csp, err := f.Get(config)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not initialize BCCSP %s", f.Name())
	}
	return csp, nil
}

func GetHashOptFromOpts(config *FactoryOpts) (string, bccsp.HashOpts, error) {
	switch config.ProviderName {
	case "SW":
		if opt, err := bccsp.GetHashOptFromFamily(config.SwOpts.SecLevel, config.SwOpts.HashFamily); err != nil {
			return "", nil, err
		} else {
			return config.SwOpts.HashFamily, opt, nil
		}
	case "PLUGIN":
		secLv := config.PluginOpts.Config["SecLevel"]
		if secLv == nil {
			return "", nil, errors.Errorf("bccsp plugin provider [%s] hash seclevel not set", config.ProviderName)
		}
		secLevel, err := strconv.Atoi(secLv.(string))
		if err != nil {
			return "", nil, err
		}
		hf := config.PluginOpts.Config["HashFamily"]
		if hf == nil {
			return "", nil, errors.Errorf("bccsp plugin provider [%s] hash family not set", config.ProviderName)
		}

		if opt, err := bccsp.GetHashOptFromFamily(secLevel, hf.(string)); err != nil {
			return "", nil, err
		} else {
			return hf.(string), opt, nil
		}
	default:
		return "", nil, errors.Errorf("Could not find HashOpt from opts, no '%s' provider", config.ProviderName)
	}
}
