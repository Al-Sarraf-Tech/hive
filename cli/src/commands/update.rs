use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::UpdateServiceRequest;

pub async fn run(
    service: &str,
    image: Option<&str>,
    replicas: Option<u32>,
    env: &[String],
    addr: &str,
    ca_cert: Option<&str>,
) -> Result<()> {
    if image.is_none() && replicas.is_none() && env.is_empty() {
        anyhow::bail!("at least one of --image, --replicas, or --env must be specified");
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
    if !env.is_empty() {
        parts.push(format!("env={} vars", env.len().to_string().yellow()));
    }
    println!("Updating {} ({})...", service.cyan(), parts.join(", "));

    // Parse KEY=VALUE env pairs
    let mut env_map = std::collections::HashMap::new();
    for kv in env {
        let (k, v) = kv.split_once('=').ok_or_else(|| {
            anyhow::anyhow!("invalid env format '{}' — expected KEY=VALUE", kv)
        })?;
        env_map.insert(k.to_string(), v.to_string());
    }

    let mut client = grpc_client::connect(addr, ca_cert).await?;
    client
        .update_service(UpdateServiceRequest {
            name: service.into(),
            image: image.unwrap_or("").into(),
            replicas: replicas.unwrap_or(0),
            env: env_map,
        })
        .await
        .map_err(grpc_client::map_grpc_error)?;

    println!("{} Updated (rolling restart in progress).", "✓".green());
    Ok(())
}
