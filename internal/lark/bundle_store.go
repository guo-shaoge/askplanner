package lark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BundleAttachmentMetadata struct {
	Kind         string   `json:"kind"`
	OriginalName string   `json:"original_name,omitempty"`
	SavedPath    string   `json:"saved_path,omitempty"`
	ExtractedDir string   `json:"extracted_dir,omitempty"`
	Notes        []string `json:"notes,omitempty"`
}

type BundleMetadata struct {
	ConversationKey string                     `json:"conversation_key"`
	MessageID       string                     `json:"message_id"`
	MessageType     string                     `json:"message_type"`
	CreatedAt       time.Time                  `json:"created_at"`
	ExpiresAt       time.Time                  `json:"expires_at"`
	Attachments     []BundleAttachmentMetadata `json:"attachments,omitempty"`
}

type Bundle struct {
	Dir          string
	RawDir       string
	ExtractedDir string
	MetaPath     string
	Meta         BundleMetadata
}

type BundleStore struct {
	Root string
}

func NewBundleStore(root string) *BundleStore {
	return &BundleStore{Root: root}
}

func (s *BundleStore) Create(conversationKey, messageID, messageType string, ttl time.Duration) (*Bundle, error) {
	conversationDir := sanitizePathComponent(conversationKey)
	messageDir := sanitizePathComponent(messageID)
	if conversationDir == "" || messageDir == "" {
		return nil, fmt.Errorf("conversation key and message id are required")
	}

	dir := filepath.Join(s.Root, conversationDir, messageDir)
	rawDir := filepath.Join(dir, "raw")
	extractedDir := filepath.Join(dir, "extracted")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return nil, fmt.Errorf("create bundle raw dir: %w", err)
	}
	if err := os.MkdirAll(extractedDir, 0o755); err != nil {
		return nil, fmt.Errorf("create bundle extracted dir: %w", err)
	}

	createdAt := time.Now().UTC()
	bundle := &Bundle{
		Dir:          dir,
		RawDir:       rawDir,
		ExtractedDir: extractedDir,
		MetaPath:     filepath.Join(dir, "meta.json"),
		Meta: BundleMetadata{
			ConversationKey: strings.TrimSpace(conversationKey),
			MessageID:       strings.TrimSpace(messageID),
			MessageType:     strings.TrimSpace(messageType),
			CreatedAt:       createdAt,
			ExpiresAt:       createdAt.Add(ttl),
		},
	}
	if err := bundle.Save(); err != nil {
		return nil, err
	}
	return bundle, nil
}

func (b *Bundle) AddAttachment(attachment BundleAttachmentMetadata) error {
	b.Meta.Attachments = append(b.Meta.Attachments, attachment)
	return b.Save()
}

func (b *Bundle) Save() error {
	data, err := json.MarshalIndent(b.Meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal bundle metadata: %w", err)
	}
	if err := os.WriteFile(b.MetaPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write bundle metadata: %w", err)
	}
	return nil
}

func sanitizePathComponent(component string) string {
	component = strings.TrimSpace(component)
	component = strings.ReplaceAll(component, "/", "_")
	component = strings.ReplaceAll(component, string(filepath.Separator), "_")
	component = strings.ReplaceAll(component, "\x00", "_")
	switch component {
	case "", ".", "..":
		return "_"
	default:
		return component
	}
}
