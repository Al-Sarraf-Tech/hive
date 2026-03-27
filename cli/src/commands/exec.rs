use anyhow::Result;
use colored::Colorize;

use crate::grpc_client;
use crate::grpc_client::hive_proto::ExecContainerRequest;

pub async fn run(
    service: &str,
    command: &[String],
    addr: &str,
    ca_cert: Option<&str>,
) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;

    let resp = client
        .exec_container(ExecContainerRequest {
            container_id: String::new(),
            service_name: service.to_string(),
            command: command.to_vec(),
        })
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    if !resp.stdout.is_empty() {
        print!("{}", resp.stdout);
    }
    if !resp.stderr.is_empty() {
        eprint!("{}", resp.stderr);
    }

    if resp.exit_code != 0 {
        eprintln!(
            "{} command exited with code {}",
            "!".yellow(),
            resp.exit_code
        );
        std::process::exit(resp.exit_code);
    }

    Ok(())
}
