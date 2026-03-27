use anyhow::Result;
use clap::{Parser, Subcommand};

mod commands;
mod grpc_client;

#[derive(Parser)]
#[command(name = "hive")]
#[command(about = "Lightweight cross-platform container orchestrator")]
#[command(version)]
struct Cli {
    #[command(subcommand)]
    command: Commands,

    /// hived gRPC address (or set HIVE_ADDR env var)
    #[arg(long, default_value_t = default_addr(), global = true)]
    addr: String,
}

fn default_addr() -> String {
    std::env::var("HIVE_ADDR").unwrap_or_else(|_| "127.0.0.1:7947".into())
}

#[derive(Subcommand)]
enum Commands {
    /// Initialize a new Hive cluster
    Init {
        /// Cluster name (default: "hive")
        #[arg(long, default_value = "hive")]
        name: String,
    },

    /// Join an existing cluster
    Join {
        /// Addresses of existing nodes (gossip host:port)
        #[arg(required = true)]
        addresses: Vec<String>,
    },

    /// List cluster nodes
    Nodes,

    /// Deploy services from a Hivefile
    Deploy {
        /// Path to Hivefile (TOML)
        file: String,
    },

    /// List running services
    Ps,

    /// Stream logs from a service
    Logs {
        /// Service name
        service: String,

        /// Follow log output
        #[arg(short, long)]
        follow: bool,

        /// Number of lines to show from the end
        #[arg(short = 'n', long, default_value = "100")]
        tail: u32,
    },

    /// Stop a service
    Stop {
        /// Service name
        service: String,
    },

    /// Scale a service
    Scale {
        /// Service name
        service: String,

        /// Number of replicas
        replicas: u32,
    },

    /// Rollback a service to the previous version
    Rollback {
        /// Service name
        service: String,
    },

    /// Manage secrets
    Secret {
        #[command(subcommand)]
        action: SecretAction,
    },

    /// Show cluster status
    Status,

    /// Manage the hived daemon
    Daemon {
        #[command(subcommand)]
        action: DaemonAction,
    },

    /// Launch the TUI dashboard
    Top,
}

#[derive(Subcommand)]
enum SecretAction {
    /// Set a secret value (reads value from stdin if not provided)
    Set {
        /// Secret key
        key: String,
        /// Secret value (omit to read from stdin)
        value: Option<String>,
    },
    /// List all secrets
    Ls,
    /// Delete a secret
    Rm {
        /// Secret key
        key: String,
    },
}

#[derive(Subcommand)]
enum DaemonAction {
    /// Install hived as a system service
    Install,
    /// Start the hived service
    Start,
    /// Stop the hived service
    Stop,
    /// Show hived service status
    Status,
}

#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();

    match cli.command {
        Commands::Init { name } => commands::init::run(&name, &cli.addr).await,
        Commands::Join { addresses } => commands::join::run(&addresses, &cli.addr).await,
        Commands::Nodes => commands::nodes::run(&cli.addr).await,
        Commands::Deploy { file } => commands::deploy::run(&file, &cli.addr).await,
        Commands::Ps => commands::ps::run(&cli.addr).await,
        Commands::Logs {
            service,
            follow,
            tail,
        } => commands::logs::run(&service, follow, tail, &cli.addr).await,
        Commands::Stop { service } => commands::stop::run(&service, &cli.addr).await,
        Commands::Scale { service, replicas } => {
            commands::scale::run(&service, replicas, &cli.addr).await
        }
        Commands::Rollback { service } => commands::rollback::run(&service, &cli.addr).await,
        Commands::Secret { action } => match action {
            SecretAction::Set { key, value } => {
                commands::secret::set(&key, value.as_deref(), &cli.addr).await
            }
            SecretAction::Ls => commands::secret::list(&cli.addr).await,
            SecretAction::Rm { key } => commands::secret::remove(&key, &cli.addr).await,
        },
        Commands::Status => commands::status::run(&cli.addr).await,
        Commands::Daemon { action } => match action {
            DaemonAction::Install => commands::daemon::install(),
            DaemonAction::Start => commands::daemon::start(),
            DaemonAction::Stop => commands::daemon::stop(),
            DaemonAction::Status => commands::daemon::status(),
        },
        Commands::Top => commands::top::run(&cli.addr),
    }
}
