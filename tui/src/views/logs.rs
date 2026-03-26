use ratatui::{
    Frame,
    layout::Rect,
    style::{Color, Style},
    text::{Line, Span},
    widgets::{Block, Borders, Paragraph},
};

pub fn draw(frame: &mut Frame, area: Rect) {
    let block = Block::default().borders(Borders::ALL).title(" Logs ");

    let text = Paragraph::new(vec![
        Line::from(Span::styled(
            "  No log streams active.",
            Style::default().fg(Color::DarkGray),
        )),
        Line::from(""),
        Line::from(Span::styled(
            "  Deploy a service and logs will stream here.",
            Style::default().fg(Color::DarkGray),
        )),
    ])
    .block(block);

    frame.render_widget(text, area);
}
