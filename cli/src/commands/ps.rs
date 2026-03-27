use anyhow::Result;
use tabled::{Table, Tabled};

use crate::grpc_client;

#[derive(Tabled)]
struct ServiceRow {
    #[tabled(rename = "NAME")]
    name: String,
    #[tabled(rename = "IMAGE")]
    image: String,
    #[tabled(rename = "REPLICAS")]
    replicas: String,
    #[tabled(rename = "STATUS")]
    status: String,
    #[tabled(rename = "NODE")]
    node: String,
}

pub async fn run(addr: &str) -> Result<()> {
    let mut client = grpc_client::connect(addr).await?;

    let resp = client
        .list_services(())
        .await
        .map_err(crate::grpc_client::map_grpc_error)?
        .into_inner();

    if resp.services.is_empty() {
        println!("No services running.");
        println!("Deploy with: hive deploy <hivefile.toml>");
        return Ok(());
    }

    let rows: Vec<ServiceRow> = resp
        .services
        .iter()
        .map(|s| ServiceRow {
            name: s.name.clone(),
            image: s.image.clone(),
            replicas: format!("{}/{}", s.replicas_running, s.replicas_desired),
            status: match s.status {
                1 => "running".into(),
                2 => "degraded".into(),
                3 => "stopped".into(),
                4 => "deploying".into(),
                5 => "rolling back".into(),
                _ => "unknown".into(),
            },
            node: s.node_constraint.clone(),
        })
        .collect();

    let table = Table::new(&rows).to_string();
    println!("{table}");
    Ok(())
}
