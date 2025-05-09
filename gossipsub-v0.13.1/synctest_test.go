//go:build goexperiment.synctest

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/simconn"
	simlibp2p "github.com/libp2p/go-libp2p/p2p/net/simconn/libp2p"
	"github.com/libp2p/go-libp2p/p2p/transport/quicreuse"
	"github.com/stretchr/testify/require"
)

func TestGossipSub(t *testing.T) {
	const nodeCount = 700
	const numberOfConnections = 10
	r := rand.New(rand.NewChaCha8([32]byte{}))

	gossipSubParams := pubsub.DefaultGossipSubParams()

	expParams := ExperimentParams{
		NodeCount:           nodeCount,
		NumberOfConnections: numberOfConnections,
		GossipSubParams:     gossipSubParams,
		MessageSize:         (2 * 1024) * 48,
		PublishCount:        32,
	}
	expParams.PublisherIndex = make([]int, 0, expParams.PublishCount)
	for range expParams.PublishCount {
		expParams.PublisherIndex = append(expParams.PublisherIndex, r.IntN(nodeCount))
	}

	runGossipSubTest(t, "gossipsub", expParams)
}

func runGossipSubTest(t *testing.T, testName string, expParams ExperimentParams) {
	synctest.Run(func() {
		// qlogDir := fmt.Sprintf("/tmp/gossipsub-%d-%s", subnetCount, publishStrategy)
		qlogDir := ""

		const latency = 20 * time.Millisecond
		const bandwidth = 50 * simlibp2p.OneMbps

		network, meta, err := simlibp2p.SimpleLibp2pNetwork([]simlibp2p.NodeLinkSettingsAndCount{
			{LinkSettings: simconn.NodeBiDiLinkSettings{
				Downlink: simconn.LinkSettings{BitsPerSecond: bandwidth, Latency: latency / 2}, // Divide by two since this is latency for each direction
				Uplink:   simconn.LinkSettings{BitsPerSecond: bandwidth, Latency: latency / 2},
			}, Count: expParams.NodeCount},
		}, simlibp2p.NetworkSettings{
			UseBlankHost: true,
			QUICReuseOptsForHostIdx: func(idx int) []quicreuse.Option {
				if idx == 0 && qlogDir != "" {
					return []quicreuse.Option{
						quicreuse.WithQlogDir(qlogDir),
					}
				}
				return nil
			},
		})
		require.NoError(t, err)
		network.Start()
		defer network.Close()

		defer func() {
			for _, node := range meta.Nodes {
				node.Close()
			}
		}()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		folder := fmt.Sprintf("synctest-%s.data", testName)
		if _, err := os.Stat(folder); err == nil {
			os.RemoveAll(folder)
		}
		err = os.MkdirAll(folder, 0755)
		require.NoError(t, err)

		connector := newSimNetConnector(t, meta.Nodes, 2)

		var wg sync.WaitGroup
		for nodeIdx, node := range meta.Nodes {
			wg.Add(1)
			go func(nodeIdx int, node host.Host) {
				defer wg.Done()
				filename := fmt.Sprintf("%s/node%d.log", folder, nodeIdx)
				f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
				require.NoError(t, err)
				defer f.Close()
				logger := log.New(f, "", log.LstdFlags|log.Lmicroseconds)
				err = RunExperiment(ctx, logger, node, nodeIdx, connector, expParams)
				if err != nil {
					t.Errorf("error running experiment on node %d: %s", nodeIdx, err)
				}
			}(nodeIdx, node)
		}
		wg.Wait()
	})
}

type SimNetConnector struct {
	t               *testing.T
	sem             chan struct{}
	allNodes        []host.Host
	connectionsDone chan struct{}
	connectedNodes  atomic.Int64
}

func newSimNetConnector(t *testing.T, allNodes []host.Host, connectorConcurrency int) *SimNetConnector {
	return &SimNetConnector{
		t:               t,
		sem:             make(chan struct{}, connectorConcurrency),
		allNodes:        allNodes,
		connectionsDone: make(chan struct{}),
	}
}

func (c *SimNetConnector) ConnectSome(ctx context.Context, h host.Host, nodeIdx int, count int) {
	defer func() {
		x := c.connectedNodes.Add(1)
		if x == int64(len(c.allNodes)) {
			close(c.connectionsDone)
		}
		if x%100 == 0 {
			c.t.Logf("connected %d out of %d nodes", x, len(c.allNodes))
			c.t.Logf("peers: %v", len(h.Network().Peers()))
		}
	}()

	for len(h.Network().Peers()) < count {
		n := rand.IntN(len(c.allNodes))
		if n == nodeIdx {
			continue
		}

		b := c.allNodes[n]
		err := h.Connect(ctx, peer.AddrInfo{ID: b.ID(), Addrs: b.Addrs()})
		if err != nil {
			c.t.Logf("error connecting to node %d: %s", n, err)
		}
	}
}

func (c *SimNetConnector) AfterConnect(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-c.connectionsDone:
		return
	}
}
