use anyhow::{Result, bail};

pub fn install() -> Result<()> {
    bail!(
        "'hive daemon install' is not yet implemented.\n\nFor now, run hived directly:\n  hived --data-dir /var/lib/hive"
    )
}

pub fn start() -> Result<()> {
    bail!(
        "'hive daemon start' is not yet implemented.\n\nFor now, run hived directly:\n  hived --data-dir /var/lib/hive"
    )
}

pub fn stop() -> Result<()> {
    bail!(
        "'hive daemon stop' is not yet implemented.\n\nStop hived with Ctrl+C or: kill $(pgrep hived)"
    )
}

pub fn status() -> Result<()> {
    bail!(
        "'hive daemon status' is not yet implemented.\n\nUse 'hive status' to check if hived is reachable."
    )
}
