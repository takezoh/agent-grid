package runtime

import (
	"encoding/json"
	"time"

	"github.com/takezoh/agent-grid/client/proto"
	"github.com/takezoh/agent-grid/client/state"
)

func (r *Runtime) directHookEvent(token, hook string, ts time.Time, payload json.RawMessage) (proto.Response, error) {
	source, err := r.frameSourceForToken(token)
	if err != nil {
		return nil, err
	}
	if ts.IsZero() {
		ts = time.Now()
	}
	r.Enqueue(state.EvDriverEvent{
		ConnID:    0,
		ReqID:     "",
		Event:     hook,
		Timestamp: ts,
		SenderID:  source,
		Payload:   payload,
	})
	return proto.RespOK{}, nil
}
