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
	PublicID     string   `json:"public_id,omitempty"`
	OriginalName string   `json:"original_name,omitempty"`
	SavedPath    string   `json:"saved_path,omitempty"`
	ExtractedDir string   `json:"extracted_dir,omitempty"`
	Notes        []string `json:"notes,omitempty"`
}

type BundleMetadata struct {
	ConversationKey   string                     `json:"conversation_key"`
	MessageID         string                     `json:"message_id"`
	MessageType       string                     `json:"message_type"`
	CreatedAt         time.Time                  `json:"created_at"`
	ExpiresAt         time.Time                  `json:"expires_at"`
	RawPublicID       string                     `json:"raw_public_id,omitempty"`
	ExtractedPublicID string                     `json:"extracted_public_id,omitempty"`
	Attachments       []BundleAttachmentMetadata `json:"attachments,omitempty"`
}

type Bundle struct {
	Dir               string
	RawDir            string
	ExtractedDir      string
	MetaPath          string
	RawPublicID       string
	ExtractedPublicID string
	Meta              BundleMetadata
}

type ResolvedReference struct {
	PublicID string
	Dir      string
	MetaPath string
	Meta     BundleMetadata
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
	rawPublicID := publicReferenceID(conversationKey, messageID, "raw")
	extractedPublicID := publicReferenceID(conversationKey, messageID, "extracted")
	bundle := &Bundle{
		Dir:               dir,
		RawDir:            rawDir,
		ExtractedDir:      extractedDir,
		MetaPath:          filepath.Join(dir, "meta.json"),
		RawPublicID:       rawPublicID,
		ExtractedPublicID: extractedPublicID,
		Meta: BundleMetadata{
			ConversationKey:   strings.TrimSpace(conversationKey),
			MessageID:         strings.TrimSpace(messageID),
			MessageType:       strings.TrimSpace(messageType),
			CreatedAt:         createdAt,
			ExpiresAt:         createdAt.Add(ttl),
			RawPublicID:       rawPublicID,
			ExtractedPublicID: extractedPublicID,
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

func publicReferenceID(conversationKey, messageID, leaf string) string {
	return strings.TrimSpace(conversationKey) + "/" + strings.TrimSpace(messageID) + "/" + strings.TrimSpace(leaf)
}

func (s *BundleStore) Resolve(publicID string) (*ResolvedReference, error) {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return nil, fmt.Errorf("public id is required")
	}
	dir, err := publicIDToPath(s.Root, publicID)
	if err != nil {
		return nil, err
	}
	leaf := filepath.Base(dir)
	if leaf != "raw" && leaf != "extracted" {
		return nil, fmt.Errorf("unsupported public id leaf: %s", publicID)
	}
	metaPath := filepath.Join(filepath.Dir(dir), "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("read bundle metadata for %s: %w", publicID, err)
	}
	var meta BundleMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse bundle metadata for %s: %w", publicID, err)
	}
	expected := meta.RawPublicID
	if leaf == "extracted" {
		expected = meta.ExtractedPublicID
	}
	if expected != publicID {
		return nil, fmt.Errorf("public id does not match bundle metadata: %s", publicID)
	}
	return &ResolvedReference{
		PublicID: publicID,
		Dir:      dir,
		MetaPath: metaPath,
		Meta:     meta,
	}, nil
}

func publicIDToPath(root, publicID string) (string, error) {
	parts := strings.Split(publicID, "/")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid public id: %s", publicID)
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("invalid public id path segment: %s", publicID)
		}
		if sanitizePathComponent(part) != part {
			return "", fmt.Errorf("unsafe public id path segment: %s", publicID)
		}
	}
	joined := filepath.Join(append([]string{root}, parts...)...)
	rel, err := filepath.Rel(filepath.Clean(root), joined)
	if err != nil {
		return "", fmt.Errorf("resolve public id %s: %w", publicID, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("public id escapes bundle root: %s", publicID)
	}
	return joined, nil
}
