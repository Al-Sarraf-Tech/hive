use ratatui::{
    Frame,
    layout::{Constraint, Rect},
    style::{Color, Style},
    widgets::{Block, Borders, Cell, Row, Table},
};

use crate::app::ClusterData;

pub fn draw(frame: &mut Frame, area: Rect, data: &Option<ClusterData>) {
    let header = Row::new(vec![
        Cell::from("NAME"),
        Cell::from("IMAGE"),
        Cell::from("REPLICAS"),
        Cell::from("STATUS"),
        Cell::from("NODE"),
    ])
    .style(Style::default().fg(Color::Yellow));

    let rows: Vec<Row> = data
        .as_ref()
        .and_then(|d| d.services.as_ref())
        .map(|s| {
            s.services
                .iter()
                .map(|svc| {
                    let status_str = match svc.status {
                        1 => "running",
                        2 => "degraded",
                        3 => "stopped",
                        4 => "deploying",
                        5 => "rolling back",
                        _ => "unknown",
                    };
                    Row::new(vec![
                        Cell::from(svc.name.as_str()),
                        Cell::from(svc.image.as_str()),
                        Cell::from(format!("{}/{}", svc.replicas_running, svc.replicas_desired)),
                        Cell::from(status_str),
                        Cell::from(svc.node_constraint.as_str()),
                    ])
                })
                .collect()
        })
        .unwrap_or_default();

    let table = Table::new(
        rows,
        [
            Constraint::Min(15),
            Constraint::Min(20),
            Constraint::Length(10),
            Constraint::Length(12),
            Constraint::Min(12),
        ],
    )
    .header(header)
    .block(Block::default().borders(Borders::ALL).title(" Services "));

    frame.render_widget(table, area);
}
