use std::io::Write;

use anyhow::Result;

use crate::grpc_client;
use crate::grpc_client::hive_proto::ExecContainerRequest;

pub async fn run(
    service: &str,
    command: &[String],
    addr: &str,
    ca_cert: Option<&str>,
) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;

    let result = client
        .exec_container(ExecContainerRequest {
            container_id: String::new(),
            service_name: service.to_string(),
            command: command.to_vec(),
        })
        .await;

    match result {
        Ok(response) => {
            let resp = response.into_inner();

            if !resp.stdout.is_empty() {
                print!("{}", resp.stdout);
            }
            if !resp.stderr.is_empty() {
                eprint!("{}", resp.stderr);
            }
            // Flush buffered output from print!/eprint! (no trailing newline = not auto-flushed)
            let _ = std::io::stdout().flush();
            let _ = std::io::stderr().flush();

            if resp.exit_code != 0 {
                anyhow::bail!("command exited with code {}", resp.exit_code);
            }
        }
        Err(status) => {
            // Even on gRPC-level errors (e.g., truncation, deadline exceeded),
            // print any partial output the server attached as error details.
            let msg = status.message();
            if !msg.is_empty() {
                eprint!("{}", msg);
            }
            let _ = std::io::stderr().flush();

            return Err(grpc_client::map_grpc_error(status));
        }
    }

    Ok(())
}
