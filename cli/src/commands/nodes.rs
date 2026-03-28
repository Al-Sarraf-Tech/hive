use anyhow::Result;
use tabled::{Table, Tabled};

use crate::grpc_client;
use crate::grpc_client::hive_proto::NodeStatus;

#[derive(Tabled)]
struct NodeRow {
    #[tabled(rename = "STATUS")]
    status: String,
    #[tabled(rename = "NAME")]
    name: String,
    #[tabled(rename = "OS")]
    os: String,
    #[tabled(rename = "ARCH")]
    arch: String,
    #[tabled(rename = "RUNTIME")]
    runtime: String,
    #[tabled(rename = "PLATFORMS")]
    platforms: String,
}

pub async fn run(addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;

    let resp = client
        .list_nodes(())
        .await
        .map_err(crate::grpc_client::map_grpc_error)?
        .into_inner();

    if resp.nodes.is_empty() {
        println!("No nodes in cluster.");
        println!("Start hived on a machine to register it as a node.");
        return Ok(());
    }

    let rows: Vec<NodeRow> = resp
        .nodes
        .iter()
        .map(|n| {
            let caps = n.capabilities.as_ref();
            NodeRow {
                status: match NodeStatus::try_from(n.status) {
                    Ok(NodeStatus::Ready) => "● ready".into(),
                    Ok(NodeStatus::Draining) => "◐ draining".into(),
                    Ok(NodeStatus::Down) => "○ down".into(),
                    _ => "? unknown".into(),
                },
                name: n.name.clone(),
                os: caps.map(|c| c.os.clone()).unwrap_or_default(),
                arch: caps.map(|c| c.arch.clone()).unwrap_or_default(),
                runtime: caps
                    .map(|c| c.container_runtime.clone())
                    .unwrap_or_default(),
                platforms: caps.map(|c| c.platforms.join(", ")).unwrap_or_default(),
            }
        })
        .collect();

    let table = Table::new(&rows).to_string();
    println!("{table}");
    Ok(())
}
