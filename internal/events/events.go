package events

import (
	"encoding/json"
)

const EVENT_CRAFT = "craft.event.json"

// Craft cloud resources using the module
type EventCraft struct {
	// Unique identity of event (job), use it follow up deployment status
	// It can be up to 128 letters long. The first character must be alphanumeric,
	// can contain uppercase and lowercase letters, numbers, hyphens (-), and
	// underscores (_).
	UID string `json:"uid,omitempty"`

	// Identity of deployable module
	// (e.g. github.com/fogfish/app)
	Module string `json:"module,omitempty"`

	// AWS CDK Context, the raw content of cdk.context.json file.
	Context json.RawMessage `json:"context,omitempty"`
}
