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

package privatetxnmgr

import (
	"context"

	"github.com/google/uuid"
	"github.com/kaleido-io/paladin/core/internal/privatetxnmgr/ptmgrtypes"
	"github.com/kaleido-io/paladin/core/internal/transactionstore"
	pb "github.com/kaleido-io/paladin/core/pkg/proto/sequence"
	"github.com/kaleido-io/paladin/toolkit/pkg/prototk"
)

type IdentityResolver interface {
	IsCurrentNode(nodeID string) bool
	ConnectToBaseLeger() error // TODO: does this function connects to the base ledger of current node/any available node as well? How about events?
	GetDispatchAddress(preferredAddresses []string) string
}
type DependencyChecker interface {
	PreReqsMatchCondition(ctx context.Context, preReqTxIDs []string, conditionFunc func(tsg transactionstore.TxStateGetters) (preReqComplete bool)) (filteredPreReqTxIDs []string)
	GetPreReqDispatchAddresses(ctx context.Context, preReqTxIDs []string) (dispatchAddresses []string)
	RegisterPreReqTrigger(ctx context.Context, txID string, txPreReq *ptmgrtypes.TxProcessPreReq)
}
type Sequencer interface {
	/*
		HandleTransactionAssembledEvent needs to be called whenever a transaction has been assembled by any node in the network, including the local node.
	*/
	HandleTransactionAssembledEvent(ctx context.Context, event *pb.TransactionAssembledEvent) error

	/*
		HandleTransactionEndorsedEvent needs to be called whenever a the endorsement rules for the given domain have been satisfied for a given transaction.
	*/
	HandleTransactionEndorsedEvent(ctx context.Context, event *pb.TransactionEndorsedEvent) error

	/*
		HandleTransactionConfirmedEvent needs to be called whenever a transaction has been confirmed on the base ledger
		i.e. it has been included in a block with enough subsequent blocks to consider this final for that particular chain.
	*/
	HandleTransactionConfirmedEvent(ctx context.Context, event *pb.TransactionConfirmedEvent) error

	/*
		OnTransationReverted needs to be called whenever a transaction has been rejected by any of the validation
		steps on any nodes or the base leddger contract. The transaction may or may not be reassembled after this
		hanlder is called.
	*/
	HandleTransactionRevertedEvent(ctx context.Context, event *pb.TransactionRevertedEvent) error

	/*
		HandleTransactionDelegatedEvent needs to be called whenever a transaction has been delegated from one node to another
		this is an event that is broadcast to all nodes after the fact and should not be confused with the DelegateTransaction message which is
		an instruction to the delegate node.
	*/
	HandleTransactionDelegatedEvent(ctx context.Context, event *pb.TransactionDelegatedEvent) error

	/*
		AssignTransaction is an instruction for the given transaction to be managed by this sequencer
	*/
	AssignTransaction(ctx context.Context, transactionID string) error

	/*
		ApproveEndorsement is a synchronous check of whether a given transaction could be endorsed by the local node. It asks the question:
		"given the information available to the local node at this point in time, does it appear that this transaction has no contention on input states".
	*/
	ApproveEndorsement(ctx context.Context, endorsementRequest ptmgrtypes.EndorsementRequest) (bool, error)
}
type Publisher interface {
	//Service for sending messages and events within the local node and as a client to the transport manager to send to other nodes
	PublishEvent(ctx context.Context, eventPayload interface{}) error
	PublishStageEvent(ctx context.Context, stageEvent *ptmgrtypes.StageEvent) error
}

type Dispatcher interface {
	// Dispatcher is the component that takes responsibility for submitting the transactions in the sequence to the base ledger in the correct order
	// most likely will be replaced with (or become an integration to) either the comms bus or some utility of the StageController framework
	Dispatch(context.Context, []uuid.UUID) error
}

type Delegator interface {
	// Delegator is the component that takes responsibility for delegating transactions to other nodes
	Delegate(ctx context.Context, transactionId string, delegateNodeId string) error
}

type EndorsementGatherer interface {
	//integrate with local signer and domain manager to satisfy the given endorsement request
	// that may have came from a transaction assembled locally or from another node
	GatherEndorsement(ctx context.Context, transactionSpecification *prototk.TransactionSpecification, verifiers []*prototk.ResolvedVerifier, signatures []*prototk.AttestationResult, inputStates []*prototk.EndorsableState, outputStates []*prototk.EndorsableState, partyName string, endorsementRequest *prototk.AttestationRequest) (*prototk.AttestationResult, *string, error)
}
