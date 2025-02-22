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

package groupmgr

import (
	"context"

	"github.com/google/uuid"
	"github.com/kaleido-io/paladin/core/internal/components"
	"github.com/kaleido-io/paladin/core/internal/filters"
	"github.com/kaleido-io/paladin/core/internal/msgs"
	"github.com/kaleido-io/paladin/core/pkg/persistence"
	"github.com/kaleido-io/paladin/toolkit/pkg/i18n"
	"github.com/kaleido-io/paladin/toolkit/pkg/pldapi"
	"github.com/kaleido-io/paladin/toolkit/pkg/query"
	"github.com/kaleido-io/paladin/toolkit/pkg/tktypes"
)

type persistedMessage struct {
	LocalSeq uint64            `gorm:"column:local_seq;autoIncrement;primaryKey"`
	Domain   string            `gorm:"column:domain"`
	Group    tktypes.HexBytes  `gorm:"column:group"`
	Node     string            `gorm:"column:node"`
	Sent     tktypes.Timestamp `gorm:"column:sent"`
	Received tktypes.Timestamp `gorm:"column:received"`
	ID       uuid.UUID         `gorm:"column:id"`
	CID      *uuid.UUID        `gorm:"column:cid"`
	Topic    string            `gorm:"column:topic"`
	Data     tktypes.RawJSON   `gorm:"column:data"`
}

func (persistedMessage) TableName() string {
	return "transaction_receipts"
}

var messageFilters = filters.FieldMap{
	"localSequence": filters.Int64Field("local_seq"),
	"domain":        filters.StringField("domain"),
	"group":         filters.HexBytesField("group"),
	"sent":          filters.TimestampField("sent"),
	"received":      filters.TimestampField("received"),
	"id":            filters.UUIDField("id"),
	"correlationId": filters.UUIDField("cid"),
	"topic":         filters.StringField("topic"),
}

func (gm *groupManager) SendMessage(ctx context.Context, dbTX persistence.DBTX, msg pldapi.PrivacyGroupMessageInput) (*uuid.UUID, error) {

	pg, err := gm.GetGroupByID(ctx, dbTX, msg.Domain, msg.Group)
	if err != nil {
		return nil, err
	}
	if pg == nil {
		return nil, i18n.NewError(ctx, msgs.MsgPGroupsGroupNotFound, msg.Group)
	}

	// Build and insert the message
	now := tktypes.TimestampNow()
	msgID := uuid.New()
	pMsg := &persistedMessage{
		Domain:   msg.Domain,
		Group:    msg.Group,
		Sent:     now,
		Received: now,
		Node:     gm.transportManager.LocalNodeName(),
		ID:       msgID,
		CID:      msg.CorrelationID,
		Topic:    msg.Topic,
		Data:     msg.Data,
	}
	if err := dbTX.DB().WithContext(ctx).Create(pMsg).Error; err != nil {
		return nil, err
	}

	// Create the reliable message delivery to the other parties
	remoteMembers, err := gm.validateMembers(ctx, pg.Members)
	if err != nil {
		return nil, err
	}

	// We also need to create a reliable send the state to all the remote members
	msgs := make([]*components.ReliableMessage, 0, len(remoteMembers))
	for node := range remoteMembers {
		// Each node gets a single copy (not one per identity)
		msgs = append(msgs, &components.ReliableMessage{
			Node:        node,
			MessageType: components.RMTPrivacyGroup.Enum(),
			Metadata: tktypes.JSONString(&components.PrivacyGroupMessageDistribution{
				Domain: msg.Domain,
				Group:  msg.Group,
				ID:     msgID,
			}),
		})
	}
	if len(msgs) > 0 {
		if err := gm.transportManager.SendReliable(ctx, dbTX, msgs...); err != nil {
			return nil, err
		}
	}

	dbTX.AddPostCommit(func(txCtx context.Context) {
		gm.notifyNewMessages([]*persistedMessage{pMsg})
	})

	return &msgID, nil

}

func (gm *groupManager) ReceiveMessages(ctx context.Context, dbTX persistence.DBTX, node string, msgs ...pldapi.PrivacyGroupMessage) error {

	// Build and insert the messages
	now := tktypes.TimestampNow()
	pMsgs := make([]*persistedMessage, len(msgs))
	for i, msg := range msgs {
		pMsgs[i] = &persistedMessage{
			Domain:   msg.Domain,
			Group:    msg.Group,
			Sent:     msg.Sent,
			Received: now,  // we're receiving
			Node:     node, // from this node
			ID:       msg.ID,
			CID:      msg.CorrelationID,
			Topic:    msg.Topic,
			Data:     msg.Data,
		}
	}
	if err := dbTX.DB().WithContext(ctx).Create(pMsgs).Error; err != nil {
		return err
	}

	dbTX.AddPostCommit(func(txCtx context.Context) {
		gm.notifyNewMessages(pMsgs)
	})

	return nil

}

func (gm *groupManager) QueryMessages(ctx context.Context, dbTX persistence.DBTX, jq *query.QueryJSON) ([]*pldapi.PrivacyGroupMessage, error) {
	qw := &filters.QueryWrapper[persistedMessage, pldapi.PrivacyGroupMessage]{
		P:           gm.p,
		DefaultSort: "-localSequence",
		Filters:     messageFilters,
		Query:       jq,
		MapResult: func(dbPM *persistedMessage) (*pldapi.PrivacyGroupMessage, error) {
			return dbPM.mapToAPI(), nil
		},
	}
	return qw.Run(ctx, dbTX)
}

func (gm *groupManager) GetMessagesByID(ctx context.Context, dbTX persistence.DBTX, ids []uuid.UUID, failNotFound bool) ([]*pldapi.PrivacyGroupMessage, error) {
	inIDs := make([]any, len(ids))
	for i, id := range ids {
		inIDs[i] = id
	}
	dbMsgs, err := gm.QueryMessages(ctx, dbTX, query.NewQueryBuilder().In("id", inIDs).Limit(len(ids)).Query())
	if err != nil {
		return nil, err
	}
	if failNotFound && len(dbMsgs) != len(ids) {
		return nil, i18n.NewError(ctx, msgs.MsgPGroupsMessageNotFound)
	}
	return dbMsgs, nil
}
