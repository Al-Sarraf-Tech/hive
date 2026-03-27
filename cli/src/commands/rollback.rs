use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::RollbackServiceRequest;

pub async fn run(service: &str, addr: &str, ca_cert: Option<&str>) -> Result<()> {
    println!("Rolling back {}...", service.cyan());

    let mut client = grpc_client::connect(addr, ca_cert).await?;
    client
        .rollback_service(RollbackServiceRequest {
            name: service.into(),
        })
        .await?;

    println!("{} Rolled back to previous version.", "✓".green());
    Ok(())
}
