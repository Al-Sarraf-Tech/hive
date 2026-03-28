use anyhow::{Context, Result};
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::{JoinClusterRequest, NodeStatus};

/// Bootstrap response from the HTTP API /api/v1/bootstrap/{code} endpoint.
#[derive(serde::Deserialize)]
struct BootstrapResponse {
    join_token: String,
    gossip_addr: String,
    #[allow(dead_code)]
    ca_cert_pem: Option<String>,
    #[allow(dead_code)]
    cluster_name: Option<String>,
}

/// Fetches join credentials from the init node using a short join code.
/// The bootstrap endpoint is unauthenticated HTTP on the daemon's HTTP port (default 7949).
async fn bootstrap(code: &str, host: &str) -> Result<BootstrapResponse> {
    let url = format!("http://{}:7949/api/v1/bootstrap/{}", host, code);
    let client = reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(10))
        .build()?;
    let resp = client
        .get(&url)
        .send()
        .await
        .with_context(|| format!("failed to reach bootstrap endpoint at {url}"))?;

    if !resp.status().is_success() {
        let status = resp.status();
        let body = resp.text().await.unwrap_or_default();
        anyhow::bail!(
            "bootstrap failed (HTTP {status}): {body}\n\
             Verify the join code and that hived is running on {host}:7949"
        );
    }

    resp.json::<BootstrapResponse>()
        .await
        .context("failed to parse bootstrap response")
}

pub async fn run(
    addresses: &[String],
    token: &str,
    addr: &str,
    ca_cert: Option<&str>,
) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;

    let resp = client
        .join_cluster(JoinClusterRequest {
            seed_addrs: addresses.to_vec(),
            join_token: token.to_string(),
        })
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    println!(
        "{} Joined cluster ({} nodes contacted)",
        "✓".green(),
        resp.nodes_joined
    );
    for node in &resp.nodes {
        let status = match NodeStatus::try_from(node.status) {
            Ok(NodeStatus::Ready) => "●".green(),
            Ok(NodeStatus::Draining) => "◐".yellow(),
            Ok(NodeStatus::Down) => "○".red(),
            _ => "?".dimmed(),
        };
        println!("  {} {}", status, node.name.bold());
    }
    Ok(())
}

/// Join using a short join code: fetches credentials via HTTP bootstrap, then joins via gRPC.
pub async fn run_with_code(
    code: &str,
    host: &str,
    addr: &str,
    ca_cert: Option<&str>,
) -> Result<()> {
    println!(
        "{} Exchanging join code with {}...",
        "⟳".cyan(),
        host.bold()
    );
    let bs = bootstrap(code, host).await?;

    println!(
        "{} Got credentials, joining via {}...",
        "✓".green(),
        bs.gossip_addr.cyan()
    );

    run(&[bs.gossip_addr], &bs.join_token, addr, ca_cert).await
}
