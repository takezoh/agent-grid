package fakedocker

const (
	DefaultContainerID = "fake-container-1234567890ab"
	DefaultPSLine      = "fake-container-1234567890ab\trunning\tfake-mount-hash"
	DefaultInspectJSON = `[{"Id":"fake-container-1234567890ab","State":{"Status":"running"}}]`
)
