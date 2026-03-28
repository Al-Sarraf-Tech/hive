use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::{CreateVolumeRequest, DeleteVolumeRequest};

pub async fn list(addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;
    let resp = client
        .list_volumes(())
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    if resp.volumes.is_empty() {
        println!("No volumes found.");
    } else {
        println!(
            "{:<30} {:<10} {:<40} CREATED",
            "NAME", "DRIVER", "MOUNTPOINT"
        );
        for v in &resp.volumes {
            let created = if v.created_at.is_empty() {
                "-".to_string()
            } else {
                v.created_at.clone()
            };
            println!(
                "{:<30} {:<10} {:<40} {}",
                v.name, v.driver, v.mountpoint, created
            );
        }
    }

    Ok(())
}

pub async fn create(name: &str, addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;
    let resp = client
        .create_volume(CreateVolumeRequest { name: name.into() })
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    println!(
        "{} Volume {} created (mountpoint: {}).",
        "✓".green(),
        resp.name.cyan(),
        resp.mountpoint
    );
    Ok(())
}

pub async fn remove(name: &str, addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;
    client
        .delete_volume(DeleteVolumeRequest { name: name.into() })
        .await
        .map_err(grpc_client::map_grpc_error)?;

    println!("{} Volume {} removed.", "✓".green(), name.cyan());
    Ok(())
}
