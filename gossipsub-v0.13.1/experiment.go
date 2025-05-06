package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"log/slog"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
)

func CalcID(msg []byte) string {
	return fmt.Sprintf("%d", binary.BigEndian.Uint64(msg))
}

const timeBetweenMessages = 12 * time.Second

type ExperimentParams struct {
	NodeCount           int
	NumberOfConnections int

	GossipSubParams pubsub.GossipSubParams

	PublisherIndex []int
	MessageSize    int
	WarmupCount    int
	PublishCount   int
}

type HostConnector interface {
	ConnectSome(ctx context.Context, h host.Host, nodeId int, count int)
	AfterConnect(ctx context.Context)
}

func RunExperiment(ctx context.Context, logger *log.Logger, h host.Host, nodeId int, connector HostConnector, params ExperimentParams) error {
	slogger := slog.New(slog.NewJSONHandler(logger.Writer(), nil))
	slogger.Info("PeerID", "id", h.ID(), "node_id", nodeId)

	// create a gossipsub node and subscribe to the topic
	psOpts := pubsubOptions(logger, params.GossipSubParams)
	ps, err := pubsub.NewGossipSub(ctx, h, psOpts...)
	if err != nil {
		return err
	}

	const topicName = "some-topic"
	topic, err := ps.Join(topicName)
	if err != nil {
		return err
	}
	sub, err := topic.Subscribe()
	if err != nil {
		return err
	}

	// wait 30 seconds for other nodes to bootstrap
	time.Sleep(30 * time.Second)

	// discover peers
	connector.ConnectSome(ctx, h, nodeId, params.NumberOfConnections)
	connector.AfterConnect(ctx)

	logger.Printf("Connected to %d peers\n", len(h.Network().Peers()))

	// wait until 00:02 for the meshes to be formed and so that the publish will be exactly at 00:02
	t := time.Date(2000, time.January, 1, 0, 2, 0, 0, time.UTC)
	time.Sleep(time.Until(t))

	var msgId int
	publishNextMessage := func() error {
		defer func() { msgId++ }()
		if len(params.PublisherIndex) < msgId {
			fmt.Println("No more messages to publish", msgId, len(params.PublisherIndex))
			return nil
		}
		publisherIndexForMsg := params.PublisherIndex[msgId]
		if publisherIndexForMsg != nodeId {
			fmt.Printf("%d Skipping message %d, publisherIndexForMsg: %d\n", nodeId, msgId, publisherIndexForMsg)
			return nil
		}
		fmt.Printf("%d Publishing message %d\n", nodeId, msgId)
		msg := make([]byte, params.MessageSize)
		// rand.Read(msg)
		binary.BigEndian.PutUint64(msg, uint64(msgId))
		return topic.Publish(ctx, msg)
	}

	for range params.WarmupCount + params.PublishCount {
		publishNextMessage()
		fmt.Printf("%d Waiting for message %d\n", nodeId, msgId-1)
		_, err := sub.Next(ctx)
		fmt.Printf("%d Received message %d\n", nodeId, msgId-1)
		if err != nil {
			return err
		}
		t = t.Add(timeBetweenMessages)
		time.Sleep(time.Until(t))
	}

	return nil
}
