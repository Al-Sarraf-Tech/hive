use anyhow::{Context, Result, bail};
use colored::Colorize;
use indicatif::{ProgressBar, ProgressStyle};
use std::process::Command;
use std::time::Duration;

pub async fn run(
    join_code: Option<&str>,
    name: Option<&str>,
    yes: bool,
    addr: &str,
    ca_cert: Option<&str>,
) -> Result<()> {
    println!("{}", "Hive Setup".bold());
    println!();

    // Step 1: Detect OS
    let os = std::env::consts::OS;
    let arch = std::env::consts::ARCH;
    println!("  {} Detected {} / {}", "✓".green(), os, arch);

    // Step 2: Check Docker/Podman
    let runtime = detect_runtime();
    let _runtime_name = match &runtime {
        Some(rt) => {
            println!("  {} Found {} ({})", "✓".green(), rt.name, rt.version);
            rt.name.clone()
        }
        None => {
            println!("  {} No container runtime found", "!".yellow());
            if os == "linux" {
                let install = if yes {
                    true
                } else {
                    confirm_prompt("Install a container runtime?")?
                };
                if install {
                    install_docker_linux()?
                } else {
                    bail!("Hive requires Docker or Podman. Install one and re-run setup.");
                }
            } else if os == "windows" {
                println!("  Install Docker Desktop: choco install docker-desktop -y");
                println!(
                    "  Or download from: https://docs.docker.com/desktop/install/windows-install/"
                );
                bail!("Install Docker Desktop and re-run setup.");
            } else {
                bail!("Unsupported OS: {os}. Install Docker or Podman manually and re-run setup.");
            }
        }
    };

    // Step 3: Install and start daemon (BEFORE init/join — the daemon must be running)
    if os == "linux" {
        let install_service = if yes {
            true
        } else {
            confirm_prompt("Install hived as a system service?")?
        };
        if install_service {
            println!("  {} Installing systemd service...", "⟳".cyan());
            if let Err(e) = crate::commands::daemon::install() {
                println!("  {} Service install failed: {e}", "!".yellow());
                println!("    You can install manually later: hive daemon install");
                println!("    Starting daemon directly for now...");
            } else {
                println!("  {} Service installed", "✓".green());
            }

            println!("  {} Starting hived...", "⟳".cyan());
            if let Err(e) = crate::commands::daemon::start() {
                println!("  {} Failed to start daemon: {e}", "!".yellow());
                println!("    Start manually: hive daemon start (or: sudo systemctl start hived)");
                bail!("Cannot continue setup without the daemon running.");
            }
            println!("  {} hived is running", "✓".green());

            // Give daemon a moment to initialize
            tokio::time::sleep(Duration::from_secs(2)).await;
        }
    } else {
        println!(
            "  {} Automatic daemon management not available on {os}",
            "!".yellow()
        );
        println!("    Ensure hived is running before continuing.");
    }

    // Step 4: Init or Join (daemon must be running at this point)
    if let Some(code) = join_code {
        println!();
        println!("  {} Joining cluster with code {}...", "⟳".cyan(), code);
        let host = addr.split(':').next().unwrap_or("127.0.0.1");
        crate::commands::join::run_with_code(code, host, addr, ca_cert).await?;
    } else {
        let cluster_name = name.map(|s| s.to_string()).unwrap_or_else(|| {
            hostname::get()
                .ok()
                .and_then(|h| h.into_string().ok())
                .unwrap_or_else(|| "hive-cluster".to_string())
        });
        println!();
        println!(
            "  {} Initializing cluster '{}'...",
            "⟳".cyan(),
            cluster_name
        );
        crate::commands::init::run(&cluster_name, addr, ca_cert).await?;
    }

    // Step 5: Verify
    println!();
    let spinner = ProgressBar::new_spinner();
    spinner.set_style(
        ProgressStyle::default_spinner()
            .template("{spinner:.green} {msg}")
            .unwrap(),
    );
    spinner.set_message("Verifying cluster...");
    spinner.enable_steady_tick(Duration::from_millis(100));

    match tokio::time::timeout(
        Duration::from_secs(10),
        crate::grpc_client::connect(addr, ca_cert),
    )
    .await
    {
        Ok(Ok(mut client)) => match client.get_cluster_status(()).await {
            Ok(resp) => {
                spinner.finish_and_clear();
                let status = resp.into_inner();
                println!(
                    "  {} Cluster is healthy: {} node(s), {} service(s)",
                    "✓".green().bold(),
                    status.total_nodes,
                    status.total_services
                );
            }
            Err(e) => {
                spinner.finish_and_clear();
                println!(
                    "  {} Daemon is running but cluster not yet ready: {}",
                    "!".yellow(),
                    e.message()
                );
            }
        },
        Ok(Err(e)) => {
            spinner.finish_and_clear();
            println!(
                "  {} Could not connect to daemon at {}: {}",
                "!".yellow(),
                addr,
                e
            );
        }
        Err(_) => {
            spinner.finish_and_clear();
            println!(
                "  {} Connection to daemon timed out ({})",
                "!".yellow(),
                addr
            );
        }
    }

    // Final summary
    println!();
    if join_code.is_none() {
        println!("{}", "Setup complete!".green().bold());
        println!("  Other nodes can join with: hive setup --join HIVE-XXXX-XXXX");
        println!("  Web console: http://localhost:7949");
    } else {
        println!(
            "{}",
            "Setup complete! Node has joined the cluster."
                .green()
                .bold()
        );
    }

    Ok(())
}

