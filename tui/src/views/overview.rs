use ratatui::{
    Frame,
    layout::{Constraint, Direction, Layout, Rect},
    style::{Color, Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, Paragraph},
};

use crate::app::ClusterData;

pub fn draw(frame: &mut Frame, area: Rect, data: &Option<ClusterData>) {
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([Constraint::Length(5), Constraint::Min(0)])
        .split(area);

    draw_stats(frame, chunks[0], data);
    draw_events(frame, chunks[1], data);
}

fn draw_stats(frame: &mut Frame, area: Rect, data: &Option<ClusterData>) {
    let (nodes, services, containers) = data
        .as_ref()
        .and_then(|d| d.status.as_ref())
        .map(|s| {
            (
                format!("{}/{}", s.healthy_nodes, s.total_nodes),
                s.total_services.to_string(),
                s.running_containers.to_string(),
            )
        })
        .unwrap_or(("-".into(), "-".into(), "-".into()));

    let cols = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([
            Constraint::Percentage(33),
            Constraint::Percentage(33),
            Constraint::Percentage(34),
        ])
        .split(area);

    let stats = [
        ("Nodes", nodes, Color::Cyan),
        ("Services", services, Color::Green),
        ("Containers", containers, Color::Yellow),
    ];

    for (i, (label, value, color)) in stats.iter().enumerate() {
        let block = Block::default()
            .borders(Borders::ALL)
            .title(format!(" {label} "));
        let text = Paragraph::new(Line::from(Span::styled(
            value.clone(),
            Style::default().fg(*color).add_modifier(Modifier::BOLD),
        )))
        .centered()
        .block(block);
        frame.render_widget(text, cols[i]);
    }
}

fn draw_events(frame: &mut Frame, area: Rect, data: &Option<ClusterData>) {
    let block = Block::default()
        .borders(Borders::ALL)
        .title(" Cluster Info ");

    let connected = data.as_ref().is_some_and(|d| d.connected);

    let lines = if !connected {
        let err = data
            .as_ref()
            .and_then(|d| d.error.as_ref())
            .map(|e| e.as_str())
            .unwrap_or("not connected");
        vec![
            Line::from(""),
            Line::from(Span::styled(
                format!("  Not connected to hived: {err}"),
                Style::default().fg(Color::Red),
            )),
            Line::from(""),
            Line::from(Span::styled(
                "  Start hived: hived --data-dir /tmp/hive-data",
                Style::default().fg(Color::DarkGray),
            )),
        ]
    } else {
        let status = data.as_ref().and_then(|d| d.status.as_ref());
        let mut lines = vec![Line::from("")];
        if let Some(s) = status {
            for node in &s.nodes {
                let caps = node
                    .capabilities
                    .as_ref()
                    .map(|c| format!("{}/{} ({})", c.os, c.arch, c.container_runtime))
                    .unwrap_or_default();
                lines.push(Line::from(format!("  Node: {} — {caps}", node.name)));
            }
        }
        lines
    };

    let text = Paragraph::new(lines).block(block);
    frame.render_widget(text, area);
}
