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
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    println!("{} Cluster initialized", "✓".green());
    println!("  Node:       {}", resp.node_name.bold());
    if !resp.join_code.is_empty() {
        println!("  Join Code:  {}", resp.join_code.yellow().bold());
    }
    println!("  Gossip:     {}", resp.gossip_addr.cyan());
    if !resp.join_token.is_empty() {
        println!("  Join Token: {}", resp.join_token.dimmed());
    }
    println!();
    if !resp.join_code.is_empty() {
        println!(
            "  Join other nodes: {}",
            format!(
                "hive join --code {} {}",
                resp.join_code,
                resp.gossip_addr
                    .split(':')
                    .next()
                    .unwrap_or(&resp.gossip_addr)
            )
            .yellow()
        );
    }
    println!(
        "  Or with token:    {}",
        format!("hive join --token {} {}", resp.join_token, resp.gossip_addr).dimmed()
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
