use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::InitClusterRequest;

pub async fn run(name: &str, addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;

    let resp = client
        .init_cluster(InitClusterRequest {
            cluster_name: name.to_string(),
        })
        .await?
        .into_inner();

    println!("{} Cluster initialized", "✓".green());
    println!("  Cluster ID: {}", resp.cluster_id.cyan());
    println!("  Node:       {}", resp.node_name.bold());
    println!("  Gossip:     {}", resp.gossip_addr.cyan());
    println!();
    println!(
        "Join other nodes with: {}",
        format!("hive join {}", resp.gossip_addr).yellow()
    );
    if !resp.ca_fingerprint.is_empty() {
        println!("  CA:         {}", resp.ca_fingerprint.dimmed());
        println!();
        println!(
            "To connect with TLS: {} {}",
            "hive --ca-cert <data-dir>/pki/ca.crt status".yellow(),
            "(after daemon restart)".dimmed()
        );
    }
    Ok(())
}
