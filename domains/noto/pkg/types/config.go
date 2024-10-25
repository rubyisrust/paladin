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
	"github.com/kaleido-io/paladin/toolkit/pkg/domain"
	"github.com/kaleido-io/paladin/toolkit/pkg/tktypes"
)

type DomainConfig struct {
	FactoryAddress string `json:"factoryAddress"`
}

var NotoConfigID_V0 = tktypes.MustParseHexBytes("0x00010000")

type NotoConfig_V0 struct {
	NotaryType    tktypes.HexUint64  `json:"notaryType"`
	NotaryAddress tktypes.EthAddress `json:"notaryAddress"`
	Variant       tktypes.HexUint64  `json:"variant"`
	Data          tktypes.HexBytes   `json:"data"`
	DecodedData   *NotoConfigData_V0 `json:"-"`
}

type NotoConfigData_V0 struct {
	NotaryLookup   string              `json:"notaryLookup"`
	PrivateAddress *tktypes.EthAddress `json:"privateAddress"`
	PrivateGroup   *PentePrivateGroup  `json:"privateGroup"`
}

// This is the structure we parse the config into in InitConfig and gets passed back to us on every call
type NotoParsedConfig struct {
	NotaryType     tktypes.HexUint64   `json:"notaryType"`
	NotaryAddress  tktypes.EthAddress  `json:"notaryAddress"`
	Variant        tktypes.HexUint64   `json:"variant"`
	NotaryLookup   string              `json:"notaryLookup"`
	PrivateAddress *tktypes.EthAddress `json:"privateAddress,omitempty"`
	PrivateGroup   *PentePrivateGroup  `json:"privateGroup,omitempty"`
}

type PentePrivateGroup struct {
	Salt    tktypes.Bytes32 `json:"salt"`
	Members []string        `json:"members"`
}

var NotoConfigABI_V0 = &abi.ParameterArray{
	{Name: "notaryType", Type: "uint64"},
	{Name: "notaryAddress", Type: "address"},
	{Name: "data", Type: "bytes"},
	{Name: "variant", Type: "uint64"},
}

var NotoTransactionData_V0 = tktypes.MustParseHexBytes("0x00010000")

type DomainHandler = domain.DomainHandler[NotoParsedConfig]
type ParsedTransaction = domain.ParsedTransaction[NotoParsedConfig]

var NotaryTypeSigner tktypes.HexUint64 = 0x0000
var NotaryTypeContract tktypes.HexUint64 = 0x0001

var NotoVariantDefault tktypes.HexUint64 = 0x0000
var NotoVariantSelfSubmit tktypes.HexUint64 = 0x0001
