use anyhow::{Result, bail};
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::{ContainerLogsRequest, ListContainersRequest};

pub async fn run(
    service: Option<&str>,
    all: bool,
    follow: bool,
    tail: u32,
    addr: &str,
    ca_cert: Option<&str>,
) -> Result<()> {
    if all {
        run_all(follow, tail, addr, ca_cert).await
    } else {
        let svc = service.unwrap_or_default();
        if svc.is_empty() {
            bail!("either a service name or --all is required");
        }
        run_single(svc, follow, tail, addr, ca_cert).await
    }
}

async fn run_single(
    service: &str,
    follow: bool,
    tail: u32,
    addr: &str,
    ca_cert: Option<&str>,
) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;

    let containers = client
        .list_containers(ListContainersRequest {
            service_name: service.into(),
            node_name: String::new(),
        })
        .await
        .map_err(grpc_client::map_grpc_error)?
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
        .await
        .map_err(grpc_client::map_grpc_error)?
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

async fn run_all(follow: bool, tail: u32, addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;

    // List all containers (empty service_name = all services)
    let containers = client
        .list_containers(ListContainersRequest {
            service_name: String::new(),
            node_name: String::new(),
        })
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    if containers.containers.is_empty() {
        println!("{}", "No running containers found".yellow());
        return Ok(());
    }

    println!(
        "Streaming logs from {} containers across all services...",
        containers.containers.len()
    );

    // Request logs with empty service_name to get all services
    let mut stream = client
        .container_logs(ContainerLogsRequest {
            container_id: String::new(),
            follow,
            tail_lines: tail,
            service_name: String::new(),
        })
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    while let Some(entry) = stream
        .message()
        .await
        .map_err(crate::grpc_client::map_grpc_error)?
    {
        let line = entry.line.trim_end();
        let svc_tag = if entry.service_name.is_empty() {
            crate::grpc_client::short_id(&entry.container_id).to_string()
        } else {
            entry.service_name.clone()
        };

        let prefix = if entry.stream == "stderr" {
            format!("[{}] {} {}", svc_tag.cyan(), entry.node_name, "ERR".red())
        } else {
            format!("[{}] {}", svc_tag.cyan(), entry.node_name.dimmed())
        };
        println!("{} | {}", prefix, line);
    }

    Ok(())
}
