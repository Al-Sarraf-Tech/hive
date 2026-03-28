use ratatui::{
    Frame,
    layout::{Constraint, Rect},
    style::{Color, Style},
    text::{Line, Span},
    widgets::{Block, Borders, Cell, Paragraph, Row, Table},
};

use crate::app::ClusterData;
use crate::grpc_client::hive_proto::NodeStatus;

pub fn draw(frame: &mut Frame, area: Rect, data: &Option<ClusterData>) {
    let header = Row::new(vec![
        Cell::from("STATUS"),
        Cell::from("NAME"),
        Cell::from("MESH IP"),
        Cell::from("OS"),
        Cell::from("ARCH"),
        Cell::from("RUNTIME"),
        Cell::from("PLATFORMS"),
    ])
    .style(Style::default().fg(Color::Yellow));

    let is_connected = data.as_ref().is_some_and(|d| d.connected);
    let has_nodes = data
        .as_ref()
        .and_then(|d| d.nodes.as_ref())
        .is_some_and(|n| !n.nodes.is_empty());

    if !has_nodes {
        let msg = if !is_connected {
            "  Not connected to hived"
        } else {
            "  No nodes in cluster"
        };
        let block = Block::default().borders(Borders::ALL).title(" Nodes ");
        let text = Paragraph::new(Line::from(Span::styled(
            msg,
            Style::default().fg(Color::DarkGray),
        )))
        .block(block);
        frame.render_widget(text, area);
        return;
    }

    let rows: Vec<Row> = data
        .as_ref()
        .and_then(|d| d.nodes.as_ref())
        .map(|n| {
            n.nodes
                .iter()
                .map(|node| {
                    let caps = node.capabilities.as_ref();
                    let mesh_ip = if node.wg_addr.is_empty() {
                        "-".to_string()
                    } else {
                        node.wg_addr.clone()
                    };
                    Row::new(vec![
                        Cell::from(match NodeStatus::try_from(node.status) {
                            Ok(NodeStatus::Ready) => "● ready",
                            Ok(NodeStatus::Draining) => "◐ draining",
                            Ok(NodeStatus::Down) => "○ down",
                            _ => "? unknown",
                        }),
                        Cell::from(node.name.as_str()),
                        Cell::from(mesh_ip),
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
            Constraint::Length(16),
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
