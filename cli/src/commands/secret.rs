use anyhow::{Context, Result};
use colored::Colorize;
use std::io::{self, BufRead, IsTerminal};

use crate::grpc_client;
use crate::grpc_client::hive_proto::{DeleteSecretRequest, SetSecretRequest};

pub async fn set(key: &str, value: Option<&str>, addr: &str) -> Result<()> {
    let secret_value = match value {
        Some(v) => v.to_string(),
        None => {
            // Read from stdin to avoid exposing secrets in ps/shell history
            if io::stdin().is_terminal() {
                eprintln!("Enter secret value (then press Enter):");
            }
            let mut line = String::new();
            io::stdin()
                .lock()
                .read_line(&mut line)
                .context("failed to read secret from stdin")?;
            line.trim_end_matches('\n')
                .trim_end_matches('\r')
                .to_string()
        }
    };

    let mut client = grpc_client::connect(addr).await?;
    client
        .set_secret(SetSecretRequest {
            key: key.into(),
            value: secret_value.as_bytes().to_vec(),
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
