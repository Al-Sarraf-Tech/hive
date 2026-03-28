use anyhow::{Context, Result};
use colored::Colorize;
use std::fs;

use crate::grpc_client;
use crate::grpc_client::hive_proto::{DiffAction, DiffDeployRequest};

pub async fn run(file: &str, addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let content =
        fs::read_to_string(file).with_context(|| format!("failed to read Hivefile: {file}"))?;

    // Validate TOML locally first
    let _: toml::Value =
        toml::from_str(&content).with_context(|| format!("invalid TOML in {file}"))?;

    let mut client = grpc_client::connect(addr, ca_cert).await?;
    let resp = client
        .diff_deploy(DiffDeployRequest {
            hivefile_toml: content,
        })
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    if resp.diffs.is_empty() {
        println!("No services in Hivefile.");
        return Ok(());
    }

    println!("Deploy preview for {}:", file.cyan());
    println!();

    for diff in &resp.diffs {
        let action = match DiffAction::try_from(diff.action) {
            Ok(DiffAction::Create) => "CREATE".green().bold(),
            Ok(DiffAction::Update) => "UPDATE".yellow().bold(),
            Ok(DiffAction::Unchanged) => "UNCHANGED".dimmed(),
            _ => "?".dimmed(),
        };
        println!("  {}: {}", diff.name.bold(), action);
        for change in &diff.changes {
            println!("    {}", change);
        }
    }

    Ok(())
}
