import re
import os
import json
import sys
import numpy as np
import matplotlib.pyplot as plt
from datetime import datetime

# Define the updated regex pattern
line_pattern = r"(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6})\s+(.*)"
paranthesis_pattern = r"\((.*?)\)"

BASE_PATH = "/hosts/node"
STDOUT_LOGFILE = "/pubsub-shadow.1000.stdout"


def extract_data(log_line):
    info_match = re.search(paranthesis_pattern, log_line)
    values = []
    if info_match:
        pairs = (info_match.group(1)).split(", ")
        for item in pairs:
            value = item.split(": ")[1]
            if "[" in value:
                matches = re.findall(r'"([^"]+)"', value)
                values.append(matches)
            else:
                values.append(value)
    else:
        raise Exception("couldn't extract content for log_line" + log_line)

    return values


def read_node_logs(lines):
    timelines = {
        "added": [],
        "removed": [],
        "throttled": [],
        "joined": [],
        "left": [],
        "grafted": [],
        "pruned": [],
        "msgs": {},
    }

    def add_timestamp(msg_id, key, timestamp):
        if msg_id is None:
            timelines[key].append(timestamp)
            return

        msg_id = msg_id.replace('"', "")

        if msg_id not in timelines["msgs"]:
            timelines["msgs"][msg_id] = {
                "validated": [],
                "delivered": [],
                "rejected": [],
                "received": [],
                "published": [],
                "duplicate": [],
                "undelivered": [],
                "idontwants_sent": [],
                "idontwants_received": [],
                "ihaves_sent": [],
                "ihaves_received": [],
                "iwants_sent": [],
                "iwants_received": [],
                "iannounces_sent": [],
                "iannounces_received": [],
                "ineeds_sent": [],
                "ineeds_received": [],
                # these are publish messages of the rpc
                "rpcs_sent": [],
                "rpcs_received": [],
            }

        timelines["msgs"][msg_id][key].append(timestamp)

    for line in lines:
        match = re.match(line_pattern, line.strip())
        if match:
            log_date_time = match.group(1)  # Date (YYYY/MM/DD)
            log_content = match.group(2)  # Log content

            timestamp = datetime.strptime(
                log_date_time, "%Y/%m/%d %H:%M:%S.%f"
            ).timestamp()

            if "GossipSub: Duplicated" in log_content:
                ext = extract_data(log_content)
                msg_id = ext[0]
                add_timestamp(msg_id, "duplicate", (timestamp))
            elif "Received:" in log_content:
                ext = extract_data(log_content)
                msg_id = ext[1]
                topic = ext[0]
                add_timestamp(msg_id, "received", (timestamp, topic))
            elif "Published:" in log_content:
                ext = extract_data(log_content)
                msg_id = ext[1]
                topic = ext[0]
                add_timestamp(msg_id, "published", (timestamp, topic))
            elif "GossipSub: Rejected" in log_content:
                ext = extract_data(log_content)
                msg_id = ext[0]
                add_timestamp(msg_id, "rejected", (timestamp))
            elif "GossipSub: Delivered" in log_content:
                ext = extract_data(log_content)
                msg_id = ext[0]
                add_timestamp(msg_id, "delivered", (timestamp))
            elif "GossipSub: Undeliverable" in log_content:
                ext = extract_data(log_content)
                msg_id = ext[0]
                add_timestamp(msg_id, "undelivered", (timestamp))
            elif "GossipSub: Validated" in log_content:
                ext = extract_data(log_content)
                msg_id = ext[0]
                add_timestamp(msg_id, "validated", (timestamp))
            elif "GossipSub: Grafted" in log_content:
                ext = extract_data(log_content)
                peer_id = ext[1]
                topic = ext[0]
                add_timestamp(None, "grafted", (timestamp, topic, peer_id))
            elif "GossipSub: Pruned" in log_content:
                ext = extract_data(log_content)
                peer_id = ext[1]
                topic = ext[0]
                add_timestamp(None, "pruned", (timestamp, topic, peer_id))
            elif "GossipSub: Joined" in log_content:
                ext = extract_data(log_content)
                topic = ext[0]
                add_timestamp(None, "joined", (timestamp, topic))
            elif "GossipSub: Left" in log_content:
                ext = extract_data(log_content)
                topic = ext[0]
                add_timestamp(None, "left", (timestamp, topic))
            elif "GossipSub: Peer Removed" in log_content:
                ext = extract_data(log_content)
                peer_id = ext[0]
                add_timestamp(None, "removed", (timestamp, peer_id))
            elif "GossipSub: Peer Added" in log_content:
                ext = extract_data(log_content)
                peer_id = ext[0]
                add_timestamp(None, "added", (timestamp, topic))
            elif "GossipSub: Throttled" in log_content:
                ext = extract_data(log_content)
                peer_id = ext[0]
                add_timestamp(None, "throttled", (timestamp, peer_id))
            elif "GossipSubRPC:" in log_content:
                ext = extract_data(log_content)
                if "Publish" in log_content:
                    msg_id = ext[1]
                    topic = ext[0]
                    if "Received" in log_content:
                        add_timestamp(msg_id, "rpcs_received", (timestamp, topic))
                    elif "Sent" in log_content:
                        add_timestamp(msg_id, "rpcs_sent", (timestamp, topic))
                elif "IHAVE" in log_content:
                    topic = ext[0]
                    if "Received" in log_content:
                        for msg_id in ext[1]:
                            add_timestamp(msg_id, "ihaves_received", (timestamp, topic))
                    elif "Sent" in log_content:
                        for msg_id in ext[1]:
                            add_timestamp(msg_id, "ihaves_sent", (timestamp, topic))
                elif "IWANT" in log_content:
                    if "Received" in log_content:
                        for msg_id in ext[0]:
                            add_timestamp(msg_id, "iwants_received", (timestamp, topic))
                    elif "Sent" in log_content:
                        for msg_id in ext[0]:
                            add_timestamp(msg_id, "iwants_sent", (timestamp, topic))
                elif "IDONTWANT" in log_content:
                    if "Received" in log_content:
                        for msg_id in ext[0]:
                            add_timestamp(
                                msg_id, "idontwants_received", (timestamp, topic)
                            )
                    elif "Sent" in log_content:
                        for msg_id in ext[0]:
                            add_timestamp(msg_id, "idontwants_sent", (timestamp, topic))
                elif "INEED" in log_content:
                    msg_id = ext[0]
                    if "Received" in log_content:
                        add_timestamp(msg_id, "ineeds_received", (timestamp))
                    elif "Sent" in log_content:
                        add_timestamp(msg_id, "ineeds_sent", (timestamp))
                elif "IANNOUNCE" in log_content:
                    msg_id = ext[1]
                    topic = ext[0]
                    if "Received" in log_content:
                        add_timestamp(msg_id, "iannounces_received", (timestamp, topic))
                    elif "Sent" in log_content:
                        add_timestamp(msg_id, "iannounces_sent", (timestamp, topic))
        else:
            raise Exception("Couldn't match pattern for timestamps")

    return timelines


