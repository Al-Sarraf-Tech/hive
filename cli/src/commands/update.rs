use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::UpdateServiceRequest;

pub async fn run(
    service: &str,
    image: Option<&str>,
    replicas: Option<u32>,
    addr: &str,
    ca_cert: Option<&str>,
) -> Result<()> {
    if image.is_none() && replicas.is_none() {
        anyhow::bail!("at least one of --image or --replicas must be specified");
    }

    let mut parts = Vec::new();
    if let Some(img) = image {
        parts.push(format!("image={}", img.cyan()));
    }
    if let Some(r) = replicas {
        if r == 0 {
            anyhow::bail!(
                "replica count must be at least 1 — use 'hive stop {service}' to stop a service"
            );
        }
        parts.push(format!("replicas={}", r.to_string().yellow()));
    }
    println!("Updating {} ({})...", service.cyan(), parts.join(", "));

    let mut client = grpc_client::connect(addr, ca_cert).await?;
    client
        .update_service(UpdateServiceRequest {
            name: service.into(),
            image: image.unwrap_or("").into(),
            replicas: replicas.unwrap_or(0),
        })
        .await
        .map_err(grpc_client::map_grpc_error)?;

    println!("{} Updated.", "✓".green());
    Ok(())
}
