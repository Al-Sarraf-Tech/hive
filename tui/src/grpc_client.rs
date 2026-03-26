use std::time::Duration;
use tonic::transport::Channel;

pub mod hive_proto {
    tonic::include_proto!("hive.v1");
}

pub use hive_proto::hive_api_client::HiveApiClient;

pub async fn connect(
    addr: &str,
) -> Result<HiveApiClient<Channel>, Box<dyn std::error::Error + Send + Sync>> {
    let url = if addr.starts_with("http") {
        addr.to_string()
    } else {
        format!("http://{addr}")
    };
    let endpoint = Channel::builder(url.parse()?)
        .connect_timeout(Duration::from_secs(3))
        .timeout(Duration::from_secs(10));
    let channel = endpoint.connect().await?;
    Ok(HiveApiClient::new(channel))
}
