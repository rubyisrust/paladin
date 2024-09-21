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

package publictxmgr

import (
	"context"
	"time"

	"github.com/kaleido-io/paladin/core/internal/cache"
	"github.com/kaleido-io/paladin/core/internal/components"
	"github.com/kaleido-io/paladin/toolkit/pkg/confutil"
	"github.com/kaleido-io/paladin/toolkit/pkg/log"
	"github.com/kaleido-io/paladin/toolkit/pkg/ptxapi"
	"github.com/kaleido-io/paladin/toolkit/pkg/retry"
)

const (
	defaultTransactionEngineMaxInFlightOrchestrators = 50
	defaultTransactionEngineInterval                 = "5s"
	defaultTransactionEngineMaxStale                 = "1m"
	defaultTransactionEngineMaxIdle                  = "10s"
	defaultMaxOverloadProcessTime                    = "10m"
	defaultTransactionEngineRetryInitDelay           = "250ms"
	defaultTransactionEngineRetryMaxDelay            = "30s"
	defaultTransactionEngineRetryFactor              = 2.0
)

type TransactionEngineConfig struct {
	MaxInFlightOrchestrators *int         `yaml:"maxInFlightOrchestrators"`
	Interval                 *string      `yaml:"interval"`
	MaxStaleTime             *string      `yaml:"maxStaleTime"`
	MaxIdleTime              *string      `yaml:"maxIdleTime"`
	MaxOverloadProcessTime   *string      `yaml:"maxOverloadProcessTime"`
	TransactionCache         cache.Config `yaml:"transactionCache"` // can be larger than number of orchestrators for hot swapping
	Retry                    retry.Config `yaml:"retry"`
}

var DefaultTransactionEngineConfig = &TransactionEngineConfig{
	MaxInFlightOrchestrators: confutil.P(50),
	Interval:                 confutil.P("5s"),
	MaxStaleTime:             confutil.P("1m"),
	MaxIdleTim:               confutil.P("10s"),
	Retry: retry.Config{
		InitialDelay: confutil.P("250ms"),
		MaxDelay:     confutil.P("30s"),
		Factor:       confutil.P(2.0),
	},
	TransactionCache: cache.Config{
		Capacity: confutil.P(1000),
	},
}

// role of transaction engine:
// 1. owner and the only manipulator of the transaction orchestrators pool
//    - decides how many transaction orchestrators can there be in total at a given time
//    - decides the size of each transaction orchestrator queue
//    - decides signing address of each transaction orchestrator
//    - controls the life cycle for transaction orchestrators based on its latest status
//       - time box transaction orchestrator lifespan
//       - creates transaction orchestrator when necessary
//       - deletes transaction orchestrator when there is no pending transaction for a signing address
//       - pauses (delete) transaction orchestrator when it's in a stuck state
//       - resumes (re-create) transaction orchestrator in a resuming mode (1 transaction to test the water before add more) based on user configuration
// 2. handle external requests of transaction status change
//       - update/delete of a transaction when its signing address not in-flight
//       - signal update/delete of a transaction in-flight to corresponding transaction orchestrator
// 3. auto fueling management - the autofueling transactions
//    - creating auto-fueling transactions when asked by transaction orchestrators
// 4. provides shared functionalities for optimization
//    - handles gas price information which is not signer specific

func (ble *pubTxManager) engineLoop() {
	defer close(ble.engineLoopDone)
	ctx := log.WithLogField(ble.ctx, "role", "engine-loop")
	log.L(ctx).Infof("Engine started polling on interval %s", ble.enginePollingInterval)

	ticker := time.NewTicker(ble.enginePollingInterval)
	for {
		// Wait to be notified, or timeout to run
		select {
		case <-ticker.C:
		case <-ble.InFlightOrchestratorStale:
		case <-ctx.Done():
			ticker.Stop()
			log.L(ctx).Infof("Engine poller exiting")
			return
		}

		polled, total := ble.poll(ctx)
		log.L(ctx).Debugf("Engine polling complete: %d transaction orchestrators were created, there are %d transaction orchestrators in flight", polled, total)
	}
}

