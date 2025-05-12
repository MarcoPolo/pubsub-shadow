from collections import defaultdict
from dataclasses import dataclass, field
import random
from typing import List

from script_action import ScriptAction
import script_action


@dataclass
class Binary:
    path: str
    percent_of_nodes: int


@dataclass
class ExperimentParams:
    gossipSubParams: dict
    script: List[ScriptAction] = field(default_factory=list)


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


def params(experiment_name: str) -> ExperimentParams:
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
