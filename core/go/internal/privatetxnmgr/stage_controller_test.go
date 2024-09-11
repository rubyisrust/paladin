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
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/kaleido-io/paladin/core/internal/privatetxnmgr/ptmgrtypes"
	"github.com/kaleido-io/paladin/core/internal/transactionstore"
	"github.com/kaleido-io/paladin/toolkit/pkg/confutil"
	"github.com/stretchr/testify/assert"
)

const testStage = "test"

type testActionOutput struct {
	Message string
}

type testStageProcessor struct {
}

func (tsp *testStageProcessor) Name() string {
	return "test"
}

func (tsp *testStageProcessor) GetIncompletePreReqTxIDs(ctx context.Context, tsg transactionstore.TxStateGetters, sfs ptmgrtypes.StageFoundationService) *ptmgrtypes.TxProcessPreReq {
	return nil
}

func (tsp *testStageProcessor) matchStage(ctx context.Context, tsg transactionstore.TxStateGetters, sfs ptmgrtypes.StageFoundationService) bool {
	return true
}

func (tsp *testStageProcessor) ProcessEvents(ctx context.Context, tsg transactionstore.TxStateGetters, sfs ptmgrtypes.StageFoundationService, stageEvents []*ptmgrtypes.StageEvent) (unprocessedStageEvents []*ptmgrtypes.StageEvent, txUpdates *transactionstore.TransactionUpdate, nextStep ptmgrtypes.StageProcessNextStep) {
	unprocessedStageEvents = []*ptmgrtypes.StageEvent{}
	nextStep = ptmgrtypes.NextStepWait
	for _, se := range stageEvents {
		if string(se.Stage) == testStage {
			// pretend we processed it
			if se.Data.(*testActionOutput).Message == "complete" {
				nextStep = ptmgrtypes.NextStepNewStage
			} else {
				txUpdates = &transactionstore.TransactionUpdate{
					SequenceID: confutil.P(uuid.New()),
				}
			}
		} else {
			unprocessedStageEvents = append(unprocessedStageEvents, se)
		}
	}
	return
}
func (tsp *testStageProcessor) PerformAction(ctx context.Context, tsg transactionstore.TxStateGetters, sfs ptmgrtypes.StageFoundationService) (actionOutput interface{}, actionTriggerErr error) {
	if tsg.GetContractAddress(ctx) == "0x000000error" {
		return nil, errors.New("pop")
	} else if tsg.GetContractAddress(ctx) == "0x000complete" {
		return &testActionOutput{
			Message: "complete",
		}, nil
	} else {
		return &testActionOutput{
			Message: "continue",
		}, nil
	}
}

func newTestStageController(ctx context.Context) *PaladinStageController {
	sc := NewPaladinStageController(ctx, NewPaladinStageFoundationService(nil, nil, nil, nil), []txStageProcessor{&testStageProcessor{}}).(*PaladinStageController)
	return sc
}

func TestBasicStageController(t *testing.T) {
	ctx := context.Background()
	sc := newTestStageController(ctx)
	testTx := &transactionstore.TransactionWrapper{
		Transaction: transactionstore.Transaction{
			ID:       uuid.New(),
			Contract: "0x000complete",
		},
	}
	// TODO: replace dummy checks with real implementation
	// test function works with test processor
	s := sc.CalculateStage(ctx, testTx)
	assert.Equal(t, "test", string(s))

	output, err := sc.PerformActionForStage(ctx, testStage, testTx)
	assert.Equal(t, "complete", output.(*testActionOutput).Message)
	assert.Empty(t, err)

	events, txUpdates, completed := sc.ProcessEventsForStage(ctx, testStage, testTx, []*ptmgrtypes.StageEvent{
		{
			Stage: testStage,
			Data:  output,
		},
	})
	assert.Empty(t, events)
	assert.Empty(t, txUpdates)
	assert.Equal(t, ptmgrtypes.NextStepNewStage, completed)

	// panic when stage is unknown
	unknownStage := "unknown"

	assert.Panics(t, func() {
		_, _ = sc.PerformActionForStage(ctx, unknownStage, testTx)
	})

	assert.Panics(t, func() {
		_, _, _ = sc.ProcessEventsForStage(ctx, unknownStage, testTx, []*ptmgrtypes.StageEvent{})
	})
}
