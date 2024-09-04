/*
 * Copyright © 2024 Kaleido, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package types

import (
	"github.com/hyperledger/firefly-signer/pkg/abi"
	"github.com/kaleido-io/paladin/domains/common/pkg/domain"
)

type Config struct {
	FactoryAddress string `json:"factoryAddress"`
	Variant        string `json:"variant"`
}

type DomainConfig struct {
	NotaryLookup  string `json:"notaryLookup"`
	NotaryAddress string `json:"notaryAddress"`
}

var DomainConfigABI = &abi.ParameterArray{
	{Name: "notaryLookup", Type: "string"},
	{Name: "notaryAddress", Type: "address"},
}

type DomainHandler = domain.DomainHandler[DomainConfig]
type ParsedTransaction = domain.ParsedTransaction[DomainConfig]
