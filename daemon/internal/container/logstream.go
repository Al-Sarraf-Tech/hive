package container

import (
	"fmt"
	"io"
	"strings"
)

const maxLogPayload = 16 * 1024 * 1024 // 16 MB max per log frame

// StreamDockerLogs reads the Docker multiplexed log stream and calls sendFn
// for each line. The Docker log protocol uses 8-byte headers per frame:
// [stream_type(1)][padding(3)][size(4 big-endian)] followed by payload.
// stream_type: 1=stdout, 2=stderr.
func StreamDockerLogs(reader io.Reader, sendFn func(line string, stream string) error) error {
	header := make([]byte, 8)
	for {
		if _, err := io.ReadFull(reader, header); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil // clean end of stream
			}
			return fmt.Errorf("read log header: %w", err)
		}

		streamType := "stdout"
		if header[0] == 2 {
			streamType = "stderr"
		}

		payloadSize := int(header[4])<<24 | int(header[5])<<16 | int(header[6])<<8 | int(header[7])
		if payloadSize <= 0 {
			continue
		}
		if payloadSize > maxLogPayload {
			return fmt.Errorf("log payload size %d exceeds maximum %d", payloadSize, maxLogPayload)
		}

		payload := make([]byte, payloadSize)
		if _, err := io.ReadFull(reader, payload); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			}
			return fmt.Errorf("read log payload: %w", err)
		}

		line := strings.TrimRight(string(payload), "\n\r")
		if line == "" {
			continue
		}

		if err := sendFn(line, streamType); err != nil {
			return err
		}
	}
}
