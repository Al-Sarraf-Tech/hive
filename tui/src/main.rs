use anyhow::Result;
use clap::Parser;
use crossterm::{
    ExecutableCommand,
    event::{self, Event, KeyCode, KeyEventKind, KeyModifiers},
    terminal::{EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode},
};
use ratatui::prelude::*;
use std::io::stdout;
use std::time::Duration;
use tokio::sync::mpsc;

mod app;
mod grpc_client;
mod views;

#[derive(Parser)]
#[command(name = "hivetop")]
#[command(about = "TUI dashboard for the Hive container orchestrator")]
#[command(version)]
struct Cli {
    /// hived gRPC address (or set HIVE_ADDR env var)
    #[arg(long, default_value_t = default_addr())]
    addr: String,

    /// Refresh interval in seconds (minimum 1)
    #[arg(long, default_value = "2")]
    refresh: u64,

    /// Path to CA certificate for TLS connections (or set HIVE_CA_CERT env var)
    #[arg(long, env = "HIVE_CA_CERT")]
    ca_cert: Option<String>,
}

fn default_addr() -> String {
    std::env::var("HIVE_ADDR").unwrap_or_else(|_| "127.0.0.1:7947".into())
}

#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();

    let refresh = cli.refresh.max(1); // enforce minimum 1 second to prevent busy loop

    // Restore terminal on panic so the user's shell isn't left corrupted
    let original_hook = std::panic::take_hook();
    std::panic::set_hook(Box::new(move |info| {
        let _ = disable_raw_mode();
        let _ = stdout().execute(LeaveAlternateScreen);
        original_hook(info);
    }));

    enable_raw_mode()?;
    stdout().execute(EnterAlternateScreen)?;
    let mut terminal = Terminal::new(CrosstermBackend::new(stdout()))?;

    let result = run(&mut terminal, &cli.addr, refresh, cli.ca_cert.as_deref()).await;

    disable_raw_mode()?;
    stdout().execute(LeaveAlternateScreen)?;

    result
}

async fn run(
    terminal: &mut Terminal<CrosstermBackend<std::io::Stdout>>,
    addr: &str,
    refresh_secs: u64,
    ca_cert: Option<&str>,
) -> Result<()> {
    let mut app = app::App::new(addr.to_string());

    // Channel for async data updates
    let (tx, mut rx) = mpsc::channel(16);

    // Spawn background data fetcher — reuses a single gRPC connection
    let fetch_addr = addr.to_string();
    let fetch_ca = ca_cert.map(|s| s.to_string());
    let fetch_tx = tx;
    tokio::spawn(async move {
        let mut interval = tokio::time::interval(Duration::from_secs(refresh_secs));
        let mut client: Option<grpc_client::HiveApiClient<tonic::transport::Channel>> = None;
        loop {
            interval.tick().await;
            let data = fetch_cluster_data(&fetch_addr, fetch_ca.as_deref(), &mut client).await;
            if fetch_tx.send(data).await.is_err() {
                break;
            }
        }
    });

    // Channel for event stream
    let (log_tx, mut log_rx) = mpsc::channel::<String>(64);

    // Spawn event stream listener
    let stream_addr = addr.to_string();
    let stream_ca = ca_cert.map(|s| s.to_string());
    tokio::spawn(async move {
        loop {
            // Connect and stream events — reconnect on failure
            if let Ok(mut client) = grpc_client::connect(&stream_addr, stream_ca.as_deref()).await {
                if let Ok(response) = client.stream_events(()).await {
                    let mut stream = response.into_inner();
                    while let Ok(Some(event)) = stream.message().await {
                        let line = format!(
                            "{} [{}] {}",
                            chrono_timestamp(),
                            event.source,
                            event.message
                        );
                        if log_tx.send(line).await.is_err() {
                            return; // receiver dropped
                        }
                    }
                }
            }
            // Wait before reconnecting
            tokio::time::sleep(Duration::from_secs(5)).await;
        }
    });

    loop {
        // Process any pending data updates
        while let Ok(data) = rx.try_recv() {
            app.update_data(data);
        }

        // Process any pending log events
        while let Ok(line) = log_rx.try_recv() {
            app.push_log(line);
        }

        terminal.draw(|frame| app.draw(frame))?;

        // Use spawn_blocking to avoid blocking the tokio executor with crossterm's sync polling
        let key_event = tokio::task::spawn_blocking(|| {
            if event::poll(Duration::from_millis(100)).unwrap_or(false) {
                if let Ok(Event::Key(key)) = event::read() {
                    if key.kind == KeyEventKind::Press {
                        return Some(key);
                    }
                }
            }
            None
        })
        .await
        .unwrap_or(None);

        if let Some(key) = key_event {
            match key.code {
                KeyCode::Char('q') | KeyCode::Esc => break,
                KeyCode::Char('c') if key.modifiers.contains(KeyModifiers::CONTROL) => break,
                KeyCode::Char('1') => app.tab = app::Tab::Overview,
                KeyCode::Char('2') => app.tab = app::Tab::Nodes,
                KeyCode::Char('3') => app.tab = app::Tab::Services,
                KeyCode::Char('4') => app.tab = app::Tab::Logs,
                _ => {}
            }
        }
    }

    Ok(())
}

async fn fetch_cluster_data(
    addr: &str,
    ca_cert: Option<&str>,
    client: &mut Option<grpc_client::HiveApiClient<tonic::transport::Channel>>,
) -> app::ClusterData {
    // Reuse existing connection, reconnect on failure
    if client.is_none() {
        match grpc_client::connect(addr, ca_cert).await {
            Ok(c) => *client = Some(c),
            Err(e) => {
                return app::ClusterData {
                    connected: false,
                    status: None,
                    services: None,
                    nodes: None,
                    error: Some(e.to_string()),
                };
            }
        }
    }

    let c = client.as_ref().unwrap();

    // Fetch all three concurrently — clone the client (cheap: wraps a Channel)
    let mut c1 = c.clone();
    let mut c2 = c.clone();
    let mut c3 = c.clone();
    let (status_res, services_res, nodes_res) = tokio::join!(
        c1.get_cluster_status(()),
        c2.list_services(()),
        c3.list_nodes(()),
    );

    // If all three failed, connection is probably dead — reset for next cycle
    if status_res.is_err() && services_res.is_err() && nodes_res.is_err() {
        let err_msg = status_res
            .as_ref()
            .err()
            .map(|e| e.to_string())
            .unwrap_or_default();
        *client = None;
        return app::ClusterData {
            connected: false,
            status: None,
            services: None,
            nodes: None,
            error: Some(err_msg),
        };
    }

    app::ClusterData {
        connected: true,
        status: status_res.ok().map(|r| r.into_inner()),
        services: services_res.ok().map(|r| r.into_inner()),
        nodes: nodes_res.ok().map(|r| r.into_inner()),
        error: None,
    }
}

fn chrono_timestamp() -> String {
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default();
    let secs = now.as_secs();
    let hours = (secs % 86400) / 3600;
    let mins = (secs % 3600) / 60;
    let s = secs % 60;
    format!("{hours:02}:{mins:02}:{s:02}Z")
}
