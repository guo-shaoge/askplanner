package larkbot

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"lab/askplanner/internal/config"
	"lab/askplanner/internal/workspace"
)

type fakeWebsocketClient struct {
	start func(context.Context) error
}

func (f fakeWebsocketClient) Start(ctx context.Context) error {
	return f.start(ctx)
}

func TestAppRunCancelsOtherBotsOnFirstError(t *testing.T) {
	manager := newTestWorkspaceManager(t)
	cancelObserved := make(chan struct{}, 1)
	errBoom := errors.New("boom")

	app := &App{
		shared: sharedServices{workspace: manager},
		bots: []*botRuntime{
			{
				bot:         botIdentity{key: "bot-a"},
				dedup:       &messageDedup{},
				dedupMaxAge: time.Minute,
				client: fakeWebsocketClient{start: func(ctx context.Context) error {
					<-ctx.Done()
					cancelObserved <- struct{}{}
					return nil
				}},
			},
			{
				bot:         botIdentity{key: "bot-b"},
				dedup:       &messageDedup{},
				dedupMaxAge: time.Minute,
				client: fakeWebsocketClient{start: func(ctx context.Context) error {
					return errBoom
				}},
			},
		},
	}

	err := app.Run(context.Background())
	if !errors.Is(err, errBoom) {
		t.Fatalf("Run error = %v, want %v", err, errBoom)
	}

	select {
	case <-cancelObserved:
	case <-time.After(2 * time.Second):
		t.Fatal("expected peer bot to observe cancellation")
	}
}

func TestMessageDedupIsIndependentPerBotRuntime(t *testing.T) {
	first := &messageDedup{}
	second := &messageDedup{}

	if first.isDuplicate("m-1") {
		t.Fatal("first runtime treated first message as duplicate")
	}
	if second.isDuplicate("m-1") {
		t.Fatal("second runtime treated first message as duplicate")
	}
	if !first.isDuplicate("m-1") {
		t.Fatal("first runtime did not dedup repeated message")
	}
}

func newTestWorkspaceManager(t *testing.T) *workspace.Manager {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{
		WorkspaceRoot:                     filepath.Join(root, "workspaces"),
		FeishuFileDir:                     filepath.Join(root, "uploads"),
		ClinicStoreDir:                    filepath.Join(root, "clinic"),
		WorkspaceRepoTidbURL:              "https://example.invalid/tidb.git",
		WorkspaceRepoTidbDefaultRef:       "master",
		WorkspaceRepoAgentRulesURL:        "https://example.invalid/agent-rules.git",
		WorkspaceRepoAgentRulesDefaultRef: "main",
		WorkspaceRepoTidbDocsURL:          "https://example.invalid/docs.git",
		WorkspaceRepoTidbDocsDefaultRef:   "master",
	}
	manager, err := workspace.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}
	return manager
}
