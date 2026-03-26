use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::InitClusterRequest;

pub async fn run(addr: &str) -> Result<()> {
    let mut client = grpc_client::connect(addr).await?;

    let resp = client
        .init_cluster(InitClusterRequest {
            cluster_name: String::new(),
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
    Ok(())
}
