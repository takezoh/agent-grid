package runtime

import (
	"bufio"
	"bytes"
	"os"
)

// ToolLogTailer replays on-disk tool-log JSONL from a persisted byte offset.
// Live append uses ToolLogReader.ProcessLine directly; the tailer supports
// restart backfill without re-emitting already-processed lines.
type ToolLogTailer struct {
	Reader  *ToolLogReader
	offsets map[string]int64
}

// NewToolLogTailer returns a tailer with empty offset map.
func NewToolLogTailer(reader *ToolLogReader) *ToolLogTailer {
	return &ToolLogTailer{
		Reader:  reader,
		offsets: make(map[string]int64),
	}
}

func (t *ToolLogTailer) key(namespace, project string) string {
	return namespace + "\x00" + project
}

// Offset returns the persisted byte offset for a namespace/project log.
func (t *ToolLogTailer) Offset(namespace, project string) int64 {
	return t.offsets[t.key(namespace, project)]
}

// Tail processes bytes appended since the last offset and returns activity
// events for complete JSONL lines only.
func (t *ToolLogTailer) Tail(sessionID, namespace, project string, content []byte) []ActivityEvent {
	k := t.key(namespace, project)
	start := t.offsets[k]
	if int64(len(content)) <= start {
		return nil
	}
	chunk := content[start:]
	var events []ActivityEvent
	sc := bufio.NewScanner(bytes.NewReader(chunk))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			start++
			continue
		}
		events = append(events, t.Reader.ProcessLine(sessionID, line)...)
		start += int64(len(line)) + 1
	}
	t.offsets[k] = start
	return events
}

// ReplayFile reads a log file from the persisted offset through EOF.
func (t *ToolLogTailer) ReplayFile(sessionID, namespace, project, path string) ([]ActivityEvent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return t.Tail(sessionID, namespace, project, data), nil
}
