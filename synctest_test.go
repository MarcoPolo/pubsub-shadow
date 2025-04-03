//go:build goexperiment.synctest

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/simconn"
	simlibp2p "github.com/libp2p/go-libp2p/p2p/net/simconn/libp2p"
	"github.com/libp2p/go-libp2p/p2p/transport/quicreuse"
	"github.com/stretchr/testify/require"
)

func TestGossipSubPublishInOrder(t *testing.T) {
	testGossipSubPublishStrategy(t, "inOrder")
}

func TestGossipSubPublishRarestFirst(t *testing.T) {
	testGossipSubPublishStrategy(t, "rarestFirst")
}

func TestGossipSubPublishShuffle(t *testing.T) {
	testGossipSubPublishStrategy(t, "shuffle")
}

func testGossipSubPublishStrategy(t *testing.T, publishStrategy string) {
	synctest.Run(func() {
		const nodeCount = 256
		const numberOfConnections = 64
		const qlogDir = ""

		const latency = 50 * time.Millisecond
		const bandwidth = 20 * simlibp2p.OneMbps

		publisherBW := 100 * simlibp2p.OneMbps
		publisherLatency := 5 * time.Millisecond
		publisherSettings := simconn.NodeBiDiLinkSettings{
			Downlink: simconn.LinkSettings{BitsPerSecond: publisherBW, Latency: publisherLatency / 2},
			Uplink:   simconn.LinkSettings{BitsPerSecond: publisherBW, Latency: publisherLatency},
		}

		network, meta, err := simlibp2p.SimpleLibp2pNetwork([]simlibp2p.NodeLinkSettingsAndCount{
			// First node will be the publisher
			{LinkSettings: publisherSettings, Count: 1},
			{LinkSettings: simconn.NodeBiDiLinkSettings{
				Downlink: simconn.LinkSettings{BitsPerSecond: bandwidth, Latency: latency / 2}, // Divide by two since this is latency for each direction
				Uplink:   simconn.LinkSettings{BitsPerSecond: bandwidth, Latency: latency / 2},
			}, Count: nodeCount - 1},
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

		folder := fmt.Sprintf("synctest-%d-blobs-%d-%s.data", blobCount, nodeCount, publishStrategy)
		err = os.MkdirAll(folder, 0755)
		require.NoError(t, err)

		connector := newSimNetConnector(t, meta.Nodes, 16)

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
				RunExperiment(ctx, logger, node, nodeIdx, connector, ExperimentParams{
					BlobSize:            blobSize,
					BlobCount:           blobCount,
					ColumnCount:         columnCount,
					SubnetCount:         columnCount,
					SamplingRequirement: samplingRequirement,
					PublishStrategy:     publishStrategy,
					NumberOfConnections: numberOfConnections,
					OnFinishPublishing: func() bool {
						// wait for 30 seconds before stopping the publisher
						time.Sleep(30 * time.Second)
						return true
					},
				})
				if nodeIdx == 0 {
					cancel()
				}
			}(nodeIdx, node)
		}
		wg.Wait()
	})
}

type SimNetConnector struct {
	t              *testing.T
	sem            chan struct{}
	allNodes       []host.Host
	connectedNodes atomic.Int64
}

func newSimNetConnector(t *testing.T, allNodes []host.Host, connectorConcurrency int) *SimNetConnector {
	return &SimNetConnector{
		t:        t,
		sem:      make(chan struct{}, connectorConcurrency),
		allNodes: allNodes,
	}
}

func (c *SimNetConnector) ConnectSome(ctx context.Context, h host.Host, nodeIdx int, count int) {
	defer func() {
		x := c.connectedNodes.Add(1)
		c.t.Logf("connected %d out of %d nodes", x, len(c.allNodes))
	}()

	c.sem <- struct{}{}
	defer func() { <-c.sem }()
	for j := 0; j < count; j++ {
		n := rand.Intn(len(c.allNodes))
		if n == nodeIdx {
			j--
			continue
		}

		b := c.allNodes[n]
		err := h.Connect(ctx, peer.AddrInfo{ID: b.ID(), Addrs: b.Addrs()})
		if err != nil {
			c.t.Errorf("error connecting to node %d: %s", n, err)
		}
	}

}
