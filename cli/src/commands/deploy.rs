use anyhow::{Context, Result};
use colored::Colorize;
use std::fs;

use crate::grpc_client;
use crate::grpc_client::hive_proto::DeployServiceRequest;

pub async fn run(file: &str, addr: &str) -> Result<()> {
    let content =
        fs::read_to_string(file).with_context(|| format!("failed to read Hivefile: {file}"))?;

    // Validate TOML locally first
    let _parsed: toml::Value =
        toml::from_str(&content).with_context(|| format!("invalid TOML in {file}"))?;

    println!("Deploying from {}...", file.cyan());

    let mut client = grpc_client::connect(addr).await?;

    let resp = client
        .deploy_service(DeployServiceRequest {
            hivefile_toml: content,
        })
        .await?
        .into_inner();

    for svc in &resp.services {
        println!(
            "{} {} deployed (image: {}, id: {})",
            "✓".green(),
            svc.name.bold(),
            svc.image.cyan(),
            &svc.id[..svc.id.len().min(12)]
        );
    }

    Ok(())
}
