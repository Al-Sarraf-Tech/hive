use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::ScaleServiceRequest;

pub async fn run(service: &str, replicas: u32, addr: &str) -> Result<()> {
    println!(
        "Scaling {} to {} replicas...",
        service.cyan(),
        replicas.to_string().yellow()
    );

    let mut client = grpc_client::connect(addr).await?;
    client
        .scale_service(ScaleServiceRequest {
            name: service.into(),
            replicas,
        })
        .await?;

    println!("{} Scaled.", "✓".green());
    Ok(())
}
