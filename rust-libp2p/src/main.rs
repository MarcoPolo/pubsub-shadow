use std::time::Instant;
use std::fs::File;
use std::path::Path;
use std::io::Read;
use clap::Parser;
use libp2p::{
    gossipsub::{
        self, MessageAuthenticity, Message, MessageId, ValidationMode
    },
    noise,
    tcp,
    yamux,
    PeerId,
    Transport,
    core::upgrade,
    Swarm,
};
use slog::{Drain, Logger, o};

mod connector;
mod experiment;
mod key;
mod script_action;

use connector::ShadowConnector;
use script_action::{ExperimentParams, NodeID};
use key::node_priv_key;
use experiment::{run_experiment, calc_id};

#[derive(Parser, Debug)]
#[clap(author, version, about)]
struct Args {
    /// Path to the params file
    #[clap(long, value_name = "FILE")]
    params: String,
}

fn create_logger() -> Logger {
    let decorator = slog_term::TermDecorator::new().build();
    let drain = slog_term::FullFormat::new(decorator).build().fuse();
    let drain = slog_async::Async::new(drain).build().fuse();
    slog::Logger::root(drain, o!())
}

fn read_params(path: &str) -> Result<ExperimentParams, Box<dyn std::error::Error>> {
    if !path.ends_with(".json") {
        return Err("Params file must be a .json file".into());
    }

    let path = Path::new(path);
    if !path.exists() {
        return Err("Params file does not exist".into());
    }

    let mut file = File::open(path)?;
    let mut contents = String::new();
    file.read_to_string(&mut contents)?;

    let params: ExperimentParams = serde_json::from_str(&contents)?;
    Ok(params)
}

// Apply gossipsub parameters from the config file to the gossipsub config
fn apply_gossipsub_params(
    config: &mut gossipsub::ConfigBuilder,
    params: &script_action::GossipSubParams,
) {
    if let Some(d) = params.d {
        config.mesh_n(d as usize);
    }
    if let Some(d_low) = params.d_low {
        config.mesh_n_low(d_low as usize);
    }
    if let Some(d_high) = params.d_high {
        config.mesh_n_high(d_high as usize);
    }
    if let Some(heartbeat_initial_delay) = params.heartbeat_initial_delay {
        config.heartbeat_initial_delay(std::time::Duration::from_secs_f64(heartbeat_initial_delay));
    }
    if let Some(heartbeat_interval) = params.heartbeat_interval {
        config.heartbeat_interval(std::time::Duration::from_secs_f64(heartbeat_interval));
    }
    if let Some(fanout_ttl) = params.fanout_ttl {
        config.fanout_ttl(std::time::Duration::from_secs_f64(fanout_ttl));
    }
    if let Some(history_length) = params.history_length {
        config.history_length(history_length as usize);
    }
    if let Some(history_gossip) = params.history_gossip {
        config.history_gossip(history_gossip as usize);
    }
    if let Some(flood_publish) = params.flood_publish {
        config.flood_publish(flood_publish);
    }
    if let Some(max_ihave_length) = params.max_ihave_length {
        config.max_ihave_length(max_ihave_length as usize);
    }
    if let Some(max_ihave_messages) = params.max_ihave_messages {
        config.max_ihave_messages(max_ihave_messages as usize);
    }
    if let Some(iwant_followup_time) = params.iwant_followup_time {
        config.iwant_followup_time(std::time::Duration::from_secs_f64(iwant_followup_time));
    }
}

// Get the node_id from hostname
fn get_node_id() -> Result<NodeID, Box<dyn std::error::Error>> {
    let hostname = hostname::get()?.into_string().unwrap_or_default();
    
    // Parse "nodeX" format
    let mut chars = hostname.chars();
    // Skip "node" prefix
    for _ in 0..4 {
        if chars.next().is_none() {
            return Err("Invalid hostname format".into());
        }
    }
    
    // Parse remaining digits as node ID
    let id_str: String = chars.collect();
    let node_id = id_str.parse::<NodeID>()?;
    
    Ok(node_id)
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args = Args::parse();
    let logger = create_logger();
    let start_time = Instant::now();
    
    // Load experiment parameters
    let params = read_params(&args.params)?;
    
    // Get the node ID from hostname
    let node_id = get_node_id()?;
    
    // Create identity key from node ID
    let local_key = node_priv_key(node_id);
    let local_peer_id = PeerId::from(local_key.public());
    
    slog::info!(logger, "Local peer id: {}", local_peer_id);
    slog::info!(logger, "Node ID: {}", node_id);
    
    // Create a transport
    let transport = tcp::tokio::Transport::default()
        .upgrade(upgrade::Version::V1)
        .authenticate(noise::Config::new(&local_key)?)
        .multiplex(yamux::Config::default())
        .boxed();
    
    // Define a custom message ID function
    let message_id_fn = |message: &Message| {
        MessageId::from(calc_id(&message.data))
    };

    // Create gossipsub configuration
    let mut config_builder = gossipsub::ConfigBuilder::default();
    config_builder
        .validation_mode(ValidationMode::Permissive)
        .message_id_fn(message_id_fn);
    
    // Apply custom params if provided
    if let Some(params) = &params.gossip_sub_params {
        apply_gossipsub_params(&mut config_builder, params);
    }
    
    // Create gossipsub configuration
    let gossipsub_config = config_builder.build().expect("Valid gossipsub config");
    
    // Create gossipsub behavior
    let gossipsub = gossipsub::Behaviour::new(
        MessageAuthenticity::Anonymous,
        gossipsub_config,
    )?;
    
    // Build swarm
    let mut swarm = Swarm::new(
        transport,
        gossipsub,
        local_peer_id,
        libp2p::swarm::Config::with_tokio_executor(),
    );
    
    // Listen on all interfaces
    swarm.listen_on("/ip4/0.0.0.0/tcp/9000".parse()?)?;
    
    // Setup connector
    let connector = ShadowConnector;
    
    // Run the experiment
    run_experiment(
        start_time,
        logger,
        swarm,
        node_id,
        connector,
        params,
    ).await?;
    
    Ok(())
}