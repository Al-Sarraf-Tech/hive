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

    /// Path to CA certificate for TLS connections (or set HIVE_CA_CERT env var)
    #[arg(long, global = true, env = "HIVE_CA_CERT")]
    ca_cert: Option<String>,
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
        /// Addresses of existing nodes (gossip host:port), or the IP of the init node when using --code
        addresses: Vec<String>,

        /// Cluster join token (from 'hive init' output)
        #[arg(long)]
        token: Option<String>,

        /// Short join code (from 'hive init' output, e.g. HIVE-AB12-CD34)
        #[arg(long)]
        code: Option<String>,
    },

    /// List cluster nodes
    Nodes,

    /// Deploy services from a Hivefile
    Deploy {
        /// Path to Hivefile (TOML)
        file: String,
    },

    /// Preview what a deploy would change (dry-run diff)
    Diff {
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

    /// Restart all replicas of a service (rolling restart)
    Restart {
        /// Service name
        service: String,
    },

    /// Execute a command in a service container
    Exec {
        /// Service name
        service: String,
        /// Command to execute
        #[arg(trailing_var_arg = true, required = true)]
        command: Vec<String>,
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

    /// List cron jobs
    Cron,

    /// Launch the TUI dashboard
    Top,

    /// Validate a Hivefile without deploying
    Validate {
        /// Path to Hivefile (TOML)
        file: String,
        /// Also check cluster state (secrets, nodes, ports)
        #[arg(long)]
        server: bool,
    },

    /// Export cluster state to a backup file
    Backup {
        /// Output file path (default: hive-backup.json)
        #[arg(short, long, default_value = "hive-backup.json")]
        output: String,
    },

    /// Restore cluster state from a backup file
    Restore {
        /// Path to backup file
        file: String,
        /// Overwrite existing keys (default: skip existing)
        #[arg(long)]
        overwrite: bool,
    },

    /// Set up Hive on this machine (install Docker, init/join cluster, start daemon)
    Setup {
        /// Join an existing cluster with this code (e.g., HIVE-AB12-CD34)
        #[arg(long)]
        join: Option<String>,
        /// Cluster name (for init mode)
        #[arg(long)]
        name: Option<String>,
        /// Accept all defaults without prompting
        #[arg(long, short)]
        yes: bool,
    },
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
        Commands::Init { name } => {
            commands::init::run(&name, &cli.addr, cli.ca_cert.as_deref()).await
        }
        Commands::Join {
            addresses,
            token,
            code,
        } => {
            if let Some(code) = code {
                // Join via short code: first address is the init node IP/host
                let host = addresses.first().map(|s| s.as_str()).unwrap_or("127.0.0.1");
                commands::join::run_with_code(&code, host, &cli.addr, cli.ca_cert.as_deref()).await
            } else {
                let token = token.ok_or_else(|| {
                    anyhow::anyhow!("either --token or --code is required for join")
                })?;
                if addresses.is_empty() {
                    anyhow::bail!("at least one seed address is required when using --token");
                }
                commands::join::run(&addresses, &token, &cli.addr, cli.ca_cert.as_deref()).await
            }
        }
        Commands::Nodes => commands::nodes::run(&cli.addr, cli.ca_cert.as_deref()).await,
        Commands::Deploy { file } => {
            commands::deploy::run(&file, &cli.addr, cli.ca_cert.as_deref()).await
        }
        Commands::Diff { file } => {
            commands::diff::run(&file, &cli.addr, cli.ca_cert.as_deref()).await
        }
        Commands::Ps => commands::ps::run(&cli.addr, cli.ca_cert.as_deref()).await,
        Commands::Logs {
            service,
            follow,
            tail,
        } => commands::logs::run(&service, follow, tail, &cli.addr, cli.ca_cert.as_deref()).await,
        Commands::Stop { service } => {
            commands::stop::run(&service, &cli.addr, cli.ca_cert.as_deref()).await
        }
        Commands::Scale { service, replicas } => {
            commands::scale::run(&service, replicas, &cli.addr, cli.ca_cert.as_deref()).await
        }
        Commands::Rollback { service } => {
            commands::rollback::run(&service, &cli.addr, cli.ca_cert.as_deref()).await
        }
        Commands::Restart { service } => {
            commands::restart::run(&service, &cli.addr, cli.ca_cert.as_deref()).await
        }
        Commands::Exec { service, command } => {
            commands::exec::run(&service, &command, &cli.addr, cli.ca_cert.as_deref()).await
        }
        Commands::Secret { action } => match action {
            SecretAction::Set { key, value } => {
                commands::secret::set(&key, value.as_deref(), &cli.addr, cli.ca_cert.as_deref())
                    .await
            }
            SecretAction::Ls => commands::secret::list(&cli.addr, cli.ca_cert.as_deref()).await,
            SecretAction::Rm { key } => {
                commands::secret::remove(&key, &cli.addr, cli.ca_cert.as_deref()).await
            }
        },
        Commands::Status => commands::status::run(&cli.addr, cli.ca_cert.as_deref()).await,
        Commands::Cron => commands::cron::list(&cli.addr, cli.ca_cert.as_deref()).await,
        Commands::Daemon { action } => match action {
            DaemonAction::Install => commands::daemon::install(),
            DaemonAction::Start => commands::daemon::start(),
            DaemonAction::Stop => commands::daemon::stop(),
            DaemonAction::Status => commands::daemon::status(),
        },
        Commands::Top => commands::top::run(&cli.addr, cli.ca_cert.as_deref()),
        Commands::Validate { file, server } => {
            commands::validate::run(&file, server, &cli.addr, cli.ca_cert.as_deref()).await
        }
        Commands::Backup { output } => {
            commands::backup::backup(&output, &cli.addr, cli.ca_cert.as_deref()).await
        }
        Commands::Restore { file, overwrite } => {
            commands::backup::restore(&file, overwrite, &cli.addr, cli.ca_cert.as_deref()).await
        }
        Commands::Setup { join, name, yes } => {
            commands::setup::run(
                join.as_deref(),
                name.as_deref(),
                yes,
                &cli.addr,
                cli.ca_cert.as_deref(),
            )
            .await
        }
    }
}
