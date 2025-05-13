from collections import defaultdict
import json
import os
import sys
from datetime import datetime
import matplotlib.pyplot as plt

messages = defaultdict(list)
duplicate_count = defaultdict(lambda: 0)
duplicate_count_by_message_and_node = defaultdict(lambda: defaultdict(int))
peer_id_to_node_id = dict()
node_id_to_peer_id = dict()


def nodeIDFromFilename(filename):
    return filename.split(".")[0]


def logfile_iterator(folder):
    """
    Returns a list of all the log files in the folder.

    Special case for shadow data folders by identifying the "hosts" subfolder.

    Otherwise, returns a list of all the files in the folder.
    """
    files = os.listdir(folder)
    if "hosts" in files:
        for host in os.listdir(os.path.join(folder, "hosts")):
            for file in os.listdir(os.path.join(folder, "hosts", host)):
                if file.endswith(".stdout"):
                    yield os.path.join(folder, "hosts", host, file)
    else:
        for file in files:
            yield os.path.join(folder, file)


def analyse_message_deliveries(folder):
    analysis_txt = []

    for file in logfile_iterator(folder):
        with open(file, "r") as f:
            node_id = ""
            for line in f:
                try:
                    parsed = json.loads(line)
                except json.JSONDecodeError:
                    continue

                if parsed["msg"] == "PeerID":
                    node_id = parsed["node_id"]
                    peer_id_to_node_id[parsed["id"]] = node_id
                    node_id_to_peer_id[node_id] = parsed["id"]
                    continue

                if parsed["service"] != "gossipsub":
                    continue
                match parsed["msg"]:
                    case "Deliver":
                        # Parse timestamp RFC3339
                        msgID = parsed["id"]
                        ts = datetime.fromisoformat(parsed["time"])
                        messages[msgID].append((ts, node_id))
                    case "Duplicate":
                        # Parse timestamp RFC3339
                        msgID = parsed["id"]
                        ts = datetime.fromisoformat(parsed["time"])
                        duplicate_count[msgID] += 1
                        duplicate_count_by_message_and_node[msgID][node_id] += 1

    # Prepare data for plotting
    msg_ids = []
    time_diffs = []

    total_nodes = len(node_id_to_peer_id)
    for msgID, deliveries in messages.items():
        deliveries.sort(key=lambda x: x[0])
        time_diff = (deliveries[-1][0] - deliveries[0][0]).total_seconds()
        msg_ids.append(msgID)
        time_diffs.append(time_diff)
        avg_duplicate_count = duplicate_count[msgID] / total_nodes
        reached = len(deliveries) / total_nodes
        analysis_txt.append(f"{msgID}, {time_diff}s, {avg_duplicate_count}, {reached}")

    # Create the plot
    plt.figure(figsize=(12, 6))
    plt.bar(range(len(msg_ids)), time_diffs)
    plt.xlabel("Message Index")
    plt.ylabel("Delivery Time Difference (seconds)")
    plt.title("Message Delivery Time Differences")
    plt.xticks(range(len(msg_ids)), msg_ids, rotation=45, ha="right")
    plt.tight_layout()

    if not os.path.exists("plots"):
        os.makedirs("plots")
    plt.savefig(f"plots/message_delivery_times_{folder}.png")
    plt.close()

    # Print the analysis and save it to a file
    with open(f"plots/analysis_{folder}.txt", "w") as f:
        f.write(
            "Message ID, Time to Disseminate, Avg Duplicate Count, Reached percent\n"
        )
        for line in analysis_txt:
            f.write(line + "\n")


def main():
    # Read folder from input
    folder = sys.argv[1]
    analyse_message_deliveries(folder)


if __name__ == "__main__":
    main()
