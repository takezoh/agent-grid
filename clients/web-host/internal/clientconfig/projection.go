package clientconfig

type Projection struct {
	SchemaVersion int                `json:"schema_version"`
	Servers       []ServerProjection `json:"servers"`
}

type ServerProjection struct {
	ServerID         string `json:"serverId"`
	DisplayName      string `json:"displayName"`
	Enabled          bool   `json:"enabled"`
	DefaultExecution bool   `json:"defaultExecution"`
}

func (snapshot Snapshot) Projection() Projection {
	servers := make([]ServerProjection, 0, len(snapshot.Servers))
	for _, server := range snapshot.Servers {
		servers = append(servers, ServerProjection{
			ServerID:         server.ID,
			DisplayName:      server.DisplayName,
			Enabled:          server.Enabled,
			DefaultExecution: server.DefaultExecution,
		})
	}
	return Projection{SchemaVersion: ServersSchemaVersion, Servers: servers}
}
