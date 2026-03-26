use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::StopServiceRequest;

pub async fn run(service: &str, addr: &str) -> Result<()> {
    println!("Stopping service {}...", service.cyan());

    let mut client = grpc_client::connect(addr).await?;
    client
        .stop_service(StopServiceRequest {
            name: service.into(),
        })
        .await?;

    println!("{} Service {} stopped.", "✓".green(), service.bold());
    Ok(())
}
