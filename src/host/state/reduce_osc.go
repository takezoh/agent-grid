package state

import "fmt"

// reduceFrameOsc handles an OSC notification event fired by the FrameTap reader.
// Every observed OSC sequence is appended to the EVENTS log so operators can
// trace what each frame is emitting. OSC 0/2 (window title) is also routed to
// the driver via DEvFrameOsc; OSC 9;4 is routed as progress evidence;
// OSC 9/99/777 and bare BEL increment the shared one-shot bell signal.
func reduceFrameOsc(s State, e EvFrameOsc) (State, []Effect) {
	kind := e.Kind
	if kind == TerminalSignalUnknown {
		switch e.Cmd {
		case 0, 2:
			kind = TerminalSignalWindowTitle
		default:
			kind = TerminalSignalNotification
		}
	}
	if kind == TerminalSignalWindowTitle {
		if e.Title == "" {
			return s, nil
		}
		if _, _, _, ok := findFrame(s, e.FrameID); !ok {
			return s, nil
		}
		effs := []Effect{EffEventLogAppend{
			FrameID: e.FrameID,
			Line:    fmt.Sprintf("[osc%d] %s", e.Cmd, e.Title),
		}}
		next, dEffs, _ := stepDriver(s, e.FrameID, DEvFrameOsc{Cmd: e.Cmd, Kind: kind, Title: e.Title, Now: e.Now})
		effs = append(effs, dEffs...)
		effs = appendMissingSessionRefreshEffects(effs)
		return next, effs
	}

	if kind == TerminalSignalProgress {
		if _, _, _, ok := findFrame(s, e.FrameID); !ok {
			return s, nil
		}
		next, dEffs, _ := stepDriver(s, e.FrameID, DEvFrameOsc{
			Cmd:           e.Cmd,
			Kind:          kind,
			ProgressState: e.ProgressState,
			ProgressValue: e.ProgressValue,
			Now:           e.Now,
		})
		effs := append([]Effect{EffEventLogAppend{
			FrameID: e.FrameID,
			Line:    fmt.Sprintf("[osc9;4] state=%d value=%d", e.ProgressState, e.ProgressValue),
		}}, dEffs...)
		return next, appendMissingSessionRefreshEffects(effs)
	}

	if kind == TerminalSignalBell {
		sessID, _, _, ok := findFrame(s, e.FrameID)
		if !ok {
			return s, nil
		}
		s = incrementStatusSignal(s, sessID)
		return s, []Effect{
			EffEventLogAppend{FrameID: e.FrameID, Line: "[bell]"},
			EffPersistSnapshot{},
			EffBroadcastSessionsChanged{},
		}
	}

	if e.Title == "" && e.Body == "" {
		return s, nil
	}
	sessID, _, _, ok := findFrame(s, e.FrameID)
	if !ok {
		return s, nil
	}
	s = incrementStatusSignal(s, sessID)
	return s, []Effect{
		EffRecordNotification{
			SessionID: sessID,
			FrameID:   e.FrameID,
			Cmd:       e.Cmd,
			Title:     e.Title,
			Body:      e.Body,
		},
		EffPersistSnapshot{},
		EffBroadcastSessionsChanged{},
	}
}

func incrementStatusSignal(s State, sessID SessionID) State {
	s.StatusSignals = cloneStatusSignals(s.StatusSignals)
	s.StatusSignals[sessID]++
	return s
}

// reduceFramePrompt routes an OSC 133 semantic-prompt event to the owning
// frame's driver as DEvFramePrompt and writes a line to the EVENTS log so
// operators can see prompt-phase transitions in real time.
func reduceFramePrompt(s State, e EvFramePrompt) (State, []Effect) {
	s.Now = e.Now
	if _, _, _, ok := findFrame(s, e.FrameID); !ok {
		return s, nil
	}
	effs := []Effect{EffEventLogAppend{
		FrameID: e.FrameID,
		Line:    promptEventLogLine(e.Phase, e.ExitCode),
	}}
	next, dEffs, _ := stepDriver(s, e.FrameID, DEvFramePrompt{
		Phase:    e.Phase,
		ExitCode: e.ExitCode,
		Now:      e.Now,
	})
	effs = append(effs, dEffs...)
	effs = appendMissingSessionRefreshEffects(effs)
	return next, effs
}

func appendMissingSessionRefreshEffects(effs []Effect) []Effect {
	var hasPersist, hasBroadcast bool
	for _, eff := range effs {
		switch eff.(type) {
		case EffPersistSnapshot:
			hasPersist = true
		case EffBroadcastSessionsChanged:
			hasBroadcast = true
		}
	}
	if !hasPersist {
		effs = append(effs, EffPersistSnapshot{})
	}
	if !hasBroadcast {
		effs = append(effs, EffBroadcastSessionsChanged{})
	}
	return effs
}

func promptEventLogLine(phase PromptPhase, exitCode *int) string {
	name := promptPhaseName(phase)
	if phase == PromptPhaseComplete && exitCode != nil {
		return fmt.Sprintf("[osc133] phase=%s exit=%d", name, *exitCode)
	}
	return fmt.Sprintf("[osc133] phase=%s", name)
}

func promptPhaseName(p PromptPhase) string {
	switch p {
	case PromptPhaseStart:
		return "start"
	case PromptPhaseInput:
		return "input"
	case PromptPhaseCommand:
		return "command"
	case PromptPhaseComplete:
		return "complete"
	default:
		return "none"
	}
}
