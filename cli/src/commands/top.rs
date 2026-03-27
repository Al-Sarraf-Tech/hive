use anyhow::{Context, Result};
use std::process::Command;

pub fn run(addr: &str) -> Result<()> {
    let status = Command::new("hivetop")
        .arg("--addr")
        .arg(addr)
        .status()
        .context("failed to launch hivetop — is it installed? Run: cargo install --path tui")?;

    // On Unix, if the process was killed by a signal, code() returns None.
    // Convention is to exit with 128 + signal number.
    #[cfg(unix)]
    {
        use std::os::unix::process::ExitStatusExt;
        if let Some(sig) = status.signal() {
            std::process::exit(128 + sig);
        }
    }
    std::process::exit(status.code().unwrap_or(1));
}
