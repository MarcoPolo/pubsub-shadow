# pubsub-shadow

## Requirements

- Go 1.24 for synctest experiments
- [Shadow](https://shadow.github.io/) for shadow experiments.
- [uv](https://docs.astral.sh/uv/) for python deps (or just dependencies
  available on your machine).

## Simulations

This repo contains two ways to run this experiment in a simulation.

1. A new synctest based simulator using the new simulated networks from
   go-libp2p: https://github.com/libp2p/go-libp2p/pull/3262.
2. Shadow, a more generic and capable simulator.

The synctest based simulator is generally faster than Shadow and can run
anywhere Go runs. Shadow is crafted specifically for network simulations, is
application agnostic, has more network topology knobs, and is more mature.

My recommendation:

1. Use the synctest based simulator to quickly iterate on the experiment.
2. Use Shadow for the actual data you want to publish.
3. Keep the logic of the experiment in `experiment.go` to let both simulators
   run the same code.

The synctest and Shadow should agree at a very high level, but details in
network topology may cause the results to look different. Specifically
go-libp2p's simnet does not yet support the Shadow's graph configuration for
simulating latencies between networks (e.g. latency from europe to america).
Synctest only supports latencies at the node level, where each node defines its
uplink/downlink latencies. 

### synctest based simulator

Modify the experiment parameters in `synctest_test.go`, then run tests with
`GOEXPERIMENT=synctest` or `-tags goexperiment.synctest`.

For a single test:
```bash
 go test -tags goexperiment.synctest -v -run TestGossipSubPublishInOrder .
```

All Tests:
```bash
 go test -tags goexperiment.synctest -v .
```


## Shadow simulator


```bash
make run node_count=256 publish_strategy=inOrder target_conns=64
```


## Plotting data:

```bash
uv run analyse_logs.py <data output folder>
```

Example for synctest simulation:
```bash
uv run analyse_logs.py synctest-8-blobs-256-rarestFirst.data
```

Example for a shadow simulation:
```
uv run analyse_logs.py shadow-8-blobs-256-inOrder.data
```
