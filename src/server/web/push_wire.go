package web

import (
	"encoding/json"

	"github.com/takezoh/agent-grid/client/proto"
)

const surfaceUnsubscribedControlData = "surface-unsubscribed"

type surfaceUnsubscribedControl struct {
	K         string `json:"k"`
	Data      string `json:"data"`
	SessionID string `json:"sessionId"`
}

func encodePushFrame(push proto.PushNotification) []byte {
	if push.Cmd != proto.CmdNameSurfaceUnsubscribe {
		return nil
	}
	body, ok := push.Body.(proto.RespSurfaceUnsubscribed)
	if !ok {
		return nil
	}
	b, err := json.Marshal(surfaceUnsubscribedControl{
		K:         "c",
		Data:      surfaceUnsubscribedControlData,
		SessionID: body.SessionID,
	})
	if err != nil {
		return nil
	}
	return b
}
