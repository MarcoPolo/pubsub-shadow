package main

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	topicName = "topic"
)

var (
	nodeCountFlag   = flag.Int("nodeCount", 100, "the number of nodes in the network")
	targetConnsFlag = flag.Int("targetConns", 70, "the target number of connected peers")
	publishStrategy = flag.String("publishStrategy", "", "publish strategy")
)

// pubsubOptions creates a list of options to configure our router with.
func pubsubOptions(logger *log.Logger) []pubsub.Option {
	psOpts := []pubsub.Option{
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign),
		pubsub.WithNoAuthor(),
		pubsub.WithMessageIdFn(func(pmsg *pubsubpb.Message) string {
			return CalcID(pmsg.Data)
		}),
		pubsub.WithPeerOutboundQueueSize(600),
		pubsub.WithMaxMessageSize(10 * 1 << 20),
		pubsub.WithValidateQueueSize(600),
		pubsub.WithRawTracer(gossipTracer{logger: logger}),
		pubsub.WithEventTracer(eventTracer{logger: logger}),
	}

	return psOpts
}

// compute a private key for node id
func nodePrivKey(id int) crypto.PrivKey {
	seed := make([]byte, ed25519.SeedSize)
	binary.LittleEndian.PutUint64(seed[:8], uint64(id))
	data := ed25519.NewKeyFromSeed(seed)

	privkey, err := crypto.UnmarshalEd25519PrivateKey(data)
	if err != nil {
		panic(err)
	}
	return privkey
}

const blobSize = 128 << 10
const subnetCount = 128
const columnCount = 128
const blobCount = 32
const samplingRequirement = 8

func main() {
	flag.Parse()
	ctx := context.Background()

	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	// parse for the node id
	var nodeId int
	if _, err := fmt.Sscanf(hostname, "node%d", &nodeId); err != nil {
		panic(err)
	}

	// listen for incoming connections
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/9000"),
		libp2p.Identity(nodePrivKey(nodeId)),
	)
	if err != nil {
		panic(err)
	}

	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)

	RunExperiment(ctx, logger, h, nodeId, ShadowConnector{}, ExperimentParams{
		PublishStrategy:     *publishStrategy,
		ColumnCount:         columnCount,
		SubnetCount:         columnCount,
		SamplingRequirement: samplingRequirement,
		NumberOfConnections: *targetConnsFlag,
		BlobSize:            blobSize,
		BlobCount:           blobCount,
	})
}

type ShadowConnector struct{}

func (c ShadowConnector) ConnectSome(ctx context.Context, h host.Host, nodeId int, count int) {
	peers := make(map[int]struct{})
	for len(h.Network().Peers()) < count {
		// do node discovery by picking the node randomly
		id := rand.Intn(*nodeCountFlag)
		if _, ok := peers[id]; ok || id == nodeId {
			continue
		}

		// resolve for ip addresses of the discovered node
		addrs, err := net.LookupHost(fmt.Sprintf("node%d", id))
		if err != nil || len(addrs) == 0 {
			log.Printf("Failed resolving for the address of node%d: %v\n", id, err)
			continue
		}

		// craft an addr info to be used to connect
		peerId, err := peer.IDFromPrivateKey(nodePrivKey(id))
		if err != nil {
			panic(err)
		}
		addr := fmt.Sprintf("/ip4/%s/tcp/9000/p2p/%s", addrs[0], peerId)
		info, err := peer.AddrInfoFromString(addr)
		if err != nil {
			panic(err)
		}

		// connect to the peer
		if err = h.Connect(ctx, *info); err != nil {
			log.Printf("Failed connecting to node%d: %v\n", id, err)
			continue
		}
		peers[id] = struct{}{}
		log.Printf("Connected to node%d: %s\n", id, addr)
	}
}

func subnetsForPeer(subnetCount int, maxSubnets int) []int {
	if subnetCount > maxSubnets {
		subnetCount = maxSubnets
	}
	subnets := make([]int, maxSubnets)
	for i := range maxSubnets {
		subnets[i] = i
	}
	if subnetCount == maxSubnets {
		// No point in shuffling
		return subnets
	}
	rand.Shuffle(len(subnets), func(i, j int) {
		subnets[i], subnets[j] = subnets[j], subnets[i]
	})
	return subnets[:subnetCount]
}
