use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;

pub async fn run(addr: &str) -> Result<()> {
    let mut client = grpc_client::connect(addr).await?;

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
            let (indicator, status_text) = match node.status {
                1 => ("●".green(), "ready".green()),
                2 => ("◐".yellow(), "draining".yellow()),
                3 => ("○".red(), "down".red()),
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
