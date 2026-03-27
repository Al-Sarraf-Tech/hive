use anyhow::Result;
use colored::Colorize;

pub fn install() -> Result<()> {
    eprintln!(
        "{} 'hive daemon install' is not yet implemented.\n",
        "note:".cyan()
    );
    eprintln!("For now, run hived directly:");
    eprintln!("  hived --data-dir /var/lib/hive");
    Ok(())
}

pub fn start() -> Result<()> {
    eprintln!(
        "{} 'hive daemon start' is not yet implemented.\n",
        "note:".cyan()
    );
    eprintln!("For now, run hived directly:");
    eprintln!("  hived --data-dir /var/lib/hive");
    Ok(())
}

pub fn stop() -> Result<()> {
    eprintln!(
        "{} 'hive daemon stop' is not yet implemented.\n",
        "note:".cyan()
    );
    eprintln!("Stop hived with Ctrl+C or: kill $(pgrep hived)");
    Ok(())
}

pub fn status() -> Result<()> {
    eprintln!(
        "{} 'hive daemon status' is not yet implemented.\n",
        "note:".cyan()
    );
    eprintln!("Use 'hive status' to check if hived is reachable.");
    Ok(())
}
