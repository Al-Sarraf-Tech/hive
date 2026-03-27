use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::RestartServiceRequest;

pub async fn run(service: &str, addr: &str, ca_cert: Option<&str>) -> Result<()> {
    println!("Restarting {}...", service.cyan());

    let mut client = grpc_client::connect(addr, ca_cert).await?;
    client
        .restart_service(RestartServiceRequest {
            name: service.into(),
        })
        .await
        .map_err(grpc_client::map_grpc_error)?;

    println!(
        "{} Service {} restarted.",
        "\u{2713}".green(),
        service.bold()
    );
    Ok(())
}
