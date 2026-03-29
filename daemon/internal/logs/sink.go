package logs

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Sink receives log entries for external shipping.
type Sink interface {
	Write(entry Entry) error
	Close() error
}

// FileSink writes JSON log lines to a rotating file.
type FileSink struct {
	mu   sync.Mutex
	f    *os.File
	path string
}

// NewFileSink creates a sink that appends JSON lines to the given path.
func NewFileSink(path string) (*FileSink, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return &FileSink{f: f, path: path}, nil
}

func (s *FileSink) Write(entry Entry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.f.Write(data)
	return err
}

func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.f.Close()
}

// SyslogSink sends log entries to a remote syslog server via UDP.
type SyslogSink struct {
	mu   sync.Mutex
	conn net.Conn
	host string
}

// NewSyslogSink creates a sink that sends entries to a syslog server.
func NewSyslogSink(host string) (*SyslogSink, error) {
	if host == "" {
		return nil, fmt.Errorf("syslog host is required")
	}
	conn, err := net.Dial("udp", host)
	if err != nil {
		return nil, fmt.Errorf("dial syslog %s: %w", host, err)
	}
	return &SyslogSink{conn: conn, host: host}, nil
}

func (s *SyslogSink) Write(entry Entry) error {
	msg := fmt.Sprintf("<%d>1 %s hive - - - %s/%s: %s",
		14, // facility=user, severity=info
		entry.Timestamp.UTC().Format(time.RFC3339),
		entry.ServiceName,
		entry.ContainerID,
		entry.Line,
	)
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.conn.Write([]byte(msg))
	return err
}

func (s *SyslogSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.Close()
}
