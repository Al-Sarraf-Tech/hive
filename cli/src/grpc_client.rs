use anyhow::{Context, Result, anyhow};
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
    // Walk back to the nearest valid UTF-8 char boundary
    while end > 0 && !id.is_char_boundary(end) {
        end -= 1;
    }
    &id[..end]
}
