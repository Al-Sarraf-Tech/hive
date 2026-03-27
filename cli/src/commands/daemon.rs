use anyhow::{Context, Result};
use colored::Colorize;
use std::process::Command;

const UNIT_NAME: &str = "hived.service";
const UNIT_PATH: &str = "/etc/systemd/system/hived.service";

pub fn install() -> Result<()> {
    if cfg!(not(target_os = "linux")) {
        anyhow::bail!("systemd integration is Linux-only. On other platforms, run hived directly.");
    }

    // Find hived binary
    let hived_path = which_hived().unwrap_or_else(|| "/usr/local/bin/hived".to_string());

    let unit = format!(
        r#"[Unit]
Description=Hive Container Orchestrator Daemon
Documentation=https://github.com/Al-Sarraf-Tech/hive
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=simple
ExecStart={hived_path} --data-dir /var/lib/hive --log-level info
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536
Environment=HIVE_GOSSIP_KEY=

[Install]
WantedBy=multi-user.target
"#
    );

    // Write unit file via sudo tee to avoid predictable /tmp path (symlink attack vector)
    let status = Command::new("sudo")
        .args(["tee", UNIT_PATH])
        .stdin(std::process::Stdio::piped())
        .stdout(std::process::Stdio::null())
        .spawn()
        .and_then(|mut child| {
            use std::io::Write;
            if let Some(ref mut stdin) = child.stdin {
                stdin.write_all(unit.as_bytes())?;
            }
            child.wait()
        })
        .context("failed to install unit file (sudo required)")?;
    if !status.success() {
        anyhow::bail!("failed to write unit file to {UNIT_PATH}");
    }

    let status = Command::new("sudo")
        .args(["systemctl", "daemon-reload"])
        .status()
        .context("failed to reload systemd")?;
    if !status.success() {
        anyhow::bail!("systemctl daemon-reload failed");
    }

    println!("{} hived.service installed to {UNIT_PATH}", "✓".green());
    println!();
    println!(
        "  Enable on boot: {} ",
        "sudo systemctl enable hived".yellow()
    );
    println!("  Start now:      {}", "hive daemon start".yellow());
    Ok(())
}

pub fn start() -> Result<()> {
    if cfg!(not(target_os = "linux")) {
        anyhow::bail!("systemd integration is Linux-only.");
    }

    println!("Starting hived...");
    let status = Command::new("sudo")
        .args(["systemctl", "start", UNIT_NAME])
        .status()
        .context("failed to start hived")?;
    if !status.success() {
        anyhow::bail!("systemctl start hived failed — check 'journalctl -u hived' for details");
    }
    println!("{} hived started.", "✓".green());
    Ok(())
}

pub fn stop() -> Result<()> {
    if cfg!(not(target_os = "linux")) {
        anyhow::bail!("systemd integration is Linux-only.");
    }

    println!("Stopping hived...");
    let status = Command::new("sudo")
        .args(["systemctl", "stop", UNIT_NAME])
        .status()
        .context("failed to stop hived")?;
    if !status.success() {
        anyhow::bail!("systemctl stop hived failed");
    }
    println!("{} hived stopped.", "✓".green());
    Ok(())
}

pub fn status() -> Result<()> {
    if cfg!(not(target_os = "linux")) {
        anyhow::bail!(
            "systemd integration is Linux-only. Use 'hive status' to check if hived is reachable."
        );
    }

    let output = Command::new("systemctl")
        .args(["status", UNIT_NAME, "--no-pager"])
        .output()
        .context("failed to query hived status")?;

    let stdout = String::from_utf8_lossy(&output.stdout);
    let stderr = String::from_utf8_lossy(&output.stderr);

    if !stdout.is_empty() {
        print!("{stdout}");
    }
    if !stderr.is_empty() {
        eprint!("{stderr}");
    }

    // systemctl status exits 3 if service is not running — not an error
    if !output.status.success() && output.status.code() != Some(3) {
        eprintln!();
        eprintln!(
            "{} hived may not be installed. Run: {}",
            "hint:".yellow(),
            "hive daemon install".cyan()
        );
    }

    Ok(())
}

fn which_hived() -> Option<String> {
    // Use POSIX 'command -v' instead of 'which' (not available on all distros)
    Command::new("sh")
        .args(["-c", "command -v hived"])
        .output()
        .ok()
        .filter(|o| o.status.success())
        .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
        .filter(|s| !s.is_empty())
}
