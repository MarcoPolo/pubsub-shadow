from dataclasses import dataclass
import argparse
import json
import os
import random
import subprocess
from typing import Dict, List, Union
from datetime import datetime, timezone, timedelta


from analyze_message_deliveries import analyse_message_deliveries
from network_graph import generate_graph

params_file_name = "params.json"

@dataclass
class Binary:
    path: str
    percent_of_nodes: int

@dataclass
class PublishAction:
    time: str # RFC 3339 encoded
    publisherIndex: int

ScriptAction = Union[PublishAction]

@dataclass
class ExperimentParams:
    gossipSubParams: dict
    numberOfConnections: int
    nodeCount: int
    messageSize: int
    publishCount: int
    script: List[ScriptAction]

@dataclass
class Experiment:
    binaries: List[Binary]
    experiment_params: ExperimentParams


def experiments(node_count: int) -> Dict[str, Experiment]:
    return {
        "gossipsub-v0.13.1-stock": Experiment(
            binaries=[Binary("gossipsub-v0.13.1/gossipsub-bin", percent_of_nodes=100)],
            experiment_params=ExperimentParams(
                gossipSubParams={},
                numberOfConnections=10,
                nodeCount=node_count,
                messageSize=2048 * 48,
                publishCount=32,
                script=[],
            ),
        ),
        "gossipsub-v0.13.1-stock-smaller-D": Experiment(
            binaries=[Binary("gossipsub-v0.13.1/gossipsub-bin", percent_of_nodes=100)],
            experiment_params=ExperimentParams(
                gossipSubParams={
                    "D": 4,
                    "Dlo": 1,
                    "Dhi": 6,
                    "Dscore": 1,
                    "Dout": 0,
                },
                numberOfConnections=10,
                nodeCount=node_count,
                messageSize=2048 * 48,
                publishCount=32,
                script=[],
            ),
        ),
    }

def script_random_publish_every_12s(experiment_params: ExperimentParams):
    start_time = datetime(2000, 1, 1, 0, 2, 0, tzinfo=timezone.utc)
    for _ in range(experiment_params.publishCount):
        start_time += timedelta(seconds=12)
        experiment_params.script.append(
            PublishAction(
                time=start_time.isoformat(),
                publisherIndex=random.randint(0, experiment_params.nodeCount - 1)
            )
        )


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--node_count", type=int, required=True)
    parser.add_argument("--seed", type=int, required=False)
    parser.add_argument("--experiment", type=str, required=True)
    parser.add_argument("--output_dir", type=str, required=False)
    args = parser.parse_args()

    if args.output_dir is None:
        args.output_dir = f"{args.experiment}.data"

    if args.seed is None:
        args.seed = 1

    random.seed(args.seed)

    exps = experiments(args.node_count)
    exp = exps[args.experiment]

    # Script random publishes 12s apart
    script_random_publish_every_12s(exp.experiment_params)

    with open(params_file_name, "w") as f:
        json.dump(exp.experiment_params, f)

    # Define the binaries we are running
    binary_paths = random.choices(
        [b.path for b in exp.binaries],
        weights=[b.percent_of_nodes for b in exp.binaries],
        k=args.node_count,
    )

    # Generate the network graph and the Shadow config for the binaries
    generate_graph(
        binary_paths,
        "graph.gml",
        "shadow.yaml",
        params_file_location=os.path.join(os.getcwd(), params_file_name),
    )

    subprocess.run(
        ["shadow", "--progress", "true", "-d", args.output_dir, "shadow.yaml"],
    )

    # Analyse message deliveries
    analyse_message_deliveries(args.output_dir)

    # Move files to output_dir
    os.rename("shadow.yaml", os.path.join(args.output_dir, "shadow.yaml"))
    os.rename("graph.gml", os.path.join(args.output_dir, "graph.gml"))
    os.rename("params.json", os.path.join(args.output_dir, "params.json"))
    os.rename("plots", os.path.join(args.output_dir, "plots"))


if __name__ == "__main__":
    main()
