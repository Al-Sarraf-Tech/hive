use ratatui::{
    Frame,
    layout::Rect,
    style::{Color, Style},
    text::{Line, Span},
    widgets::{Block, Borders, Paragraph},
};

pub fn draw(frame: &mut Frame, area: Rect, logs: &std::collections::VecDeque<String>) {
    let block = Block::default().borders(Borders::ALL).title(" Events ");

    if logs.is_empty() {
        let text = Paragraph::new(vec![
            Line::from(""),
            Line::from(Span::styled(
                "  Waiting for cluster events...",
                Style::default().fg(Color::DarkGray),
            )),
            Line::from(""),
            Line::from(Span::styled(
                "  Events will appear here as nodes join/leave and services deploy.",
                Style::default().fg(Color::DarkGray),
            )),
        ])
        .block(block);
        frame.render_widget(text, area);
        return;
    }

    // Show latest logs, auto-scrolled to bottom
    let lines: Vec<Line> = logs
        .iter()
        .map(|l| {
            let style = if l.contains("ERR") || l.contains("failed") || l.contains("FAILED") {
                Style::default().fg(Color::Red)
            } else if l.contains("WARN") || l.contains("degraded") {
                Style::default().fg(Color::Yellow)
            } else {
                Style::default().fg(Color::Gray)
            };
            Line::from(Span::styled(l.as_str(), style))
        })
        .collect();

    // Auto-scroll: show the last N lines that fit in the area
    let visible_height = area.height.saturating_sub(2) as usize; // subtract borders
    let scroll = if lines.len() > visible_height {
        (lines.len() - visible_height) as u16
    } else {
        0
    };

    // Don't use Wrap — it makes lines span multiple rows, which breaks the
    // scroll offset calculation (we assume 1 line = 1 row). Long lines are
    // truncated instead; users can widen the terminal to see more.
    let text = Paragraph::new(lines).block(block).scroll((scroll, 0));

    frame.render_widget(text, area);
}
