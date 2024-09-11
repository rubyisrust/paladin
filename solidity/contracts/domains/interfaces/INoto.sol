// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.20;

interface INoto {
    event UTXOTransfer(
        bytes32[] inputs,
        bytes32[] outputs,
        bytes signature,
        bytes data
    );

    event UTXOApproved(address delegate, bytes32 txhash, bytes signature);

    function initialize(
        bytes32 transactionId,
        address domain,
        address notary,
        bytes memory config
    ) external;

    function transfer(
        bytes32[] memory inputs,
        bytes32[] memory outputs,
        bytes memory signature,
        bytes memory data
    ) external;

    function approve(
        address delegate,
        bytes32 txhash,
        bytes memory signature
    ) external;

    function approvedTransfer(
        bytes32[] memory inputs,
        bytes32[] memory outputs,
        bytes memory signature,
        bytes memory data
    ) external;
}
