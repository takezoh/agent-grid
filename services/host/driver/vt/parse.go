package vt

import (
	"strconv"
	"strings"
)

// ParseOscProgress recognizes ConEmu/Windows Terminal OSC 9;4 progress.
// Payload is the portion after "9;" and therefore starts with "4;".
func ParseOscProgress(n OscNotification) (OscProgress, bool) {
	if n.Cmd != 9 {
		return OscProgress{}, false
	}
	parts := strings.Split(n.Payload, ";")
	if len(parts) < 2 || parts[0] != "4" {
		return OscProgress{}, false
	}
	state, err := strconv.Atoi(parts[1])
	if err != nil || state < 0 || state > 4 {
		return OscProgress{}, false
	}
	value := 0
	if len(parts) >= 3 {
		value, err = strconv.Atoi(parts[2])
		if err != nil {
			return OscProgress{}, false
		}
	}
	return OscProgress{State: state, Value: value}, true
}

// ParseOscNotification extracts title and body from an OSC notification payload.
// OSC 9 (iTerm2): payload is the title text.
// OSC 777 (urxvt): payload is "notify;<title>;<body>".
// OSC 99 (Kitty): colon-separated key=value pairs; d= is title, p= is body.
func ParseOscNotification(n OscNotification) (title, body string) {
	switch n.Cmd {
	case 9:
		return strings.TrimSpace(n.Payload), ""
	case 777:
		parts := strings.SplitN(n.Payload, ";", 3)
		if len(parts) >= 3 {
			return parts[1], parts[2]
		}
		if len(parts) == 2 {
			return parts[1], ""
		}
	case 99:
		title, body = parseKittyPayload(n.Payload)
		if title == "" && body == "" {
			body = n.Payload
		}
	}
	return title, body
}

func parseKittyPayload(payload string) (title, body string) {
	for _, part := range strings.Split(payload, ":") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch k {
		case "d":
			title = v
		case "p":
			body = v
		}
	}
	return title, body
}