/// Prompt the user for confirmation. Falls back to a clear error if no TTY.
fn confirm_prompt(msg: &str) -> Result<bool> {
    use dialoguer::Confirm;
    Confirm::new()
        .with_prompt(msg)
        .default(true)
        .interact()
        .map_err(|e| {
            let msg = e.to_string();
            if msg.contains("not a terminal") || msg.contains("No such device") {
                anyhow::anyhow!("No TTY detected. Use --yes (-y) to accept all defaults.")
            } else {
                anyhow::anyhow!("prompt failed: {e}")
            }
        })
}

struct RuntimeInfo {
    name: String,
    version: String,
}

fn detect_runtime() -> Option<RuntimeInfo> {
    if let Ok(output) = Command::new("docker")
        .args(["version", "--format", "{{.Server.Version}}"])
        .output()
    {
        if output.status.success() {
            let version = String::from_utf8_lossy(&output.stdout).trim().to_string();
            if !version.is_empty() {
                return Some(RuntimeInfo {
                    name: "Docker".to_string(),
                    version,
                });
            }
        }
    }
    if let Ok(output) = Command::new("podman")
        .args(["version", "--format", "{{.Server.Version}}"])
        .output()
    {
        if output.status.success() {
            let version = String::from_utf8_lossy(&output.stdout).trim().to_string();
            if !version.is_empty() {
                return Some(RuntimeInfo {
                    name: "Podman".to_string(),
                    version,
                });
            }
        }
    }
    None
}

/// Install a container runtime on Linux. Returns the name of what was installed.
fn install_docker_linux() -> Result<String> {
    let os_release = std::fs::read_to_string("/etc/os-release")
        .unwrap_or_default()
        .to_lowercase();

    // Parse ID and ID_LIKE for robust distro detection
    let is_rpm = os_release.contains("id=fedora")
        || os_release.contains("id=rhel")
        || os_release.contains("id=centos")
        || os_release.contains("id=rocky")
        || os_release.contains("id=almalinux")
        || os_release.contains("id_like=") && os_release.contains("fedora");

    let is_deb = os_release.contains("id=ubuntu")
        || os_release.contains("id=debian")
        || os_release.contains("id=linuxmint")
        || os_release.contains("id=pop");

    if is_rpm {
        println!("    Running: sudo dnf install -y podman");
        let status = Command::new("sudo")
            .args(["dnf", "install", "-y", "podman"])
            .status()
            .context("failed to install podman")?;
        if !status.success() {
            bail!("Podman installation failed");
        }
        println!("  {} Podman installed", "✓".green());
        Ok("Podman".to_string())
    } else if is_deb {
        println!("    Running: sudo apt-get install -y docker.io");
        let status = Command::new("sudo")
            .args(["apt-get", "install", "-y", "docker.io"])
            .status()
            .context("failed to install docker")?;
        if !status.success() {
            bail!("Docker installation failed");
        }
        let _ = Command::new("sudo")
            .args(["systemctl", "start", "docker"])
            .status();
        let _ = Command::new("sudo")
            .args(["systemctl", "enable", "docker"])
            .status();
        println!("  {} Docker installed and started", "✓".green());
        Ok("Docker".to_string())
    } else {
        println!("    Running: sudo apt-get install -y docker.io (fallback)");
        let status = Command::new("sudo")
            .args(["apt-get", "install", "-y", "docker.io"])
            .status()
            .or_else(|_| {
                // If apt-get not available, try dnf
                Command::new("sudo")
                    .args(["dnf", "install", "-y", "podman"])
                    .status()
            })
            .context("failed to install container runtime")?;
        if !status.success() {
            bail!("Container runtime installation failed. Install Docker or Podman manually.");
        }
        println!("  {} Container runtime installed", "✓".green());
        Ok("Docker".to_string())
    }
}
