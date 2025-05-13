use byteorder::{ByteOrder, LittleEndian};
use crate::script_action::NodeID;
use libp2p::identity;

/// Generate a private key for a node ID
pub fn node_priv_key(id: NodeID) -> identity::Keypair {
    // Create a deterministic seed based on the node ID
    let mut seed = [0u8; 32];
    LittleEndian::write_i32(&mut seed[0..4], id);
    
    // Create a keypair from the seed
    identity::Keypair::ed25519_from_bytes(seed).expect("Failed to create keypair")
}

// This function verifies that a peer ID matches what we expect for a node ID
pub fn verify_peer_id_for_node(peer_id: &libp2p::PeerId, node_id: NodeID) -> bool {
    let expected_keypair = node_priv_key(node_id);
    let expected_peer_id = libp2p::PeerId::from(expected_keypair.public());
    
    expected_peer_id == *peer_id
}