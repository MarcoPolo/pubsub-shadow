import argparse
import random
import json

random.seed(1)

parser = argparse.ArgumentParser()
parser.add_argument("--node_count", type=int, required=True)
parser.add_argument("--output", type=str, required=True)
args = parser.parse_args()

experiment_params = {
    "GossipSubParams": {},
    "NumberOfConnections": 10,
    "NodeCount": args.node_count,
    "MessageSize": 2048 * 48,
    "WarmupCount": 16,
    "PublishCount": 16,
    "PublisherIndex": [],
}

for i in range(experiment_params["PublishCount"] + experiment_params["WarmupCount"]):
    experiment_params["PublisherIndex"].append(random.randint(0, args.node_count - 1))

with open(args.output, "w") as f:
    json.dump(experiment_params, f)
