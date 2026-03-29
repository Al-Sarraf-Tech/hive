use anyhow::{Context, Result};
use colored::Colorize;
use std::io::{self, BufRead, IsTerminal};

use crate::grpc_client;
use crate::grpc_client::hive_proto::{RegistryLoginRequest, RemoveRegistryRequest};

pub async fn login(
    url: &str,
    username: Option<&str>,
    addr: &str,
    ca_cert: Option<&str>,
) -> Result<()> {
    let user = match username {
        Some(u) => u.to_string(),
        None => {
            eprint!("Username: ");
            let mut line = String::new();
            io::stdin()
                .lock()
                .read_line(&mut line)
                .context("failed to read username")?;
            line.trim().to_string()
        }
    };

    // Read password from stdin
    if io::stdin().is_terminal() {
        eprint!("Password: ");
    }
    let mut password = String::new();
    io::stdin()
        .lock()
        .read_line(&mut password)
        .context("failed to read password")?;
    let password = password.trim().to_string();

    if password.is_empty() {
        anyhow::bail!("password cannot be empty");
    }

    let mut client = grpc_client::connect(addr, ca_cert).await?;
    client
        .registry_login(RegistryLoginRequest {
            url: url.into(),
            username: user.clone(),
            password,
        })
        .await
        .map_err(grpc_client::map_grpc_error)?;

    println!(
        "{} Logged in to {} as {}",
        "✓".green(),
        url.cyan(),
        user.bold()
    );
    Ok(())
}

pub async fn list(addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;
    let resp = client
        .list_registries(())
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    if resp.registries.is_empty() {
        println!("No registries configured.");
        return Ok(());
    }

    println!("{:<40} {:<20}", "URL", "USERNAME");
    for r in &resp.registries {
        println!("{:<40} {:<20}", r.url, r.username);
    }

    Ok(())
}

pub async fn remove(url: &str, addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;
    client
        .remove_registry(RemoveRegistryRequest { url: url.into() })
        .await
        .map_err(grpc_client::map_grpc_error)?;

    println!("{} Registry {} removed.", "✓".green(), url.cyan());
    Ok(())
}
