use byteorder::{BigEndian, ByteOrder};
use futures::StreamExt;
use libp2p::gossipsub::{self, IdentTopic, MessageId};
use libp2p::Swarm;
use slog::{error, info, Logger};
use std::collections::HashMap;
use std::time::{Duration, Instant};
use tokio::time::sleep;

use crate::connector::HostConnector;
use crate::script_action::{ExperimentParams, NodeID, ScriptAction};

// Calculate message ID based on content (equivalent to Go's CalcID)
pub fn calc_id(data: &[u8]) -> String {
    if data.len() >= 8 {
        format!("{}", BigEndian::read_u64(data))
    } else {
        // If data is too short, return a placeholder
        "invalid_message".to_string()
    }
}

// Custom message ID function similar to Go implementation
pub fn message_id_fn(message: &gossipsub::Message) -> MessageId {
    MessageId::from(calc_id(&message.data))
}

pub struct ScriptedNode {
    node_id: NodeID,
    swarm: Swarm<gossipsub::Behaviour>,
    logger: Logger,
    connector: ShadowConnector,
    topics: HashMap<String, IdentTopic>,
    start_time: Instant,
}

use crate::connector::ShadowConnector;

impl ScriptedNode {
    pub fn new(
        node_id: NodeID,
        swarm: Swarm<gossipsub::Behaviour>,
        logger: Logger,
        connector: ShadowConnector,
        start_time: Instant,
    ) -> Self {
        Self {
            node_id,
            swarm,
            logger,
            connector,
            topics: HashMap::new(),
            start_time,
        }
    }

    pub fn get_topic(&mut self, topic_str: &str) -> IdentTopic {
        if let Some(topic) = self.topics.get(topic_str) {
            topic.clone()
        } else {
            let topic = IdentTopic::new(topic_str);
            self.topics.insert(topic_str.to_string(), topic.clone());
            topic
        }
    }

    pub fn run_action(
        &mut self,
        action: ScriptAction,
    ) -> futures::future::BoxFuture<'_, Result<(), Box<dyn std::error::Error>>> {
        Box::pin(async move {
            match action {
                ScriptAction::Connect { connect_to } => {
                    for target_node_id in connect_to {
                        match self
                            .connector
                            .connect_to(&mut self.swarm, target_node_id)
                            .await
                        {
                            Ok(_) => {
                                info!(self.logger, "Connected to node {}", target_node_id);
                            }
                            Err(e) => {
                                error!(
                                    self.logger,
                                    "Failed to connect to node {}: {}", target_node_id, e
                                );
                                return Err(e);
                            }
                        }
                    }
                    info!(self.logger, "Node {} connected to peers", self.node_id);
                }
                ScriptAction::IfNodeIDEquals { node_id, action } => {
                    if node_id == self.node_id {
                        self.run_action(*action).await?;
                    }
                }
                ScriptAction::WaitUntil { elapsed_seconds } => {
                    let target_time = self.start_time + Duration::from_secs(elapsed_seconds);
                    let now = Instant::now();

                    if now < target_time {
                        let wait_time = target_time.duration_since(now);
                        info!(
                            self.logger,
                            "Waiting {:?} (until elapsed: {}s)", wait_time, elapsed_seconds
                        );

                        // Create a timeout future
                        let mut timeout = Box::pin(sleep(wait_time));

                        // Process events while waiting for the timeout
                        loop {
                            tokio::select! {
                                _ = &mut timeout => {
                                    // Timeout complete, we can continue
                                    break;
                                }
                                event = self.swarm.select_next_some() => {
                                    // Process any messages that arrive during sleep
                                    if let libp2p::swarm::SwarmEvent::Behaviour(gossipsub::Event::Message {
                                        propagation_source: peer_id,
                                        message_id: _,
                                        message,
                                    }) = event {
                                        if message.data.len() >= 8 {
                                            let msg_id = BigEndian::read_u64(&message.data);
                                            info!(self.logger, "Received message {}", msg_id;
                                                "id" => msg_id, "from" => peer_id.to_string());
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
                ScriptAction::Publish {
                    message_id,
                    message_size_bytes,
                    topic_id,
                } => {
                    let topic = self.get_topic(&topic_id);

                    info!(self.logger, "Publishing message {}", message_id);

                    let mut msg = vec![0u8; message_size_bytes];
                    BigEndian::write_u64(&mut msg, message_id);

                    match self.swarm.behaviour_mut().publish(topic, msg.clone()) {
                        Ok(_) => {
                            info!(self.logger, "Published message {}", message_id);
                        }
                        Err(e) => {
                            error!(
                                self.logger,
                                "Failed to publish message {}: {}", message_id, e
                            );
                            return Err(Box::new(std::io::Error::new(
                                std::io::ErrorKind::Other,
                                e.to_string(),
                            )));
                        }
                    }
                }
                ScriptAction::SubscribeToTopic { topic_id } => {
                    let topic = self.get_topic(&topic_id);

                    match self.swarm.behaviour_mut().subscribe(&topic) {
                        Ok(_) => {
                            info!(self.logger, "Subscribed to topic {}", topic_id);
                        }
                        Err(e) => {
                            error!(
                                self.logger,
                                "Failed to subscribe to topic {}: {}", topic_id, e
                            );
                            return Err(Box::new(std::io::Error::new(
                                std::io::ErrorKind::Other,
                                e.to_string(),
                            )));
                        }
                    }
                }
            }

            Ok(())
        })
    }
}

pub async fn run_experiment(
    start_time: Instant,
    logger: Logger,
    swarm: Swarm<gossipsub::Behaviour>,
    node_id: NodeID,
    connector: ShadowConnector,
    params: ExperimentParams,
) -> Result<(), Box<dyn std::error::Error>> {
    let mut node = ScriptedNode::new(node_id, swarm, logger.clone(), connector, start_time);
    for action in params.script {
        node.run_action(action).await?;
    }
    Ok(())
}
