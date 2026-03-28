use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::{JoinClusterRequest, NodeStatus};

pub async fn run(
    addresses: &[String],
    token: &str,
    addr: &str,
    ca_cert: Option<&str>,
) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;

    let resp = client
        .join_cluster(JoinClusterRequest {
            seed_addrs: addresses.to_vec(),
            join_token: token.to_string(),
        })
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    println!(
        "{} Joined cluster ({} nodes contacted)",
        "✓".green(),
        resp.nodes_joined
    );
    for node in &resp.nodes {
        let status = match NodeStatus::try_from(node.status) {
            Ok(NodeStatus::Ready) => "●".green(),
            Ok(NodeStatus::Draining) => "◐".yellow(),
            Ok(NodeStatus::Down) => "○".red(),
            _ => "?".dimmed(),
        };
        println!("  {} {}", status, node.name.bold());
    }
    Ok(())
}
