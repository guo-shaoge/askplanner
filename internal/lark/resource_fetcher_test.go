package lark

import (
	"archive/zip"
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type fakeMessageResourceClient struct {
	resp *larkim.GetMessageResourceResp
}

func (f fakeMessageResourceClient) Get(ctx context.Context, req *larkim.GetMessageResourceReq, options ...larkcore.RequestOptionFunc) (*larkim.GetMessageResourceResp, error) {
	return f.resp, nil
}

func fakeZipResourceResponse() *larkim.GetMessageResourceResp {
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	file, _ := writer.Create("sql_meta.toml")
	_, _ = file.Write([]byte("trace"))
	_ = writer.Close()
	return &larkim.GetMessageResourceResp{
		CodeError: larkcore.CodeError{Code: 0},
		File:      bytes.NewReader(buf.Bytes()),
		FileName:  "trace.zip",
	}
}

func TestResourceFetcherStoresFileInTargetDir(t *testing.T) {
	fetcher := &ResourceFetcher{
		Client: fakeMessageResourceClient{resp: &larkim.GetMessageResourceResp{
			CodeError: larkcore.CodeError{Code: 0},
			File:      bytes.NewBufferString("payload"),
			FileName:  "trace.zip",
		}},
		MaxBytes: 1024,
	}
	targetDir := t.TempDir()
	saved, err := fetcher.Fetch(context.Background(), "om_1", AttachmentRef{
		Kind:         AttachmentKindFile,
		ResourceType: "file",
		ResourceKey:  "file_1",
	}, targetDir)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if saved.OriginalName != "trace.zip" {
		t.Fatalf("unexpected original name: %q", saved.OriginalName)
	}
	if filepath.Dir(saved.SavedPath) != targetDir {
		t.Fatalf("unexpected saved path: %q", saved.SavedPath)
	}
}

func TestResourceFetcherRejectsFailedAPIResponseEvenWithFileBody(t *testing.T) {
	fetcher := &ResourceFetcher{
		Client: fakeMessageResourceClient{resp: &larkim.GetMessageResourceResp{
			CodeError: larkcore.CodeError{Code: 999, Msg: "denied"},
			File:      bytes.NewBufferString("payload"),
			FileName:  "trace.zip",
		}},
		MaxBytes: 1024,
	}

	_, err := fetcher.Fetch(context.Background(), "om_1", AttachmentRef{
		Kind:         AttachmentKindFile,
		ResourceType: "file",
		ResourceKey:  "file_1",
	}, t.TempDir())
	if err == nil {
		t.Fatalf("expected fetch to fail on unsuccessful API response")
	}
	if !strings.Contains(err.Error(), "code=999") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResourceFetcherRejectsMissingFileBody(t *testing.T) {
	fetcher := &ResourceFetcher{
		Client: fakeMessageResourceClient{resp: &larkim.GetMessageResourceResp{
			CodeError: larkcore.CodeError{Code: 0},
			FileName:  "trace.zip",
		}},
		MaxBytes: 1024,
	}

	_, err := fetcher.Fetch(context.Background(), "om_1", AttachmentRef{
		Kind:         AttachmentKindFile,
		ResourceType: "file",
		ResourceKey:  "file_1",
	}, t.TempDir())
	if err == nil {
		t.Fatalf("expected fetch to fail on empty file body")
	}
	if !strings.Contains(err.Error(), "empty file body") {
		t.Fatalf("unexpected error: %v", err)
	}
}
