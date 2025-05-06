package main

import (
	"context"
	"log/slog"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

type gossipTracer struct {
	logger *slog.Logger
}

// Trace implements pubsub.EventTracer.
func (g *gossipTracer) Trace(evt *pubsub_pb.TraceEvent) {
	switch *evt.Type {
	case pubsub_pb.TraceEvent_DELIVER_MESSAGE:
		mid := string(evt.DeliverMessage.MessageID)
		from := peer.ID(evt.DeliverMessage.ReceivedFrom)
		g.logger.LogAttrs(context.Background(), slog.LevelInfo, "Deliver", slog.String("id", mid), slog.String("from", from.String()))
	}
}

var _ pubsub.EventTracer = &gossipTracer{}

// AddPeer .
func (g gossipTracer) AddPeer(p peer.ID, proto protocol.ID) {
	g.logger.LogAttrs(context.Background(), slog.LevelInfo, "Peer Added", slog.String("id", p.String()), slog.String("protocol", string(proto)))
}

// RemovePeer .
func (g gossipTracer) RemovePeer(p peer.ID) {
	g.logger.LogAttrs(context.Background(), slog.LevelInfo, "Peer Removed", slog.String("id", p.String()))
}

// Join .
func (g gossipTracer) Join(topic string) {
	g.logger.LogAttrs(context.Background(), slog.LevelInfo, "Joined", slog.String("topic", topic))
}

// Leave .
func (g gossipTracer) Leave(topic string) {
	g.logger.LogAttrs(context.Background(), slog.LevelInfo, "Left", slog.String("topic", topic))
}

// Graft .
func (g gossipTracer) Graft(p peer.ID, topic string) {
	g.logger.LogAttrs(context.Background(), slog.LevelInfo, "Grafted", slog.String("topic", topic), slog.String("peer", p.String()))
}

// Prune .
func (g gossipTracer) Prune(p peer.ID, topic string) {
	g.logger.LogAttrs(context.Background(), slog.LevelInfo, "Prune", slog.String("topic", topic), slog.String("peer", p.String()))
}

// ValidateMessage .
func (g gossipTracer) ValidateMessage(msg *pubsub.Message) {
	g.logger.LogAttrs(context.Background(), slog.LevelInfo, "Validate", slog.String("id", msg.ID), slog.String("from", msg.ReceivedFrom.String()))
}

// DeliverMessage .
func (g gossipTracer) DeliverMessage(msg *pubsub.Message) {
	// handled in the Trace function because this isn't called if we deliver our own message
	// g.logger.LogAttrs(context.Background(), slog.LevelInfo, "Deliver", slog.String("id", msg.ID), slog.String("from", msg.ReceivedFrom.String()))
}

// RejectMessage .
func (g gossipTracer) RejectMessage(msg *pubsub.Message, reason string) {
	g.logger.LogAttrs(context.Background(), slog.LevelInfo, "Reject", slog.String("id", msg.ID), slog.String("from", msg.ReceivedFrom.String()), slog.String("reason", reason))
}

// DuplicateMessage .
func (g gossipTracer) DuplicateMessage(msg *pubsub.Message) {
	g.logger.LogAttrs(context.Background(), slog.LevelInfo, "Duplicate", slog.String("id", msg.ID), slog.String("from", msg.ReceivedFrom.String()))
}

// UndeliverableMessage .
func (g gossipTracer) UndeliverableMessage(msg *pubsub.Message) {
	g.logger.LogAttrs(context.Background(), slog.LevelInfo, "Undeliverable", slog.String("id", msg.ID), slog.String("from", msg.ReceivedFrom.String()))
}

// ThrottlePeer .
func (g gossipTracer) ThrottlePeer(p peer.ID) {
	g.logger.LogAttrs(context.Background(), slog.LevelInfo, "Throttle", slog.String("peer", p.String()))
}

// RecvRPC .
func (g gossipTracer) RecvRPC(rpc *pubsub.RPC) {
}

// SendRPC .
func (g gossipTracer) SendRPC(rpc *pubsub.RPC, p peer.ID) {
}

// DropRPC .
func (g gossipTracer) DropRPC(rpc *pubsub.RPC, p peer.ID) {
}
