use ratatui::{
    Frame,
    layout::{Constraint, Rect},
    style::{Color, Style},
    widgets::{Block, Borders, Cell, Row, Table},
};

use crate::app::ClusterData;

pub fn draw(frame: &mut Frame, area: Rect, data: &Option<ClusterData>) {
    let header = Row::new(vec![
        Cell::from("STATUS"),
        Cell::from("NAME"),
        Cell::from("OS"),
        Cell::from("ARCH"),
        Cell::from("RUNTIME"),
        Cell::from("PLATFORMS"),
    ])
    .style(Style::default().fg(Color::Yellow));

    let rows: Vec<Row> = data
        .as_ref()
        .and_then(|d| d.nodes.as_ref())
        .map(|n| {
            n.nodes
                .iter()
                .map(|node| {
                    let caps = node.capabilities.as_ref();
                    Row::new(vec![
                        Cell::from(match node.status {
                            1 => "● ready",
                            2 => "◐ draining",
                            3 => "○ down",
                            _ => "? unknown",
                        }),
                        Cell::from(node.name.as_str()),
                        Cell::from(caps.map(|c| c.os.as_str()).unwrap_or("-")),
                        Cell::from(caps.map(|c| c.arch.as_str()).unwrap_or("-")),
                        Cell::from(caps.map(|c| c.container_runtime.as_str()).unwrap_or("-")),
                        Cell::from(caps.map(|c| c.platforms.join(", ")).unwrap_or_default()),
                    ])
                })
                .collect()
        })
        .unwrap_or_default();

    let table = Table::new(
        rows,
        [
            Constraint::Length(12),
            Constraint::Min(15),
            Constraint::Length(10),
            Constraint::Length(7),
            Constraint::Length(12),
            Constraint::Min(15),
        ],
    )
    .header(header)
    .block(Block::default().borders(Borders::ALL).title(" Nodes "));

    frame.render_widget(table, area);
}