def extract_node_timelines(folder, count):
    extracted_data = {}
    for id in range(count - 1):
        filename = folder + BASE_PATH + str(id) + STDOUT_LOGFILE
        if isSyncTest:
            filename = folder + "/node" + str(id) + ".log"
        with open(
            filename,
            "r",
            encoding="utf-8",
            errors="replace",
        ) as f:
            logs = f.readlines()
            extracted_data[str(id)] = read_node_logs(logs)

    return extracted_data


def analyse_timeline(extracted_data, shouldhave):
    arrival_times = {}
    rx_msgs = []
    dup_msgs = []

    # we know node 0 is the publiisher
    timeline = extracted_data["0"]["msgs"]

    publishing_times = {}
    first_publish = 0
    last_publish = 0
    for msg_id in timeline:
        if len(timeline[msg_id]["published"]) > 0:
            # since every messaage is published only once we don't need to sort
            publishing_times[msg_id] = (timeline[msg_id]["published"][0])[0]

            if first_publish == 0 or publishing_times[msg_id] < first_publish:
                first_publish = publishing_times[msg_id]

            if publishing_times[msg_id] > last_publish:
                last_publish = publishing_times[msg_id]
        else:
            pass

    for id in extracted_data:
        timeline = extracted_data[id]["msgs"]

        dups = 0
        received = 0

        rx_times = {}
        first_receive = 0.0
        last_receive = 0.0

        for msg_id in timeline:
            if len(timeline[msg_id]["delivered"]) > 0:
                # the first time we received a particular msg_id
                rx_times[msg_id] = sorted(timeline[msg_id]["delivered"])[0]

                # received time is the time at which the last message (any msg_id) was received
                if first_receive == 0.0 or rx_times[msg_id] < first_receive:
                    first_receive = rx_times[msg_id]

                if rx_times[msg_id] > last_receive:
                    last_receive = rx_times[msg_id]

                received += 1

            else:
                rx_times[msg_id] = 0.0

            dups += len(timeline[msg_id]["duplicate"])

            if msg_id not in arrival_times:
                arrival_times[msg_id] = []

            # this can be negative if the message was not received
            arrival_times[msg_id].append(
                (id, rx_times[msg_id] - publishing_times[msg_id])
            )

        rx_msgs.append(shouldhave - received)
        dup_msgs.append(dups)

        if first_receive == 0.0 or last_receive == 0.0:
            # the node for some reason did not receive any messages
            continue  # TODO: there must be implication of this on the plot. Resolve them

        if "f2l" not in arrival_times:
            arrival_times["f2l"] = []

        if "l2f" not in arrival_times:
            arrival_times["l2f"] = []

        arrival_times["f2l"].append((id, last_receive - first_publish))
        arrival_times["l2f"].append((id, first_receive - last_publish))

    return arrival_times, rx_msgs, dup_msgs