func (ble *pubTxManager) poll(ctx context.Context) (polled int, total int) {
	pollStart := time.Now()
	ble.InFlightOrchestratorMux.Lock()
	defer ble.InFlightOrchestratorMux.Unlock()

	oldInFlight := ble.InFlightOrchestrators
	ble.InFlightOrchestrators = make(map[string]*orchestrator)

	InFlightSigningAddresses := make([]string, 0, len(oldInFlight))

	stateCounts := make(map[string]int)
	for _, sName := range AllOrchestratorStates {
		// map for saving number of in flight transaction per stage
		stateCounts[sName] = 0
	}

	// Run through copying across from the old InFlight list to the new one, those that aren't ready to be deleted
	for signingAddress, oc := range oldInFlight {
		log.L(ctx).Debugf("Engine checking orchestrator for %s: state: %s, state duration: %s, number of transactions: %d", oc.signingAddress, oc.state, time.Since(oc.stateEntryTime), len(oc.InFlightTxs))
		if (oc.state == OrchestratorStateStale && time.Since(oc.stateEntryTime) > ble.maxOrchestratorStale) ||
			(oc.state == OrchestratorStateIdle && time.Since(oc.stateEntryTime) > ble.maxOrchestratorIdle) {
			// tell transaction orchestrator to stop, there is a chance we later found new transaction for this address, but we got to make a call at some point
			// so it's here. The transaction orchestrator won't be removed immediately as the state update is async
			oc.Stop()
		}
		if oc.state != OrchestratorStateStopped {
			ble.InFlightOrchestrators[signingAddress] = oc
			oc.MarkInFlightTxStale()
			stateCounts[string(oc.state)] = stateCounts[string(oc.state)] + 1
			InFlightSigningAddresses = append(InFlightSigningAddresses, signingAddress)
		} else {
			log.L(ctx).Infof("Engine removed orchestrator for signing address %s", signingAddress)
		}
	}

	totalBeforePoll := len(ble.InFlightOrchestrators)
	// check and poll new signers from the persistence if there are more transaction orchestrators slots
	spaces := ble.maxInFlightOrchestrators - totalBeforePoll
	if spaces > 0 {

		// Run through the paused orchestrators for fairness control
		for signingAddress, pausedUntil := range ble.SigningAddressesPausedUntil {
			if time.Now().Before(pausedUntil) {
				log.L(ctx).Debugf("Engine excluded orchestrator for signing address %s from polling as it's paused util %s", signingAddress, pausedUntil.String())
				stateCounts[string(OrchestratorStatePaused)] = stateCounts[string(OrchestratorStatePaused)] + 1
				InFlightSigningAddresses = append(InFlightSigningAddresses, signingAddress)
			}
		}

		var additionalTxFromNonInFlightSigners []*ptxapi.PublicTx
		// We retry the get from persistence indefinitely (until the context cancels)
		err := ble.retry.Do(ctx, "get pending transactions with non InFlight signing addresses", func(attempt int) (retry bool, err error) {
			tf := &components.PubTransactionQueries{
				InStatus: []string{string(PubTxStatusPending)},
				Sort:     confutil.P("sequence"),
				Limit:    &spaces,
			}
			if len(InFlightSigningAddresses) > 0 {
				tf.NotFrom = InFlightSigningAddresses
			}
			additionalTxFromNonInFlightSigners, err = ble.txStore.ListTransactions(ctx, tf)
			return true, err
		})
		if err != nil {
			log.L(ctx).Infof("Engine polling context cancelled while retrying")
			return -1, len(ble.InFlightOrchestrators)
		}

		log.L(ctx).Debugf("Engine polled %d items to fill in %d empty slots.", len(additionalTxFromNonInFlightSigners), spaces)

		for _, mtx := range additionalTxFromNonInFlightSigners {
			if _, exist := ble.InFlightOrchestrators[string(mtx.From)]; !exist {
				oc := NewOrchestrator(ble, string(mtx.From), ble.orchestratorConfig)
				ble.InFlightOrchestrators[string(mtx.From)] = oc
				stateCounts[string(oc.state)] = stateCounts[string(oc.state)] + 1
				_, _ = oc.Start(ble.ctx)
				log.L(ctx).Infof("Engine added orchestrator for signing address %s", mtx.From)
			} else {
				log.L(ctx).Warnf("Engine fetched extra transactions from signing address %s", mtx.From)
			}
		}
		total = len(ble.InFlightOrchestrators)
		if total > 0 {
			polled = total - totalBeforePoll
		}
	} else {
		// the in-flight orchestrator pool is full, do the fairness control

		// TODO: don't stop more than required number of slots

		// Run through the existing running orchestrators and stop the ones that exceeded the max process timeout
		for signingAddress, oc := range ble.InFlightOrchestrators {
			if time.Since(oc.orchestratorBirthTime) > ble.maxOverloadProcessTime {
				log.L(ctx).Infof("Engine pause, attempt to stop orchestrator for signing address %s", signingAddress)
				oc.Stop()
				ble.SigningAddressesPausedUntil[signingAddress] = time.Now().Add(ble.maxOverloadProcessTime)
			}
		}
	}
	ble.thMetrics.RecordInFlightOrchestratorPoolMetrics(ctx, stateCounts, ble.maxInFlightOrchestrators-len(ble.InFlightOrchestrators))
	log.L(ctx).Debugf("Engine poll loop took %s", time.Since(pollStart))
	return polled, total
}

