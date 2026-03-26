use anyhow::{Context, Result};
use std::time::Duration;
use tonic::transport::Channel;

pub mod hive_proto {
    tonic::include_proto!("hive.v1");
}

pub use hive_proto::hive_api_client::HiveApiClient;

pub async fn connect(addr: &str) -> Result<HiveApiClient<Channel>> {
    let url = if addr.starts_with("http") {
        addr.to_string()
    } else {
        format!("http://{addr}")
    };

    let endpoint = Channel::builder(url.parse().context("invalid address format")?)
        .connect_timeout(Duration::from_secs(5))
        .timeout(Duration::from_secs(30));

    let channel = endpoint
        .connect()
        .await
        .context("failed to connect to hived — is it running?")?;

    Ok(HiveApiClient::new(channel))
}
