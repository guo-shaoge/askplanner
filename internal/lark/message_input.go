package lark

type AttachmentRef struct {
	Kind         string
	ResourceType string
	ResourceKey  string
}

type ParsedMessage struct {
	MessageID   string
	MessageType string
	UserText    string
	Attachments []AttachmentRef
}

const (
	AttachmentKindImage = "image"
	AttachmentKindFile  = "file"
)