def plot_cdf(data, label):
    x = [v for _, v in data]
    y = np.arange(len(data)) / float(len(data))

    x.sort()

    plt.plot(x, y, label=label)


folder = sys.argv[1]
isSyncTest = "synctest" in folder


if __name__ == "__main__":
    nodeCount = 0
    if isSyncTest:
        # List files in folder
        files = os.listdir(folder)
        nodeCount = len(files)
    else:
        files = os.listdir(folder + "/hosts")
        nodeCount = len(files)

    timeline = {}
    # this value is tuned after running this script for a couple times
    max_arr_time = 10.0

    print("Parsing log files")
    # Read Simulations from folder
    timeline = extract_node_timelines(folder, nodeCount)

    with open(f"{folder}/analysed_timeline.json", "w") as f:
        json.dump(timeline, f)

    print(f"\tAnalysis for {folder}")
    # only for one message published
    num_msgs = len(timeline["0"]["msgs"])
    print(f"Number of messages: {num_msgs}")
    arr_times, rx_count, dups = analyse_timeline(timeline, num_msgs)

    msg_times = {}
    for msgId, times in arr_times.items():
        if msgId == "f2l" or msgId == "l2f":
            continue
        for nodeId, time in times:
            msg_times.setdefault(msgId, []).append(time)

    # Plot msg delivery count across time
    plt.figure()
    for msgId, times in msg_times.items():
        sorted_times = sorted(times)
        # Create step function data: y = 1,2,...,len(times)
        cdf = list(range(1, len(sorted_times) + 1))
        plt.step(sorted_times, cdf, where="post", label=msgId)

    plt.xlim(0.0, max_arr_time)
    plt.xlabel("Time")
    plt.ylabel("Number of Nodes")
    # plt.legend(fontsize="small")
    plt.title(f"Node delivery count of msgID across time {folder}")
    plt.grid(True)

    # Make plots dir if it doesn't exist
    os.makedirs("./plots", exist_ok=True)

    plt.savefig(f"./plots/{folder}.png")
    print("plot saved")
