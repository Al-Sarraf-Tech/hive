use anyhow::{Context, Result, anyhow};
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

/// Maps a tonic gRPC Status to a user-friendly error message.
pub fn map_grpc_error(status: tonic::Status) -> anyhow::Error {
    match status.code() {
        tonic::Code::Unavailable => anyhow!("hived is not running or unreachable"),
        tonic::Code::NotFound => anyhow!("{}", status.message()),
        tonic::Code::InvalidArgument => anyhow!("invalid input: {}", status.message()),
        tonic::Code::Unimplemented => anyhow!("not yet implemented: {}", status.message()),
        tonic::Code::FailedPrecondition => anyhow!("{}", status.message()),
        tonic::Code::Internal => anyhow!("server error: {}", status.message()),
        tonic::Code::Cancelled => anyhow!("request cancelled"),
        _ => anyhow!("rpc error ({}): {}", status.code(), status.message()),
    }
}

/// Safely truncates a string ID for display. Always returns valid UTF-8.
pub fn short_id(id: &str) -> &str {
    let mut end = id.len().min(12);
    while end > 0 && !id.is_char_boundary(end) {
        end -= 1;
    }
    &id[..end]
}
