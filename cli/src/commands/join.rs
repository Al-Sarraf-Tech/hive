use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::JoinClusterRequest;

pub async fn run(address: &str, addr: &str) -> Result<()> {
    let mut client = grpc_client::connect(addr).await?;

    let resp = client
        .join_cluster(JoinClusterRequest {
            seed_addrs: vec![address.to_string()],
        })
        .await?
        .into_inner();

    println!(
        "{} Joined cluster ({} nodes contacted)",
        "✓".green(),
        resp.nodes_joined
    );
    for node in &resp.nodes {
        let status = match node.status {
            1 => "●".green(),
            2 => "◐".yellow(),
            _ => "○".dimmed(),
        };
        println!("  {} {}", status, node.name.bold());
    }
    Ok(())
}
