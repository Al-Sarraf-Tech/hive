use ratatui::{
    Frame,
    layout::{Constraint, Rect},
    style::{Color, Style},
    text::{Line, Span},
    widgets::{Block, Borders, Cell, Paragraph, Row, Table},
};

use crate::app::ClusterData;
use crate::grpc_client::hive_proto::ServiceStatus;

pub fn draw(frame: &mut Frame, area: Rect, data: &Option<ClusterData>) {
    let header = Row::new(vec![
        Cell::from("NAME"),
        Cell::from("IMAGE"),
        Cell::from("REPLICAS"),
        Cell::from("HEALTH"),
        Cell::from("STATUS"),
        Cell::from("NODE"),
    ])
    .style(Style::default().fg(Color::Yellow));

    let is_connected = data.as_ref().is_some_and(|d| d.connected);
    let has_services = data
        .as_ref()
        .and_then(|d| d.services.as_ref())
        .is_some_and(|s| !s.services.is_empty());

    if !has_services {
        let msg = if !is_connected {
            "  Not connected to hived"
        } else {
            "  No services running. Deploy with: hive deploy <hivefile.toml>"
        };
        let block = Block::default().borders(Borders::ALL).title(" Services ");
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
        .and_then(|d| d.services.as_ref())
        .map(|s| {
            s.services
                .iter()
                .map(|svc| {
                    let status_str = match ServiceStatus::try_from(svc.status) {
                        Ok(ServiceStatus::Running) => "running",
                        Ok(ServiceStatus::Degraded) => "degraded",
                        Ok(ServiceStatus::Stopped) => "stopped",
                        Ok(ServiceStatus::Deploying) => "deploying",
                        Ok(ServiceStatus::RollingBack) => "rolling back",
                        _ => "unknown",
                    };
                    let health_cell = if svc.replicas_desired == 0 {
                        Cell::from("-").style(Style::default().fg(Color::DarkGray))
                    } else if svc.replicas_running == svc.replicas_desired {
                        Cell::from("OK").style(Style::default().fg(Color::Green))
                    } else if svc.replicas_running > 0 {
                        Cell::from("DEGRADED").style(Style::default().fg(Color::Yellow))
                    } else {
                        Cell::from("DOWN").style(Style::default().fg(Color::Red))
                    };
                    Row::new(vec![
                        Cell::from(svc.name.as_str()),
                        Cell::from(svc.image.as_str()),
                        Cell::from(format!("{}/{}", svc.replicas_running, svc.replicas_desired)),
                        health_cell,
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
            Constraint::Length(10),
            Constraint::Length(12),
            Constraint::Min(12),
        ],
    )
    .header(header)
    .block(Block::default().borders(Borders::ALL).title(" Services "));

    frame.render_widget(table, area);
}
