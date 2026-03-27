use anyhow::{Context, Result};
use colored::Colorize;
use std::io::{self, BufRead, IsTerminal};

use crate::grpc_client;
use crate::grpc_client::hive_proto::{DeleteSecretRequest, SetSecretRequest};

pub async fn set(key: &str, value: Option<&str>, addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let secret_value = match value {
        Some(v) => {
            eprintln!(
                "{} Passing secrets as CLI arguments is insecure (visible in process table).",
                "warning:".yellow()
            );
            eprintln!("         Prefer: echo SECRET | hive secret set {key}");
            v.to_string()
        }
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

    if secret_value.is_empty() {
        anyhow::bail!("secret value cannot be empty (received EOF or empty input)");
    }

    let mut client = grpc_client::connect(addr, ca_cert).await?;
    client
        .set_secret(SetSecretRequest {
            key: key.into(),
            value: secret_value.as_bytes().to_vec(),
        })
        .await
        .map_err(grpc_client::map_grpc_error)?;

    println!("{} Secret {} set.", "✓".green(), key.cyan());
    Ok(())
}

pub async fn list(addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;
    let resp = client
        .list_secrets(())
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    if resp.secrets.is_empty() {
        println!("No secrets stored.");
    } else {
        println!("{:<30} {:<20}", "KEY", "CREATED");
        for s in &resp.secrets {
            let created = if s.created_at_unix > 0 {
                // Format epoch seconds as ISO 8601 UTC
                let secs = s.created_at_unix as u64;
                let days = secs / 86400;
                let time_of_day = secs % 86400;
                let h = time_of_day / 3600;
                let m = (time_of_day % 3600) / 60;
                // Approximate date from Unix epoch (good enough for display)
                let (year, month, day) = epoch_days_to_ymd(days);
                format!("{year:04}-{month:02}-{day:02} {h:02}:{m:02}Z")
            } else {
                "-".to_string()
            };
            println!("{:<30} {:<20}", s.key, created);
        }
    }

    Ok(())
}

pub async fn remove(key: &str, addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;
    client
        .delete_secret(DeleteSecretRequest { key: key.into() })
        .await
        .map_err(grpc_client::map_grpc_error)?;

    println!("{} Secret {} removed.", "✓".green(), key.cyan());
    Ok(())
}

/// Convert days since Unix epoch to (year, month, day).
/// Uses the civil calendar algorithm from Howard Hinnant.
fn epoch_days_to_ymd(days: u64) -> (u64, u64, u64) {
    let z = days + 719468;
    let era = z / 146097;
    let doe = z - era * 146097;
    let yoe = (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365;
    let y = yoe + era * 400;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let d = doy - (153 * mp + 2) / 5 + 1;
    let m = if mp < 10 { mp + 3 } else { mp - 9 };
    let y = if m <= 2 { y + 1 } else { y };
    (y, m, d)
}
