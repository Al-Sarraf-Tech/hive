use anyhow::{Context, Result};
use std::process::Command;

pub fn run(addr: &str) -> Result<()> {
    let status = Command::new("hivetop")
        .arg("--addr")
        .arg(addr)
        .status()
        .context("failed to launch hivetop — is it installed? Run: cargo install --path tui")?;
    std::process::exit(status.code().unwrap_or(1));
}
