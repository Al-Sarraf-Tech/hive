use anyhow::Result;
use colored::Colorize;

pub fn run() -> Result<()> {
    println!("Launching {}...", "hivetop".cyan());
    // TODO: exec into hivetop binary, or embed TUI
    println!("hivetop is a separate binary. Run: hivetop");
    Ok(())
}
