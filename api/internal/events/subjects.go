package events

// NATS subject constants for StreamSpace events.
// Format: streamspace.<domain>.<action>[.<platform>]

const (
	// Session events
	SubjectSessionCreate    = "streamspace.session.create"
	SubjectSessionDelete    = "streamspace.session.delete"
	SubjectSessionHibernate = "streamspace.session.hibernate"
	SubjectSessionWake      = "streamspace.session.wake"
	SubjectSessionStatus    = "streamspace.session.status"

	// Application events
	SubjectAppInstall   = "streamspace.app.install"
	SubjectAppUninstall = "streamspace.app.uninstall"
	SubjectAppStatus    = "streamspace.app.status"

	// Template events
	SubjectTemplateCreate = "streamspace.template.create"
	SubjectTemplateDelete = "streamspace.template.delete"

	// Node management events
	SubjectNodeCordon   = "streamspace.node.cordon"
	SubjectNodeUncordon = "streamspace.node.uncordon"
	SubjectNodeDrain    = "streamspace.node.drain"

	// Controller events
	SubjectControllerHeartbeat   = "streamspace.controller.heartbeat"
	SubjectControllerSyncRequest = "streamspace.controller.sync.request"

	// Dead letter queue prefix
	SubjectDLQPrefix = "streamspace.dlq"
)

// PlatformSubject returns a platform-specific subject.
// Example: SubjectWithPlatform(SubjectSessionCreate, PlatformKubernetes)
// Returns: "streamspace.session.create.kubernetes"
func SubjectWithPlatform(subject, platform string) string {
	return subject + "." + platform
}

// DLQSubject returns the dead letter queue subject for a given subject.
// Example: DLQSubject(SubjectSessionCreate)
// Returns: "streamspace.dlq.streamspace.session.create"
func DLQSubject(subject string) string {
	return SubjectDLQPrefix + "." + subject
}
