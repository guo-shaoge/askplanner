package lark

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type MessageResourceClient interface {
	Get(ctx context.Context, req *larkim.GetMessageResourceReq, options ...larkcore.RequestOptionFunc) (*larkim.GetMessageResourceResp, error)
}

type SavedResource struct {
	OriginalName string
	SavedPath    string
}

type ResourceFetcher struct {
	Client   MessageResourceClient
	MaxBytes int64
}

func (f *ResourceFetcher) Fetch(ctx context.Context, messageID string, ref AttachmentRef, targetDir string) (SavedResource, error) {
	req := larkim.NewGetMessageResourceReqBuilder().
		MessageId(strings.TrimSpace(messageID)).
		FileKey(strings.TrimSpace(ref.ResourceKey)).
		Type(strings.TrimSpace(ref.ResourceType)).
		Build()
	resp, err := f.Client.Get(ctx, req)
	if err != nil {
		return SavedResource{}, fmt.Errorf("fetch lark message resource: %w", err)
	}
	if !resp.Success() {
		return SavedResource{}, fmt.Errorf("lark message resource API error: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	if resp.File == nil {
		return SavedResource{}, fmt.Errorf("lark message resource API returned an empty file body")
	}

	data, err := readAllLimited(resp.File, f.MaxBytes)
	if err != nil {
		return SavedResource{}, err
	}
	name := sanitizeFileName(resp.FileName)
	if name == "" {
		name = fallbackResourceName(ref)
	}
	path := uniqueFilePath(filepath.Join(targetDir, name))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return SavedResource{}, fmt.Errorf("write resource file: %w", err)
	}
	return SavedResource{
		OriginalName: name,
		SavedPath:    path,
	}, nil
}

func readAllLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("empty message resource body")
	}
	if maxBytes <= 0 {
		return io.ReadAll(r)
	}
	limited := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read message resource body: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("message resource exceeds configured limit of %d bytes", maxBytes)
	}
	return data, nil
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = filepath.Base(name)
	if name == "." || name == string(filepath.Separator) {
		return ""
	}
	return name
}

func fallbackResourceName(ref AttachmentRef) string {
	ext := ".bin"
	if ref.ResourceType == "image" {
		ext = ".img"
	}
	return sanitizePathComponent(ref.Kind+"_"+ref.ResourceKey) + ext
}

func uniqueFilePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	dir := filepath.Dir(path)
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)
	for i := 1; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
