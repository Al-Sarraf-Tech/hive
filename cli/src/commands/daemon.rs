use anyhow::{Context, Result, bail};
use colored::Colorize;
use std::process::Command;

const UNIT_NAME: &str = "hived.service";
const UNIT_PATH: &str = "/etc/systemd/system/hived.service";
const WIN_SERVICE_NAME: &str = "hived";

pub fn install() -> Result<()> {
    if cfg!(target_os = "linux") {
        install_linux()
    } else if cfg!(target_os = "windows") {
        install_windows()
    } else {
        bail!("daemon management not supported on this OS");
    }
}

pub fn start() -> Result<()> {
    if cfg!(target_os = "linux") {
        start_linux()
    } else if cfg!(target_os = "windows") {
        start_windows()
    } else {
        bail!("daemon management not supported on this OS");
    }
}

pub fn stop() -> Result<()> {
    if cfg!(target_os = "linux") {
        stop_linux()
    } else if cfg!(target_os = "windows") {
        stop_windows()
    } else {
        bail!("daemon management not supported on this OS");
    }
}

pub fn status() -> Result<()> {
    if cfg!(target_os = "linux") {
        status_linux()
    } else if cfg!(target_os = "windows") {
        status_windows()
    } else {
        bail!(
            "daemon management not supported on this OS. Use 'hive status' to check if hived is reachable."
        );
    }
}

// ---------------------------------------------------------------------------
// Linux (systemd)
// ---------------------------------------------------------------------------

fn install_linux() -> Result<()> {
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
            if let Some(mut stdin) = child.stdin.take() {
                stdin.write_all(unit.as_bytes())?;
            }
            // stdin is dropped here, sending EOF to tee
            child.wait()
        })
        .context("failed to install unit file (sudo required)")?;
    if !status.success() {
        bail!("failed to write unit file to {UNIT_PATH}");
    }

    let status = Command::new("sudo")
        .args(["systemctl", "daemon-reload"])
        .status()
        .context("failed to reload systemd")?;
    if !status.success() {
        bail!("systemctl daemon-reload failed");
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

fn start_linux() -> Result<()> {
    println!("Starting hived...");
    let status = Command::new("sudo")
        .args(["systemctl", "start", UNIT_NAME])
        .status()
        .context("failed to start hived")?;
    if !status.success() {
        bail!("systemctl start hived failed — check 'journalctl -u hived' for details");
    }
    println!("{} hived started.", "✓".green());
    Ok(())
}

fn stop_linux() -> Result<()> {
    println!("Stopping hived...");
    let status = Command::new("sudo")
        .args(["systemctl", "stop", UNIT_NAME])
        .status()
        .context("failed to stop hived")?;
    if !status.success() {
        bail!("systemctl stop hived failed");
    }
    println!("{} hived stopped.", "✓".green());
    Ok(())
}

fn status_linux() -> Result<()> {
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

// ---------------------------------------------------------------------------
// Windows (sc.exe)
// ---------------------------------------------------------------------------

fn install_windows() -> Result<()> {
    let hived_path =
        which_hived().unwrap_or_else(|| r"C:\Program Files\Hive\hived.exe".to_string());

    // Create data directory if it doesn't exist
    let data_dir = r"C:\ProgramData\Hive\data";
    let _ = std::fs::create_dir_all(data_dir);

    println!("  Installing hived as Windows service...");
    // Quote the executable path — it may contain spaces (e.g., C:\Program Files\...)
    let bin_path = format!(
        "\"{}\" --data-dir {} --log-level info",
        hived_path, data_dir
    );
    let status = Command::new("sc.exe")
        .args([
            "create",
            WIN_SERVICE_NAME,
            &format!("binPath={bin_path}"),
            "start=auto",
            "DisplayName=Hive Daemon",
        ])
        .status()
        .context("failed to run sc.exe")?;
    if !status.success() {
        bail!("sc.exe create failed — run as Administrator");
    }
    println!("{} hived service installed", "✓".green());
    println!("  Start: {}", "hive daemon start".yellow());
    Ok(())
}

fn start_windows() -> Result<()> {
    println!("Starting hived...");
    let status = Command::new("sc.exe")
        .args(["start", WIN_SERVICE_NAME])
        .status()
        .context("failed to run sc.exe")?;
    if !status.success() {
        bail!("sc.exe start failed — run as Administrator");
    }
    println!("{} hived started.", "✓".green());
    Ok(())
}

fn stop_windows() -> Result<()> {
    println!("Stopping hived...");
    let status = Command::new("sc.exe")
        .args(["stop", WIN_SERVICE_NAME])
        .status()
        .context("failed to run sc.exe")?;
    if !status.success() {
        bail!("sc.exe stop failed — run as Administrator");
    }
    println!("{} hived stopped.", "✓".green());
    Ok(())
}

fn status_windows() -> Result<()> {
    let output = Command::new("sc.exe")
        .args(["query", WIN_SERVICE_NAME])
        .output()
        .context("failed to run sc.exe")?;

    let stdout = String::from_utf8_lossy(&output.stdout);
    let stderr = String::from_utf8_lossy(&output.stderr);

    if !stdout.is_empty() {
        print!("{stdout}");
    }
    if !stderr.is_empty() {
        eprint!("{stderr}");
    }

    if !output.status.success() {
        eprintln!();
        eprintln!(
            "{} hived service may not be installed. Run: {}",
            "hint:".yellow(),
            "hive daemon install".cyan()
        );
    }

    Ok(())
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

fn which_hived() -> Option<String> {
    if cfg!(target_os = "windows") {
        // Try where.exe first (Windows equivalent of 'which')
        let from_path = Command::new("where.exe")
            .arg("hived.exe")
            .output()
            .ok()
            .filter(|o| o.status.success())
            .map(|o| {
                String::from_utf8_lossy(&o.stdout)
                    .lines()
                    .next()
                    .unwrap_or("")
                    .to_string()
            })
            .filter(|s| !s.is_empty());
        if from_path.is_some() {
            return from_path;
        }
        // Check common install location
        let default = r"C:\Program Files\Hive\hived.exe";
        if std::path::Path::new(default).exists() {
            return Some(default.to_string());
        }
        None
    } else {
        // Use POSIX 'command -v' instead of 'which' (not available on all distros)
        Command::new("sh")
            .args(["-c", "command -v hived"])
            .output()
            .ok()
            .filter(|o| o.status.success())
            .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
            .filter(|s| !s.is_empty())
    }
}
