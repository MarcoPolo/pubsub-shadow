package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
)

type ExperimentParams struct {
	D         int
	DAnnounce int
	// PublishStrategy is the strategy to use for publishing messages
	// inOrder: publish messages in order
	// rarestFirst: publish messages in rarest first order
	PublishStrategy           string
	NumberOfConnections       int
	ColumnCount               int
	ColumnSamplingRequirement int
	SubnetCount               int
	BlobSize                  int
	BlobCount                 int
	// OnFinishPublishing is called when the node has finished publishing messages
	// It returns true if the node should stop listening for messages and exit
	OnFinishPublishing func() bool
}

type HostConnector interface {
	ConnectSome(ctx context.Context, h host.Host, nodeId int, count int)
	AfterConnect(ctx context.Context)
}

func RunExperiment(ctx context.Context, logger *log.Logger, h host.Host, nodeId int, connector HostConnector, params ExperimentParams) {
	if params.D == 0 {
		panic("D must be set")
	}
	logger.Printf("Publish Strategy: %s\n", params.PublishStrategy)
	logger.Printf("NodeId: %d\n", nodeId)
	logger.Printf("PeerId: %s\n", h.ID())
	logger.Printf("Listening on: %v\n", h.Addrs())

	if params.BlobCount == 0 {
		panic("BlobCount must be set")
	}

	// create a gossipsub node and subscribe to the topic
	psOpts := pubsubOptions(logger, params.D, params.DAnnounce)
	ps, err := pubsub.NewGossipSub(ctx, h, psOpts...)
	if err != nil {
		panic(err)
	}

	var subnets []int
	if nodeId == 0 {
		subnets = subnetsForPeer(params.SubnetCount, params.SubnetCount)
	} else {
		subnetSampleRequirement := params.ColumnSamplingRequirement * params.SubnetCount / params.ColumnCount
		subnets = subnetsForPeer(subnetSampleRequirement, params.SubnetCount)
	}

	var topics []*pubsub.Topic
	var subs []*pubsub.Subscription
	for _, subnet := range subnets {
		topic, err := ps.Join(fmt.Sprintf("%s-%d", topicName, subnet))
		if err != nil {
			panic(err)
		}
		topics = append(topics, topic)
		sub, err := topic.Subscribe()
		if err != nil {
			panic(err)
		}
		subs = append(subs, sub)
	}

	// wait 30 seconds for other nodes to bootstrap
	time.Sleep(30 * time.Second)

	// discover peers
	connector.ConnectSome(ctx, h, nodeId, params.NumberOfConnections)
	connector.AfterConnect(ctx)

	logger.Printf("Connected to %d peers\n", len(h.Network().Peers()))

	// wait until 00:02 for the meshes to be formed and so that the publish will be exactly at 00:02
	time.Sleep(time.Until(time.Date(2000, time.January, 1, 0, 2, 0, 0, time.UTC)))

	// var messageBatch *pubsub.MessageBatch

	// if it's a turn for the node to publish, publish
	if nodeId == 0 {
		switch params.PublishStrategy {
		case "inOrder":
		// case "rarestFirst":
		// 	messageBatch, err = pubsub.NewMessageBatch(ps)
		// 	if err != nil {
		// 		panic(err)
		// 	}
		default:
			panic(fmt.Sprintf("Invalid publish strategy: %s", params.PublishStrategy))
		}

		var msgsToPublish [][]byte

		msgHeader := make([]byte, 32)
		for i := range msgHeader {
			msgHeader[i] = ' '
		}
		msgsToPublish = make([][]byte, 0, params.ColumnCount)
		for i := 0; i < params.ColumnCount; i++ {
			topic := topics[i%len(topics)]
			columnSize := 2 * params.BlobSize * params.BlobCount / params.ColumnCount
			msg := make([]byte, columnSize)
			rand.Read(msg) // it takes about a 50-100 us to fill the buffer on macpro 2019. Can be considered simulataneous
			copy(msgHeader, fmt.Sprintf("msg %d on %s.  ", i, topic.String()))
			copy(msg, msgHeader)
			msgsToPublish = append(msgsToPublish, msg)
		}

		for i := 0; i < params.ColumnCount; i++ {
			topic := topics[i%len(topics)]
			msg := msgsToPublish[i]
			// var err error
			// if messageBatch != nil {
			// 	err = messageBatch.Add(ctx, topic, msg)
			// } else {
			// 	err = topic.Publish(ctx, msg)
			// }

			err := topic.Publish(ctx, msg)
			if err != nil {
				logger.Printf("Failed to publish message by %s\n", h.ID())
			} else {
				logger.Printf("Published: (topic: %s, id: %s)\n", topic.String(), CalcID(msg))
			}
		}

		// if messageBatch != nil {
		// 	messageBatch.Publish()
		// }
	}

	if nodeId == 0 {
		if params.OnFinishPublishing != nil && params.OnFinishPublishing() {
			return
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(subs))
	for _, sub := range subs {
		go func(sub *pubsub.Subscription) {
			defer wg.Done()
			for {
				// block and wait to receive the next message
				m, err := sub.Next(ctx)
				if err == context.Canceled {
					return
				}
				if err != nil {
					panic(err)
				}
				logger.Printf("Received: (topic: %s, id: %s)\n", *m.Topic, CalcID(m.Message.Data))
			}
		}(sub)
	}
	wg.Wait()
}