func (ble *pubTxManager) MarkInFlightOrchestratorsStale() {
	// try to send an item in `InFlightStale` channel, which has a buffer of 1
	// to trigger a polling event to update the in flight transaction orchestrators
	// if it already has an item in the channel, this function does nothing
	select {
	case ble.InFlightOrchestratorStale <- true:
	default:
	}
}

func (ble *pubTxManager) GetPendingFuelingTransaction(ctx context.Context, sourceAddress string, destinationAddress string) (tx *ptxapi.PublicTx, err error) {
	tf := &components.PubTransactionQueries{
		InStatus:   []string{string(PubTxStatusPending)},
		To:         confutil.P(destinationAddress),
		From:       confutil.P(sourceAddress),
		Sort:       confutil.P("-nonce"),
		Limit:      confutil.P(1),
		HasTxValue: true, // NB: we assume if a transaction has value then it's a fueling transaction
	}

	txs, err := ble.txStore.ListTransactions(ctx, tf)
	if err != nil {
		return nil, err
	}
	if len(txs) > 0 {
		tx = txs[0]
	}
	return tx, nil
}

func (ble *pubTxManager) CheckTransactionCompleted(ctx context.Context, tx *ptxapi.PublicTx) (completed bool) {
	// no need for locking here as outdated information is OK given we do frequent retires
	log.L(ctx).Debugf("CheckTransactionCompleted checking state for transaction %s.", tx.ID)
	completedTxNonce, exists := ble.completedTxNoncePerAddress[string(tx.From)]
	if !exists {
		// need to query the database to check the status of managed transaction
		tf := &components.PubTransactionQueries{
			InStatus: []string{string(PubTxStatusSucceeded), string(PubTxStatusFailed)},
			From:     confutil.P(string(tx.From)),
			Sort:     confutil.P("-nonce"),
			Limit:    confutil.P(1),
		}

		txs, err := ble.txStore.ListTransactions(ctx, tf)
		if err != nil {
			// can not read from the database, treat transaction as incomplete
			return false
		}
		if len(txs) > 0 {
			ble.updateCompletedTxNonce(txs[0])
			completedTxNonce = *txs[0].Nonce.BigInt()
			// found completed fueling transaction, do the comparison
			completed = completedTxNonce.Cmp(tx.Nonce.BigInt()) >= 0
		}
		// if no completed fueling transaction is found, the value of "completed" stays false (still pending)
	} else {
		// in memory tracked highest nonce available, do the comparison
		completed = completedTxNonce.Cmp(tx.Nonce.BigInt()) >= 0
	}
	log.L(ctx).Debugf("CheckTransactionCompleted checking against completed nonce of %s, from: %s, to: %s, value: %s. Completed nonce: %d, current tx nonce: %d.", tx.ID, tx.From, tx.To, tx.Value.String(), completedTxNonce.Uint64(), tx.Nonce.Uint64())

	return completed

}

func (ble *pubTxManager) updateCompletedTxNonce(tx *ptxapi.PublicTx) (updated bool) {
	updated = false
	// no need for locking here as outdated information is OK given we do frequent retires
	ble.completedTxNoncePerAddressMutex.Lock()
	defer ble.completedTxNoncePerAddressMutex.Unlock()
	if tx.Status != PubTxStatusSucceeded && tx.Status != PubTxStatusFailed {
		// not a completed tx, no op
		return updated
	}
	currentNonce, exists := ble.completedTxNoncePerAddress[string(tx.From)]
	if !exists || currentNonce.Cmp(tx.Nonce.BigInt()) == -1 {
		ble.completedTxNoncePerAddress[string(tx.From)] = *tx.Nonce.BigInt()
		updated = true
	}
	return updated
}
