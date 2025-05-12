from collections import defaultdict
from dataclasses import dataclass, field, asdict
import argparse
import json
import os
import random
import subprocess
from typing import List

from script_action import ScriptAction
import script_action
from network_graph import generate_graph

from analyze_message_deliveries import analyse_message_deliveries

params_file_name = "params.json"


@dataclass
class Binary:
    path: str
    percent_of_nodes: int


@dataclass
class ExperimentParams:
    gossipSubParams: dict
    script: List[ScriptAction] = field(default_factory=list)


@dataclass
class Experiment:
    binaries: List[Binary]
    experiment_params: ExperimentParams


def scenario(
    scenario_name: str, node_count: int, number_of_conns_per_node: int
) -> List[ScriptAction]:
    actions: List[ScriptAction] = []
    match scenario_name:
        case "subnet-blob-msg":
            actions.extend(random_network_mesh(node_count, number_of_conns_per_node))
            message_size = 2 * 1024 * 48
            num_messages = 32
            actions.extend(
                random_publish_every_12s(node_count, num_messages, message_size)
            )
        case _:
            raise ValueError(f"Unknown scenario name: {scenario_name}")

    return actions


def composition(preset_name: str) -> List[Binary]:
    match preset_name:
        case "all-go":
            return [Binary("gossipsub-v0.13.1/gossipsub-bin", percent_of_nodes=100)]
    raise ValueError(f"Unknown preset name: {preset_name}")


def experiment_params(experiment_name: str) -> ExperimentParams:
    match experiment_name:
        case "gossipsub-v0.13.1-stock":
            return ExperimentParams(gossipSubParams={})
        case "gossipsub-v0.13.1-stock-smaller-D":
            return ExperimentParams(
                gossipSubParams={
                    "D": 4,
                    "Dlo": 1,
                    "Dhi": 6,
                    "Dscore": 1,
                    "Dout": 0,
                },
            )

    raise ValueError(f"Unknown experiment name: {experiment_name}")


def random_network_mesh(
    node_count: int, number_of_connections: int
) -> List[ScriptAction]:
    connections = defaultdict(list)
    for node_id in range(node_count):
        while len(connections[node_id]) < number_of_connections:
            target = random.randint(0, node_count - 1)
            if target == node_id:
                continue
            connections[node_id].append(target)
            connections[target].append(node_id)

    actions = []
    for node_id, node_connections in connections.items():
        actions.append(
            script_action.Connect(
                nodeID=node_id,
                connectTo=node_connections,
            )
        )
    return actions


def random_publish_every_12s(
    node_count: int, numMessages: int, messageSize: int
) -> List[ScriptAction]:
    # Start at 120 seconds (2 minutes) to allow for setup time
    elapsed_seconds = 120
    actions = []
    actions.append(script_action.WaitUntil(elapsedSeconds=elapsed_seconds))

    for _ in range(numMessages):
        random_node = random.randint(0, node_count - 1)
        actions.append(
            script_action.IfNodeIDEquals(
                nodeID=random_node,
                action=script_action.Publish(
                    messageSizeBytes=messageSize,
                    publisherIndex=random.randint(0, node_count - 1),
                ),
            )
        )
        elapsed_seconds += 12  # Add 12 seconds for each subsequent message
        actions.append(script_action.WaitUntil(elapsedSeconds=elapsed_seconds))

    return actions


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--node_count", type=int, required=True)
    parser.add_argument("--seed", type=int, required=False, default=1)
    parser.add_argument("--experiment", type=str, required=True)
    parser.add_argument(
        "--scenario", type=str, required=False, default="subnet-blob-msg"
    )
    parser.add_argument("--composition", type=str, required=False, default="all-go")
    parser.add_argument("--output_dir", type=str, required=False)
    args = parser.parse_args()

    if args.output_dir is None:
        args.output_dir = f"{args.experiment}.data"

    random.seed(args.seed)

    subprocess.run(["make", "binaries"])

    c = composition(args.composition)
    params = experiment_params(args.experiment)
    params.script = scenario(args.scenario, args.node_count, 10)

    exp = Experiment(
        binaries=c,
        experiment_params=params,
    )

    with open(params_file_name, "w") as f:
        json.dump(asdict(exp.experiment_params), f)

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
