use anyhow::{Context, Result};
use std::time::Duration;
use tonic::transport::Channel;

pub mod hive_proto {
    tonic::include_proto!("hive.v1");
}

pub use hive_proto::hive_api_client::HiveApiClient;

/// Connect to hived, with optional TLS when a CA cert path is provided.
pub async fn connect(addr: &str, ca_cert: Option<&str>) -> Result<HiveApiClient<Channel>> {
    let use_tls = ca_cert.is_some() || addr.starts_with("https");

    let url = if addr.starts_with("http") {
        addr.to_string()
    } else if use_tls {
        format!("https://{addr}")
    } else {
        format!("http://{addr}")
    };

    let mut endpoint = Channel::builder(url.parse().context("invalid address format")?)
        .connect_timeout(Duration::from_secs(5))
        .timeout(Duration::from_secs(30));

    if let Some(ca_path) = ca_cert {
        let pem = tokio::fs::read(ca_path)
            .await
            .with_context(|| format!("failed to read CA cert: {ca_path}"))?;
        let ca = tonic::transport::Certificate::from_pem(pem);
        let tls_config = tonic::transport::ClientTlsConfig::new().ca_certificate(ca);
        endpoint = endpoint
            .tls_config(tls_config)
            .context("invalid TLS configuration")?;
    }

    let channel = endpoint
        .connect()
        .await
        .context("failed to connect to hived — is it running?")?;

    Ok(HiveApiClient::new(channel))
}
