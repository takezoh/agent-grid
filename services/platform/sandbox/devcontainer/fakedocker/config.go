package fakedocker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	EnvConfigPath = "FAKEDOCKER_CONFIG"
	EnvRecordPath = "FAKEDOCKER_RECORD_FILE"
)

type Config struct {
	Responses map[string][]Response `json:"responses"`
}

type Response struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

type Invocation struct {
	Args       []string `json:"args"`
	Dir        string   `json:"dir"`
	Subcommand string   `json:"subcommand"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Responses == nil {
		cfg.Responses = map[string][]Response{}
	}
	return &cfg, nil
}

func WriteConfig(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func ReadInvocations(path string) ([]Invocation, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open records: %w", err)
	}
	defer f.Close()

	var out []Invocation
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var inv Invocation
		if err := json.Unmarshal([]byte(line), &inv); err != nil {
			return nil, fmt.Errorf("parse record: %w", err)
		}
		out = append(out, inv)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan records: %w", err)
	}
	return out, nil
}

func AppendInvocation(path string, inv Invocation) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open records for append: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(inv)
	if err != nil {
		return fmt.Errorf("marshal invocation: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append invocation: %w", err)
	}
	return nil
}

func Classify(args []string) string {
	if len(args) == 0 {
		return ""
	}
	if len(args) >= 2 {
		switch args[0] + " " + args[1] {
		case "image inspect", "container inspect":
			return args[0] + " " + args[1]
		}
	}
	return args[0]
}

func ResponseFor(cfg *Config, records []Invocation, subcommand string) (Response, bool) {
	choices := cfg.Responses[subcommand]
	if len(choices) == 0 {
		return Response{}, false
	}
	idx := 0
	for _, rec := range records {
		if rec.Subcommand == subcommand {
			idx++
		}
	}
	if idx >= len(choices) {
		idx = len(choices) - 1
	}
	return choices[idx], true
}
