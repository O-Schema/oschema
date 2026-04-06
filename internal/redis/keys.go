package redis

import "fmt"

const (
	QueueStream      = "oschema:queue"
	QueueGroup       = "oschema-workers"
	DeadLetterStream = "oschema:deadletter"
	EventIndex       = "oschema:event_index"
	AttemptPrefix    = "oschema:attempts:"
	DedupePrefix     = "dedupe:"
)

func EventStreamKey(source string) string {
	return fmt.Sprintf("oschema:events:%s", source)
}

func DedupeKey(source, externalID string) string {
	return DedupePrefix + source + ":" + externalID
}
