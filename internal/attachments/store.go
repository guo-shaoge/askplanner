package attachments

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

const manifestFileName = ".index.json"

type ItemType string

const (
	ItemTypeFile   ItemType = "file"
	ItemTypeImage  ItemType = "image"
	ItemTypeZipDir ItemType = "extracted_zip_dir"
)

type ImportRequest struct {
	UserKey      string
	OriginalName string
	MessageID    string
	FileKey      string
	ResourceType string
	SourcePath   string
	ImportedAt   time.Time
}

type Item struct {
	Name         string    `json:"name"`
	OriginalName string    `json:"original_name,omitempty"`
	Type         ItemType  `json:"type"`
	MessageID    string    `json:"message_id,omitempty"`
	FileKey      string    `json:"file_key,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type SaveResult struct {
	UserKey   string
	UserDir   string
	Item      Item
	Replaced  bool
	Evicted   []Item
	Library   Library
	SourceWas string
}

type Library struct {
	UserKey string
	RootDir string
	Items   []Item
}

type Manager struct {
	rootDir  string
	maxItems int

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewManager(rootDir string, maxItems int) (*Manager, error) {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return nil, fmt.Errorf("attachment root dir is empty")
	}
	if maxItems <= 0 {
		maxItems = 100
	}
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("create attachment root dir: %w", err)
	}
	return &Manager{
		rootDir:  rootDir,
		maxItems: maxItems,
		locks:    make(map[string]*sync.Mutex),
	}, nil
}

func (m *Manager) RootDir() string {
	return m.rootDir
}

func (m *Manager) MaxItems() int {
	return m.maxItems
}

func (m *Manager) UserDir(userKey string) string {
	return filepath.Join(m.rootDir, sanitizePathSegment(userKey, "user"))
}

func (m *Manager) Import(req ImportRequest) (*SaveResult, error) {
	userKey := sanitizePathSegment(req.UserKey, "")
	if userKey == "" {
		return nil, fmt.Errorf("attachment user key is empty")
	}
	if strings.TrimSpace(req.SourcePath) == "" {
		return nil, fmt.Errorf("attachment source path is empty")
	}
	lock := m.userLock(userKey)
	lock.Lock()
	defer lock.Unlock()

	importedAt := req.ImportedAt
	if importedAt.IsZero() {
		importedAt = time.Now()
	}

	userDir := m.UserDir(userKey)
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return nil, fmt.Errorf("create user attachment dir: %w", err)
	}

	items, err := loadOrRebuildManifest(userDir)
	if err != nil {
		return nil, err
	}

	targetName, itemType, err := determineTarget(req, importedAt)
	if err != nil {
		return nil, err
	}
	targetPath := filepath.Join(userDir, targetName)
	replaced := itemExists(items, targetName)

	if strings.EqualFold(filepath.Ext(targetName), ".zip") {
		return nil, fmt.Errorf("zip target should not keep .zip extension: %s", targetName)
	}

	if err := replaceManagedItem(req.SourcePath, targetPath, req.OriginalName, itemType); err != nil {
		return nil, err
	}

	item := Item{
		Name:         targetName,
		OriginalName: strings.TrimSpace(req.OriginalName),
		Type:         itemType,
		MessageID:    strings.TrimSpace(req.MessageID),
		FileKey:      strings.TrimSpace(req.FileKey),
		CreatedAt:    importedAt.UTC(),
	}
	if item.OriginalName == "" {
		item.OriginalName = targetName
	}
	items = upsertItem(items, item)

	evicted, items, err := enforceQuota(userDir, items, m.maxItems)
	if err != nil {
		return nil, err
	}
	if err := saveManifest(userDir, items); err != nil {
		return nil, err
	}

	library := Library{
		UserKey: userKey,
		RootDir: userDir,
		Items:   newestFirst(items),
	}
	return &SaveResult{
		UserKey:   userKey,
		UserDir:   userDir,
		Item:      item,
		Replaced:  replaced,
		Evicted:   evicted,
		Library:   library,
		SourceWas: req.SourcePath,
	}, nil
}

func (m *Manager) Snapshot(userKey string) (Library, error) {
	userKey = sanitizePathSegment(userKey, "")
	if userKey == "" {
		return Library{}, fmt.Errorf("attachment user key is empty")
	}
	lock := m.userLock(userKey)
	lock.Lock()
	defer lock.Unlock()

	userDir := m.UserDir(userKey)
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return Library{}, fmt.Errorf("create user attachment dir: %w", err)
	}
	items, err := loadOrRebuildManifest(userDir)
	if err != nil {
		return Library{}, err
	}
	return Library{
		UserKey: userKey,
		RootDir: userDir,
		Items:   newestFirst(items),
	}, nil
}

func (m *Manager) userLock(userKey string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	lock, ok := m.locks[userKey]
	if !ok {
		lock = &sync.Mutex{}
		m.locks[userKey] = lock
	}
	return lock
}

func determineTarget(req ImportRequest, importedAt time.Time) (string, ItemType, error) {
	resourceType := strings.TrimSpace(req.ResourceType)
	originalName := strings.TrimSpace(req.OriginalName)
	switch resourceType {
	case "image":
		ext := strings.ToLower(filepath.Ext(originalName))
		if ext == "" {
			ext = ".png"
		}
		name := fmt.Sprintf("image_%s_%s%s",
			importedAt.Format("20060102_150405"),
			sanitizePathSegment(req.MessageID, "image"),
			ext,
		)
		return sanitizeFileName(name, "image"+ext), ItemTypeImage, nil
	case "file":
		if strings.EqualFold(filepath.Ext(originalName), ".zip") {
			base := strings.TrimSuffix(filepath.Base(originalName), filepath.Ext(originalName))
			base = sanitizePathSegment(base, "archive")
			return base, ItemTypeZipDir, nil
		}
		return sanitizeFileName(originalName, "attachment.bin"), ItemTypeFile, nil
	default:
		return "", "", fmt.Errorf("unsupported attachment resource type: %s", resourceType)
	}
}

func replaceManagedItem(sourcePath, targetPath, originalName string, itemType ItemType) error {
	switch itemType {
	case ItemTypeZipDir:
		if err := extractZipAtomically(sourcePath, targetPath); err != nil {
			return fmt.Errorf("extract zip %s: %w", originalName, err)
		}
	default:
		if err := copyFileAtomically(sourcePath, targetPath); err != nil {
			return fmt.Errorf("store attachment %s: %w", originalName, err)
		}
	}
	return nil
}

func copyFileAtomically(sourcePath, targetPath string) error {
	tmpFile, err := os.CreateTemp(filepath.Dir(targetPath), ".attachment-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	src, err := os.Open(sourcePath)
	if err != nil {
		_ = tmpFile.Close()
		return err
	}
	defer src.Close()

	if _, err := io.Copy(tmpFile, src); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.RemoveAll(targetPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tmpPath, targetPath)
}

func extractZipAtomically(sourcePath, targetPath string) error {
	tempDir, err := os.MkdirTemp(filepath.Dir(targetPath), ".extract-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	reader, err := zip.OpenReader(sourcePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		name := filepath.Clean(file.Name)
		if name == "." || name == "" {
			continue
		}
		destPath := filepath.Join(tempDir, name)
		if err := ensureInDir(tempDir, destPath); err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			src.Close()
			return err
		}
		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			src.Close()
			return err
		}
		if err := dst.Close(); err != nil {
			src.Close()
			return err
		}
		if err := src.Close(); err != nil {
			return err
		}
	}

	if err := os.RemoveAll(targetPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tempDir, targetPath)
}

func ensureInDir(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("zip entry escapes target dir: %s", path)
	}
	return nil
}

func itemExists(items []Item, name string) bool {
	for _, item := range items {
		if item.Name == name {
			return true
		}
	}
	return false
}

func upsertItem(items []Item, item Item) []Item {
	out := make([]Item, 0, len(items)+1)
	for _, existing := range items {
		if existing.Name == item.Name {
			continue
		}
		out = append(out, existing)
	}
	out = append(out, item)
	return out
}

func enforceQuota(userDir string, items []Item, maxItems int) ([]Item, []Item, error) {
	if maxItems <= 0 || len(items) <= maxItems {
		return nil, items, nil
	}
	sorted := make([]Item, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
	})
	evictCount := len(sorted) - maxItems
	evicted := append([]Item(nil), sorted[:evictCount]...)
	keepNames := make(map[string]struct{}, len(sorted)-evictCount)
	for _, item := range sorted[evictCount:] {
		keepNames[item.Name] = struct{}{}
	}
	for _, item := range evicted {
		if err := os.RemoveAll(filepath.Join(userDir, item.Name)); err != nil && !os.IsNotExist(err) {
			return nil, nil, err
		}
	}
	kept := make([]Item, 0, len(items)-evictCount)
	for _, item := range items {
		if _, ok := keepNames[item.Name]; ok {
			kept = append(kept, item)
		}
	}
	return evicted, kept, nil
}

func newestFirst(items []Item) []Item {
	out := append([]Item(nil), items...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].Name < out[j].Name
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func loadOrRebuildManifest(userDir string) ([]Item, error) {
	items, err := loadManifest(userDir)
	switch {
	case err == nil:
		if manifestMatchesDisk(userDir, items) {
			return items, nil
		}
	case !os.IsNotExist(err):
		return nil, err
	}
	items, err = rebuildManifest(userDir)
	if err != nil {
		return nil, err
	}
	if err := saveManifest(userDir, items); err != nil {
		return nil, err
	}
	return items, nil
}

func loadManifest(userDir string) ([]Item, error) {
	data, err := os.ReadFile(filepath.Join(userDir, manifestFileName))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("parse attachment manifest: %w", err)
	}
	return items, nil
}

func saveManifest(userDir string, items []Item) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("encode attachment manifest: %w", err)
	}
	tmpFile, err := os.CreateTemp(userDir, ".index-*.json")
	if err != nil {
		return fmt.Errorf("create attachment manifest temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write attachment manifest: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close attachment manifest temp file: %w", err)
	}
	return os.Rename(tmpPath, filepath.Join(userDir, manifestFileName))
}

func manifestMatchesDisk(userDir string, items []Item) bool {
	entries, err := os.ReadDir(userDir)
	if err != nil {
		return false
	}
	names := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if shouldSkipManagedName(name) {
			continue
		}
		names[name] = struct{}{}
	}
	if len(names) != len(items) {
		return false
	}
	for _, item := range items {
		if _, ok := names[item.Name]; !ok {
			return false
		}
	}
	return true
}

func rebuildManifest(userDir string) ([]Item, error) {
	entries, err := os.ReadDir(userDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read attachment dir: %w", err)
	}
	items := make([]Item, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if shouldSkipManagedName(name) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat attachment entry %s: %w", name, err)
		}
		itemType := ItemTypeFile
		if entry.IsDir() {
			itemType = ItemTypeZipDir
		}
		items = append(items, Item{
			Name:         name,
			OriginalName: name,
			Type:         itemType,
			CreatedAt:    info.ModTime().UTC(),
		})
	}
	return items, nil
}

func shouldSkipManagedName(name string) bool {
	return name == manifestFileName || strings.HasPrefix(name, ".index-") || strings.HasPrefix(name, ".attachment-") || strings.HasPrefix(name, ".extract-")
}

func sanitizeFileName(name, fallback string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = fallback
	}
	base := filepath.Base(name)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	stem = sanitizePathSegment(stem, fallback)
	ext = sanitizeExtension(ext)
	if ext == "" && strings.Contains(fallback, ".") {
		ext = sanitizeExtension(filepath.Ext(fallback))
	}
	if ext == "" {
		ext = ".bin"
	}
	return stem + ext
}

func sanitizeExtension(ext string) string {
	ext = strings.TrimSpace(ext)
	if ext == "" {
		return ""
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	var b strings.Builder
	b.WriteByte('.')
	for _, r := range ext[1:] {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		case r == '_', r == '-':
			b.WriteRune(r)
		}
	}
	if b.Len() == 1 {
		return ""
	}
	return b.String()
}

func sanitizePathSegment(s, fallback string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		return fallback
	}
	return out
}
