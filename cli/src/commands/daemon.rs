use anyhow::{Context, Result};
use colored::Colorize;
use std::process::Command;

const UNIT_NAME: &str = "hived.service";
const UNIT_PATH: &str = "/etc/systemd/system/hived.service";

pub fn install() -> Result<()> {
    if cfg!(not(target_os = "linux")) {
        eprintln!("{} systemd integration is Linux-only.", "note:".cyan());
        eprintln!("On other platforms, run hived directly.");
        return Ok(());
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

    // Write unit file (requires root)
    let tmp = "/tmp/hived.service.tmp";
    std::fs::write(tmp, &unit).context("failed to write temp unit file")?;

    let status = Command::new("sudo")
        .args(["cp", tmp, UNIT_PATH])
        .status()
        .context("failed to install unit file (sudo required)")?;
    if !status.success() {
        anyhow::bail!("failed to copy unit file to {UNIT_PATH}");
    }

    let _ = std::fs::remove_file(tmp);

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
        eprintln!("{} systemd integration is Linux-only.", "note:".cyan());
        return Ok(());
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
        eprintln!("{} systemd integration is Linux-only.", "note:".cyan());
        return Ok(());
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
        eprintln!("{} systemd integration is Linux-only.", "note:".cyan());
        eprintln!("Use 'hive status' to check if hived is reachable.");
        return Ok(());
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
    Command::new("which")
        .arg("hived")
        .output()
        .ok()
        .filter(|o| o.status.success())
        .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
        .filter(|s| !s.is_empty())
}
