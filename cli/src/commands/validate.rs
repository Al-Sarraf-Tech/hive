use anyhow::{Context, Result};
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::{ValidateHivefileRequest, ValidationSeverity};

pub async fn run(file: &str, server_checks: bool, addr: &str, ca_cert: Option<&str>) -> Result<()> {
    // 1. Read the file
    let content = std::fs::read_to_string(file)
        .with_context(|| format!("failed to read Hivefile: {file}"))?;

    // 2. Quick local TOML check
    let _: toml::Value =
        toml::from_str(&content).with_context(|| format!("invalid TOML in {file}"))?;

    // 3. Connect and call ValidateHivefile RPC
    let mut client = grpc_client::connect(addr, ca_cert).await?;
    let resp = client
        .validate_hivefile(ValidateHivefileRequest {
            hivefile_toml: content,
            server_checks,
        })
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    // 4. Display results
    if resp.issues.is_empty() {
        println!("{} Hivefile is valid", "\u{2713}".green().bold());
        return Ok(());
    }

    let mut errors = 0;
    let mut warnings = 0;

    for issue in &resp.issues {
        let severity =
            ValidationSeverity::try_from(issue.severity).unwrap_or(ValidationSeverity::Unspecified);
        let (icon, label) = match severity {
            ValidationSeverity::Error => {
                errors += 1;
                ("\u{2717}".red().bold(), "error".red())
            }
            ValidationSeverity::Warning => {
                warnings += 1;
                ("!".yellow().bold(), "warning".yellow())
            }
            ValidationSeverity::Info => ("\u{00b7}".dimmed(), "info".dimmed()),
            _ => ("?".dimmed(), "unknown".dimmed()),
        };

        let location = if !issue.service.is_empty() && !issue.field.is_empty() {
            format!("{}.{}", issue.service, issue.field)
        } else if !issue.service.is_empty() {
            issue.service.clone()
        } else if !issue.field.is_empty() {
            issue.field.clone()
        } else {
            String::new()
        };

        if location.is_empty() {
            println!("  {} {}: {}", icon, label, issue.message);
        } else {
            println!(
                "  {} {} [{}]: {}",
                icon,
                label,
                location.dimmed(),
                issue.message
            );
        }
    }

    println!();
    if errors > 0 {
        println!(
            "{} {} error(s), {} warning(s)",
            "\u{2717}".red().bold(),
            errors,
            warnings
        );
        anyhow::bail!("validation failed with {} error(s)", errors);
    } else {
        println!(
            "{} {} warning(s), no errors",
            "\u{2713}".green().bold(),
            warnings
        );
    }

    Ok(())
}
