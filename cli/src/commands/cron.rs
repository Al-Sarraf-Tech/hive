use anyhow::Result;
use tabled::{Table, Tabled};

use crate::grpc_client;

#[derive(Tabled)]
struct CronRow {
    #[tabled(rename = "NAME")]
    name: String,
    #[tabled(rename = "SERVICE")]
    service: String,
    #[tabled(rename = "SCHEDULE")]
    schedule: String,
    #[tabled(rename = "NEXT RUN")]
    next_run: String,
    #[tabled(rename = "LAST RUN")]
    last_run: String,
}

pub async fn list(addr: &str, ca_cert: Option<&str>) -> Result<()> {
    let mut client = grpc_client::connect(addr, ca_cert).await?;
    let resp = client
        .list_cron_jobs(())
        .await
        .map_err(grpc_client::map_grpc_error)?
        .into_inner();

    if resp.jobs.is_empty() {
        println!("No cron jobs registered.");
        println!("Define cron jobs in your Hivefile under [[service.<name>.cron]]");
        return Ok(());
    }

    let rows: Vec<CronRow> = resp
        .jobs
        .iter()
        .map(|j| CronRow {
            name: j.name.clone(),
            service: j.service.clone(),
            schedule: j.schedule.clone(),
            next_run: j.next_run.clone(),
            last_run: if j.last_run.is_empty() {
                "-".into()
            } else {
                j.last_run.clone()
            },
        })
        .collect();

    let table = Table::new(&rows).to_string();
    println!("{table}");
    Ok(())
}
