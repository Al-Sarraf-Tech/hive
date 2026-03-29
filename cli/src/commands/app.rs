use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::{
    GetAppRequest, InstallAppRequest, ListAppsRequest, SearchAppsRequest,
};

pub async fn list(category: Option<&str>, addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;
    let resp = client
        .list_apps(ListAppsRequest {
            category: category.unwrap_or("").into(),
        })
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    if resp.apps.is_empty() {
        println!("No apps available.");
        return Ok(());
    }

    println!(
        "{:<4} {:<15} {:<25} {:<12} DESCRIPTION",
        "ICON", "ID", "NAME", "CATEGORY"
    );
    for app in &resp.apps {
        let desc = if app.description.len() > 40 {
            format!("{}...", &app.description[..37])
        } else {
            app.description.clone()
        };
        println!(
            "{:<4} {:<15} {:<25} {:<12} {}",
            app.icon, app.id, app.name, app.category, desc
        );
    }

    Ok(())
}

pub async fn search(query: &str, addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;
    let resp = client
        .search_apps(SearchAppsRequest {
            query: query.into(),
        })
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    if resp.apps.is_empty() {
        println!("No apps match '{}'.", query);
        return Ok(());
    }

    println!("{:<4} {:<15} {:<25} DESCRIPTION", "ICON", "ID", "NAME");
    for app in &resp.apps {
        println!(
            "{:<4} {:<15} {:<25} {}",
            app.icon, app.id, app.name, app.description
        );
    }

    Ok(())
}

pub async fn info(id: &str, addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;
    let app = client
        .get_app(GetAppRequest { id: id.into() })
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    println!(
        "{} {} {}",
        app.icon,
        app.name.bold(),
        format!("({})", app.id).dimmed()
    );
    println!("  {}", app.description);
    println!("  Image:    {}", app.image.cyan());
    println!("  Category: {}", app.category);
    if !app.tags.is_empty() {
        println!("  Tags:     {}", app.tags.join(", "));
    }
    if !app.min_memory.is_empty() {
        println!("  Min RAM:  {}", app.min_memory);
    }

    if !app.config_fields.is_empty() {
        println!("\n  Configuration:");
        for f in &app.config_fields {
            let req = if f.required { " (required)" } else { "" };
            let def = if f.default_value.is_empty() {
                String::new()
            } else {
                format!(" [default: {}]", f.default_value)
            };
            println!(
                "    {:<20} {}{}{}",
                f.key.yellow(),
                f.label,
                req.red(),
                def.dimmed()
            );
        }
    }

    println!(
        "\n  Install: hive app install {} --config key=value",
        app.id.cyan()
    );

    Ok(())
}

pub async fn install(
    id: &str,
    name: Option<&str>,
    config: &[String],
    addr: &str,
    ca_cert: Option<&str>,
) -> Result<()> {
    let mut config_map = std::collections::HashMap::new();
    for kv in config {
        let (k, v) = kv.split_once('=').ok_or_else(|| {
            anyhow::anyhow!("invalid config format '{}' — expected KEY=VALUE", kv)
        })?;
        config_map.insert(k.to_string(), v.to_string());
    }

    let service_name = name.unwrap_or(id);
    println!("Installing {} as {}...", id.cyan(), service_name.bold());

    let mut client = grpc_client::connect(addr, ca_cert).await?;
    let resp = client
        .install_app(InstallAppRequest {
            app_id: id.into(),
            service_name: service_name.into(),
            config: config_map,
        })
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    for svc in &resp.services {
        println!(
            "{} {} installed (image: {}, id: {})",
            "✓".green(),
            svc.name.bold(),
            svc.image.cyan(),
            &svc.id[..12]
        );
    }

    Ok(())
}

pub async fn installed(addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;
    let resp = client
        .list_installed_apps(())
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    if resp.apps.is_empty() {
        println!("No apps installed from the catalog.");
        return Ok(());
    }

    println!("{:<20} {:<20}", "APP", "SERVICE");
    for app in &resp.apps {
        println!("{:<20} {:<20}", app.app_id, app.service_name);
    }

    Ok(())
}
