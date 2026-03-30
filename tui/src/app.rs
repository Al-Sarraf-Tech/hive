use std::collections::VecDeque;

use ratatui::{
    Frame,
    layout::{Constraint, Direction, Layout, Rect},
    style::{Color, Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, Paragraph, Tabs},
};

use crate::grpc_client::hive_proto::{
    ClusterStatusResponse, ListNodesResponse, ListServicesResponse,
};
use crate::views;

#[derive(Clone, Copy, PartialEq, Eq)]
pub enum Tab {
    Overview,
    Nodes,
    Services,
    Logs,
}

impl Tab {
    fn titles() -> Vec<&'static str> {
        vec!["[1] Overview", "[2] Nodes", "[3] Services", "[4] Logs"]
    }

    fn index(self) -> usize {
        match self {
            Tab::Overview => 0,
            Tab::Nodes => 1,
            Tab::Services => 2,
            Tab::Logs => 3,
        }
    }
}

pub struct ClusterData {
    pub connected: bool,
    pub status: Option<ClusterStatusResponse>,
    pub services: Option<ListServicesResponse>,
    pub nodes: Option<ListNodesResponse>,
    pub error: Option<String>,
}

pub struct App {
    pub tab: Tab,
    pub addr: String,
    pub data: Option<ClusterData>,
    pub logs: VecDeque<String>,
}

impl App {
    pub fn new(addr: String) -> Self {
        Self {
            tab: Tab::Overview,
            addr,
            data: None,
            logs: VecDeque::new(),
        }
    }

    pub fn update_data(&mut self, data: ClusterData) {
        self.data = Some(data);
    }

    pub fn push_log(&mut self, line: String) {
        self.logs.push_back(line);
        // Keep at most 500 lines — pop_front is O(1) with VecDeque
        if self.logs.len() > 500 {
            self.logs.pop_front();
        }
    }

    pub fn draw(&self, frame: &mut Frame) {
        let chunks = Layout::default()
            .direction(Direction::Vertical)
            .constraints([
                Constraint::Length(3),
                Constraint::Min(0),
                Constraint::Length(1),
            ])
            .split(frame.area());

        self.draw_header(frame, chunks[0]);
        self.draw_content(frame, chunks[1]);
        self.draw_status_bar(frame, chunks[2]);
    }

    fn draw_header(&self, frame: &mut Frame, area: Rect) {
        let connected = self.data.as_ref().is_some_and(|d| d.connected);
        let conn_indicator = if connected { "●" } else { "○" };
        let conn_color = if connected { Color::Green } else { Color::Red };

        let title = format!(
            " Hive v2.6.0 {} {}",
            conn_indicator,
            if connected {
                &self.addr
            } else {
                "disconnected"
            }
        );

        let titles: Vec<Line> = Tab::titles()
            .iter()
            .map(|t| Line::from(Span::raw(*t)))
            .collect();

        let tabs = Tabs::new(titles)
            .block(
                Block::default()
                    .borders(Borders::ALL)
                    .title(Span::styled(title, Style::default().fg(conn_color))),
            )
            .select(self.tab.index())
            .highlight_style(
                Style::default()
                    .fg(Color::Yellow)
                    .add_modifier(Modifier::BOLD),
            );

        frame.render_widget(tabs, area);
    }

    fn draw_content(&self, frame: &mut Frame, area: Rect) {
        match self.tab {
            Tab::Overview => views::overview::draw(frame, area, &self.data),
            Tab::Nodes => views::nodes::draw(frame, area, &self.data),
            Tab::Services => views::services::draw(frame, area, &self.data),
            Tab::Logs => views::logs::draw(frame, area, &self.logs),
        }
    }

    fn draw_status_bar(&self, frame: &mut Frame, area: Rect) {
        let status = Paragraph::new(Line::from(vec![
            Span::styled(" 1-4", Style::default().fg(Color::Yellow)),
            Span::raw(":tabs  "),
            Span::styled("q/Esc/^C", Style::default().fg(Color::Yellow)),
            Span::raw(":quit  "),
            Span::styled(
                concat!("hive v", env!("CARGO_PKG_VERSION")),
                Style::default().fg(Color::DarkGray),
            ),
        ]));
        frame.render_widget(status, area);
    }
}
