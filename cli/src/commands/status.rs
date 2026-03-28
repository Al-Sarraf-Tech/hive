use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::NodeStatus;

pub async fn run(addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;

    let resp = client
        .get_cluster_status(())
        .await
        .map_err(crate::grpc_client::map_grpc_error)?
        .into_inner();

    println!("{}", "Hive Cluster Status".bold());
    println!(
        "  Nodes:      {}/{}",
        resp.healthy_nodes.to_string().green(),
        resp.total_nodes
    );
    println!("  Services:   {}", resp.total_services.to_string().cyan());
    println!(
        "  Containers: {}",
        resp.running_containers.to_string().cyan()
    );

    if !resp.nodes.is_empty() {
        println!();
        for node in &resp.nodes {
            let (indicator, status_text) = match NodeStatus::try_from(node.status) {
                Ok(NodeStatus::Ready) => ("●".green(), "ready".green()),
                Ok(NodeStatus::Draining) => ("◐".yellow(), "draining".yellow()),
                Ok(NodeStatus::Down) => ("○".red(), "down".red()),
                _ => ("?".dimmed(), "unknown".dimmed()),
            };
            let caps = node
                .capabilities
                .as_ref()
                .map(|c| format!("{}/{} ({})", c.os, c.arch, c.container_runtime))
                .unwrap_or_default();
            println!("  {} {} — {}", indicator, node.name.bold(), caps);
            println!("    status: {status_text}");
        }
    }

    Ok(())
}
