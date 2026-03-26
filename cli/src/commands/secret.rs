use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::{DeleteSecretRequest, SetSecretRequest};

pub async fn set(key: &str, value: &str, addr: &str) -> Result<()> {
    let mut client = grpc_client::connect(addr).await?;
    client
        .set_secret(SetSecretRequest {
            key: key.into(),
            value: value.as_bytes().to_vec(),
        })
        .await?;

    println!("{} Secret {} set.", "✓".green(), key.cyan());
    Ok(())
}

pub async fn list(addr: &str) -> Result<()> {
    let mut client = grpc_client::connect(addr).await?;
    let resp = client.list_secrets(()).await?.into_inner();

    if resp.secrets.is_empty() {
        println!("No secrets stored.");
    } else {
        println!("{:<30} {:<20}", "KEY", "CREATED");
        for s in &resp.secrets {
            println!("{:<30} {:<20}", s.key, "-");
        }
    }

    Ok(())
}

pub async fn remove(key: &str, addr: &str) -> Result<()> {
    let mut client = grpc_client::connect(addr).await?;
    client
        .delete_secret(DeleteSecretRequest { key: key.into() })
        .await?;

    println!("{} Secret {} removed.", "✓".green(), key.cyan());
    Ok(())
}
