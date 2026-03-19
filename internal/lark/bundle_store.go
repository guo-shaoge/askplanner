package lark

import (
	"crypto/sha256"
	"encoding/hex"
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
	raw := strings.TrimSpace(conversationKey) + "/" + strings.TrimSpace(messageID) + "/" + strings.TrimSpace(leaf)
	sum := sha256.Sum256([]byte(raw))
	return "bundle-" + hex.EncodeToString(sum[:4])
}

func (s *BundleStore) Resolve(publicID string) (*ResolvedReference, error) {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return nil, fmt.Errorf("public id is required")
	}
	convEntries, err := os.ReadDir(s.Root)
	if err != nil {
		return nil, fmt.Errorf("read bundle root: %w", err)
	}
	for _, convEntry := range convEntries {
		if !convEntry.IsDir() {
			continue
		}
		convDir := filepath.Join(s.Root, convEntry.Name())
		msgEntries, err := os.ReadDir(convDir)
		if err != nil {
			continue
		}
		for _, msgEntry := range msgEntries {
			if !msgEntry.IsDir() {
				continue
			}
			metaPath := filepath.Join(convDir, msgEntry.Name(), "meta.json")
			data, err := os.ReadFile(metaPath)
			if err != nil {
				continue
			}
			var meta BundleMetadata
			if err := json.Unmarshal(data, &meta); err != nil {
				continue
			}
			var dir string
			if meta.RawPublicID == publicID {
				dir = filepath.Join(convDir, msgEntry.Name(), "raw")
			} else if meta.ExtractedPublicID == publicID {
				dir = filepath.Join(convDir, msgEntry.Name(), "extracted")
			} else {
				continue
			}
			return &ResolvedReference{
				PublicID: publicID,
				Dir:      dir,
				MetaPath: metaPath,
				Meta:     meta,
			}, nil
		}
	}
	return nil, fmt.Errorf("public id not found: %s", publicID)
}
