use anyhow::{Context, Result};
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::ImportClusterRequest;

/// Export cluster state to a backup file.
pub async fn backup(output: &str, addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;
    let resp = client
        .export_cluster(())
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    std::fs::write(output, &resp.data)
        .with_context(|| format!("failed to write backup to {output}"))?;

    let size = resp.data.len();
    let human_size = if size > 1_048_576 {
        format!("{:.1} MB", size as f64 / 1_048_576.0)
    } else if size > 1024 {
        format!("{:.1} KB", size as f64 / 1024.0)
    } else {
        format!("{size} bytes")
    };

    println!(
        "{} Backup saved to {} ({})",
        "OK".green(),
        output.cyan(),
        human_size
    );
    Ok(())
}

/// Restore cluster state from a backup file.
pub async fn restore(file: &str, overwrite: bool, addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let data =
        std::fs::read(file).with_context(|| format!("failed to read backup file: {file}"))?;

    let mut client = grpc_client::connect(addr, ca_cert).await?;
    let resp = client
        .import_cluster(ImportClusterRequest { data, overwrite })
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    println!(
        "{} Restored: {} services, {} secrets",
        "OK".green(),
        resp.services_imported.to_string().cyan(),
        resp.secrets_imported.to_string().cyan(),
    );
    if overwrite {
        println!("  (overwrite mode: existing keys were replaced)");
    }
    Ok(())
}
