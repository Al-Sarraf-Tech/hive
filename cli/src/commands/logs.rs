use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::{ContainerLogsRequest, ListContainersRequest};

pub async fn run(service: &str, follow: bool, tail: u32, addr: &str) -> Result<()> {
    let mut client = grpc_client::connect(addr).await?;

    let containers = client
        .list_containers(ListContainersRequest {
            service_name: service.into(),
            node_name: String::new(),
        })
        .await?
        .into_inner();

    if containers.containers.is_empty() {
        println!("No containers found for service {}", service.yellow());
        return Ok(());
    }

    let container_id = &containers.containers[0].id;
    let short_id = crate::grpc_client::short_id(container_id);

    if containers.containers.len() > 1 {
        println!(
            "Service has {} containers, showing logs from {}",
            containers.containers.len(),
            short_id
        );
    }

    println!(
        "Streaming logs for {} (container {})...",
        service.cyan(),
        short_id
    );

    let mut stream = client
        .container_logs(ContainerLogsRequest {
            container_id: container_id.clone(),
            follow,
            tail_lines: tail,
            service_name: service.to_string(),
        })
        .await?
        .into_inner();

    while let Some(entry) = stream
        .message()
        .await
        .map_err(crate::grpc_client::map_grpc_error)?
    {
        let line = entry.line.trim_end();
        let prefix = if entry.stream == "stderr" {
            format!("{} {}", entry.node_name, "ERR".red())
        } else {
            entry.node_name.dimmed().to_string()
        };
        println!("{} | {}", prefix, line);
    }

    Ok(())
}
