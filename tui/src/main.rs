use anyhow::Result;
use clap::Parser;
use crossterm::{
    ExecutableCommand,
    event::{self, Event, KeyCode, KeyEventKind},
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
    /// hived gRPC address
    #[arg(long, default_value = "127.0.0.1:7947")]
    addr: String,

    /// Refresh interval in seconds
    #[arg(long, default_value = "2")]
    refresh: u64,
}

#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();

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

    let result = run(&mut terminal, &cli.addr, cli.refresh).await;

    disable_raw_mode()?;
    stdout().execute(LeaveAlternateScreen)?;

    result
}

async fn run(
    terminal: &mut Terminal<CrosstermBackend<std::io::Stdout>>,
    addr: &str,
    refresh_secs: u64,
) -> Result<()> {
    let mut app = app::App::new(addr.to_string());

    // Channel for async data updates
    let (tx, mut rx) = mpsc::channel(16);

    // Spawn background data fetcher — tokio interval fires immediately on first tick
    let fetch_addr = addr.to_string();
    let fetch_tx = tx;
    tokio::spawn(async move {
        let mut interval = tokio::time::interval(Duration::from_secs(refresh_secs));
        loop {
            interval.tick().await;
            let data = fetch_cluster_data(&fetch_addr).await;
            if fetch_tx.send(data).await.is_err() {
                break;
            }
        }
    });

    loop {
        // Process any pending data updates
        while let Ok(data) = rx.try_recv() {
            app.update_data(data);
        }

        terminal.draw(|frame| app.draw(frame))?;

        // Poll for keyboard input with 100ms timeout (keeps UI responsive)
        if event::poll(Duration::from_millis(100))? {
            if let Event::Key(key) = event::read()? {
                if key.kind != KeyEventKind::Press {
                    continue;
                }
                match key.code {
                    KeyCode::Char('q') | KeyCode::Esc => break,
                    KeyCode::Char('1') => app.tab = app::Tab::Overview,
                    KeyCode::Char('2') => app.tab = app::Tab::Nodes,
                    KeyCode::Char('3') => app.tab = app::Tab::Services,
                    KeyCode::Char('4') => app.tab = app::Tab::Logs,
                    _ => {}
                }
            }
        }
    }

    Ok(())
}

async fn fetch_cluster_data(addr: &str) -> app::ClusterData {
    let client = grpc_client::connect(addr).await;
    match client {
        Ok(mut c) => {
            let status = c.get_cluster_status(()).await.ok().map(|r| r.into_inner());
            let services = c.list_services(()).await.ok().map(|r| r.into_inner());
            let nodes = c.list_nodes(()).await.ok().map(|r| r.into_inner());
            app::ClusterData {
                connected: true,
                status,
                services,
                nodes,
                error: None,
            }
        }
        Err(e) => app::ClusterData {
            connected: false,
            status: None,
            services: None,
            nodes: None,
            error: Some(e.to_string()),
        },
    }
}
