package larkbot

import (
	"context"
	"strings"
	"testing"

	"lab/askplanner/internal/clinic"
	"lab/askplanner/internal/codex"
	"lab/askplanner/internal/modelcmd"
	"lab/askplanner/internal/usererr"
	"lab/askplanner/internal/workspace"
)

type fakeResponder struct {
	answer            string
	err               error
	calls             int
	lastContext       context.Context
	lastConversation  string
	lastQuestion      string
	lastRuntime       codex.RuntimeContext
	modelState        codex.ModelState
	setModelResult    codex.ModelChangeResult
	setModelErr       error
	resetModelResult  codex.ModelChangeResult
	resetModelErr     error
	setEffortResult   codex.ModelChangeResult
	setEffortErr      error
	resetEffortResult codex.ModelChangeResult
	resetEffortErr    error
	modelCalls        int
	lastModel         string
	lastEffort        string
	lastModelAction   string
}

func (f *fakeResponder) AnswerWithContext(ctx context.Context, conversationKey, question string, runtime codex.RuntimeContext) (string, error) {
	f.calls++
	f.lastContext = ctx
	f.lastConversation = conversationKey
	f.lastQuestion = question
	f.lastRuntime = runtime
	if f.err != nil {
		return "", f.err
	}
	return f.answer, nil
}

func (f *fakeResponder) GetModelState(conversationKey string) codex.ModelState {
	f.modelCalls++
	f.lastConversation = conversationKey
	f.lastModelAction = "status"
	return f.modelState
}

func (f *fakeResponder) SetModel(conversationKey, model string) (codex.ModelChangeResult, error) {
	f.modelCalls++
	f.lastConversation = conversationKey
	f.lastModel = model
	f.lastModelAction = "set"
	if f.setModelErr != nil {
		return codex.ModelChangeResult{}, f.setModelErr
	}
	return f.setModelResult, nil
}

func (f *fakeResponder) ResetModel(conversationKey string) (codex.ModelChangeResult, error) {
	f.modelCalls++
	f.lastConversation = conversationKey
	f.lastModelAction = "reset"
	if f.resetModelErr != nil {
		return codex.ModelChangeResult{}, f.resetModelErr
	}
	return f.resetModelResult, nil
}

func (f *fakeResponder) SetReasoningEffort(conversationKey, effort string) (codex.ModelChangeResult, error) {
	f.modelCalls++
	f.lastConversation = conversationKey
	f.lastEffort = effort
	f.lastModelAction = "effort"
	if f.setEffortErr != nil {
		return codex.ModelChangeResult{}, f.setEffortErr
	}
	return f.setEffortResult, nil
}

func (f *fakeResponder) ResetReasoningEffort(conversationKey string) (codex.ModelChangeResult, error) {
	f.modelCalls++
	f.lastConversation = conversationKey
	f.lastModelAction = "reset-effort"
	if f.resetEffortErr != nil {
		return codex.ModelChangeResult{}, f.resetEffortErr
	}
	return f.resetEffortResult, nil
}

type fakePrefetcher struct {
	result       clinic.EnrichResult
	err          error
	passthrough  bool
	calls        int
	lastUserKey  string
	lastQuestion string
	lastRuntime  codex.RuntimeContext
}

func (f *fakePrefetcher) Enrich(ctx context.Context, userKey, question string, runtime codex.RuntimeContext) (clinic.EnrichResult, error) {
	f.calls++
	f.lastUserKey = userKey
	f.lastQuestion = question
	f.lastRuntime = runtime
	if f.err != nil {
		return clinic.EnrichResult{}, f.err
	}
	result := f.result
	if f.passthrough && runtimeContextEmpty(result.RuntimeContext) {
		result.RuntimeContext = runtime
	}
	return result, nil
}

type fakeWorkspaceService struct {
	ensureWS    *workspace.Workspace
	ensureErr   error
	statusWS    *workspace.Workspace
	statusErr   error
	switchWS    *workspace.Workspace
	switchErr   error
	syncWS      *workspace.Workspace
	syncErr     error
	resetWS     *workspace.Workspace
	resetErr    error
	lastAction  string
	lastUserKey string
	lastRepo    string
	lastRef     string
}

func (f *fakeWorkspaceService) Ensure(ctx context.Context, userKey string) (*workspace.Workspace, error) {
	f.lastAction = "ensure"
	f.lastUserKey = userKey
	return f.ensureWS, f.ensureErr
}

func (f *fakeWorkspaceService) Status(ctx context.Context, userKey string) (*workspace.Workspace, error) {
	f.lastAction = "status"
	f.lastUserKey = userKey
	return f.statusWS, f.statusErr
}

func (f *fakeWorkspaceService) SwitchRepo(ctx context.Context, userKey, repoName, ref string) (*workspace.Workspace, error) {
	f.lastAction = "switch"
	f.lastUserKey = userKey
	f.lastRepo = repoName
	f.lastRef = ref
	return f.switchWS, f.switchErr
}

func (f *fakeWorkspaceService) Sync(ctx context.Context, userKey, repoName string) (*workspace.Workspace, error) {
	f.lastAction = "sync"
	f.lastUserKey = userKey
	f.lastRepo = repoName
	return f.syncWS, f.syncErr
}

func (f *fakeWorkspaceService) Reset(ctx context.Context, userKey, repoName string) (*workspace.Workspace, error) {
	f.lastAction = "reset"
	f.lastUserKey = userKey
	f.lastRepo = repoName
	return f.resetWS, f.resetErr
}

func TestHandlePreparedReplyPrefixesAnswerForStandardQuestion(t *testing.T) {
	ws := newWorkspaceFixture()
	responder := &fakeResponder{answer: "final answer"}
	prefetcher := &fakePrefetcher{passthrough: true}
	workspaceSvc := &fakeWorkspaceService{ensureWS: ws}

	prepared := &preparedReply{
		question:        "select * from t",
		prefix:          "Downloaded 1 item(s).",
		attachmentCtx:   codex.AttachmentContext{RootDir: "/tmp/original-files"},
		threadCtx:       &codex.ThreadContext{ThreadID: "omt-thread"},
		conversationKey: "conv-1",
		userKey:         "ou_user",
	}

	got, err := handlePreparedReply(context.Background(), responder, prefetcher, workspaceSvc, prepared)
	if err != nil {
		t.Fatalf("handlePreparedReply error: %v", err)
	}
	if got != "Downloaded 1 item(s).\n\nfinal answer" {
		t.Fatalf("result = %q", got)
	}
	if workspaceSvc.lastAction != "ensure" {
		t.Fatalf("workspace action = %q, want ensure", workspaceSvc.lastAction)
	}
	if prefetcher.lastQuestion != "select * from t" {
		t.Fatalf("prefetch question = %q", prefetcher.lastQuestion)
	}
	if prefetcher.lastRuntime.Attachment.RootDir != ws.UserFilesDir {
		t.Fatalf("prefetch attachment root = %q, want %q", prefetcher.lastRuntime.Attachment.RootDir, ws.UserFilesDir)
	}
	if responder.lastConversation != "conv-1" {
		t.Fatalf("conversation = %q, want conv-1", responder.lastConversation)
	}
	if responder.lastRuntime.Workspace == nil || responder.lastRuntime.Workspace.RootDir != ws.RootDir {
		t.Fatalf("responder workspace root = %+v", responder.lastRuntime.Workspace)
	}
	if responder.lastRuntime.Thread == nil || responder.lastRuntime.Thread.ThreadID != "omt-thread" {
		t.Fatalf("responder thread context = %+v", responder.lastRuntime.Thread)
	}
}

func TestHandlePreparedReplyUsesIntroReplyWithoutCallingResponder(t *testing.T) {
	workspaceSvc := &fakeWorkspaceService{ensureWS: newWorkspaceFixture()}
	responder := &fakeResponder{answer: "should not be used"}
	prefetcher := &fakePrefetcher{
		passthrough: true,
		result: clinic.EnrichResult{
			IntroReply: "intro reply",
		},
	}

	prepared := &preparedReply{
		question:        "help",
		prefix:          "Downloaded 1 item(s).",
		conversationKey: "conv-2",
		userKey:         "ou_user",
	}

	got, err := handlePreparedReply(context.Background(), responder, prefetcher, workspaceSvc, prepared)
	if err != nil {
		t.Fatalf("handlePreparedReply error: %v", err)
	}
	if got != "Downloaded 1 item(s).\n\nintro reply" {
		t.Fatalf("result = %q", got)
	}
	if responder.calls != 0 {
		t.Fatalf("responder calls = %d, want 0", responder.calls)
	}
}

func TestHandlePreparedReplyPassesThreadContextLoaderToResponder(t *testing.T) {
	ws := newWorkspaceFixture()
	responder := &fakeResponder{answer: "final answer"}
	prefetcher := &fakePrefetcher{passthrough: true}
	workspaceSvc := &fakeWorkspaceService{ensureWS: ws}
	loads := 0

	prepared := &preparedReply{
		question:        "select * from t",
		conversationKey: "conv-thread-new",
		userKey:         "ou_user",
		threadCtxLoader: func(ctx context.Context) (*codex.ThreadContext, error) {
			loads++
			return &codex.ThreadContext{ThreadID: "omt-thread"}, nil
		},
	}

	got, err := handlePreparedReply(context.Background(), responder, prefetcher, workspaceSvc, prepared)
	if err != nil {
		t.Fatalf("handlePreparedReply error: %v", err)
	}
	if got != "final answer" {
		t.Fatalf("result = %q", got)
	}
	if loads != 0 {
		t.Fatalf("thread context loads = %d, want 0", loads)
	}
	if responder.lastRuntime.Thread != nil {
		t.Fatalf("responder thread context = %+v, want nil", responder.lastRuntime.Thread)
	}
	if responder.lastRuntime.ThreadLoader == nil {
		t.Fatalf("responder thread loader = nil, want non-nil")
	}
	threadCtx, err := responder.lastRuntime.ThreadLoader(context.Background())
	if err != nil {
		t.Fatalf("runtime thread loader returned error: %v", err)
	}
	if loads != 1 {
		t.Fatalf("thread context loads = %d, want 1 after explicit load", loads)
	}
	if threadCtx == nil || threadCtx.ThreadID != "omt-thread" {
		t.Fatalf("loaded thread context = %+v", threadCtx)
	}
}

func TestHandlePreparedReplyReturnsUserFacingClinicErrorWithUploadPrefix(t *testing.T) {
	workspaceSvc := &fakeWorkspaceService{ensureWS: newWorkspaceFixture()}
	responder := &fakeResponder{answer: "should not be used"}
	prefetcher := &fakePrefetcher{
		err: usererr.New(usererr.KindUnavailable, "clinic failed"),
	}

	prepared := &preparedReply{
		question:        "help",
		prefix:          "Downloaded 1 item(s).",
		conversationKey: "conv-3",
		userKey:         "ou_user",
	}

	got, err := handlePreparedReply(context.Background(), responder, prefetcher, workspaceSvc, prepared)
	if err != nil {
		t.Fatalf("handlePreparedReply error: %v", err)
	}
	if got != "Downloaded 1 item(s).\n\nclinic failed" {
		t.Fatalf("result = %q", got)
	}
	if responder.calls != 0 {
		t.Fatalf("responder calls = %d, want 0", responder.calls)
	}
}

func TestHandlePreparedReplyRunsWorkspaceStatusQuestion(t *testing.T) {
	ws := newWorkspaceFixture()
	responder := &fakeResponder{answer: "workspace answer"}
	prefetcher := &fakePrefetcher{passthrough: true}
	workspaceSvc := &fakeWorkspaceService{statusWS: ws}

	prepared := &preparedReply{
		question:        "why is this slow",
		workspaceCmd:    &workspace.Command{Action: "status"},
		conversationKey: "conv-4",
		userKey:         "ou_user",
	}

	got, err := handlePreparedReply(context.Background(), responder, prefetcher, workspaceSvc, prepared)
	if err != nil {
		t.Fatalf("handlePreparedReply error: %v", err)
	}
	want := workspace.FormatStatus(ws) + "\n\nworkspace answer"
	if got != want {
		t.Fatalf("result mismatch:\n got: %q\nwant: %q", got, want)
	}
	if workspaceSvc.lastAction != "status" {
		t.Fatalf("workspace action = %q, want status", workspaceSvc.lastAction)
	}
}

func TestHandlePreparedReplyRunsWorkspaceSwitchQuestionWithUserFacingError(t *testing.T) {
	ws := newWorkspaceFixture()
	responder := &fakeResponder{answer: "should not be used"}
	prefetcher := &fakePrefetcher{
		err: usererr.New(usererr.KindUnavailable, "clinic failed"),
	}
	workspaceSvc := &fakeWorkspaceService{switchWS: ws}

	prepared := &preparedReply{
		question:        "inspect this branch",
		workspaceCmd:    &workspace.Command{Action: "switch", Repo: "tidb", Ref: "release-8.5"},
		conversationKey: "conv-5",
		userKey:         "ou_user",
	}

	got, err := handlePreparedReply(context.Background(), responder, prefetcher, workspaceSvc, prepared)
	if err != nil {
		t.Fatalf("handlePreparedReply error: %v", err)
	}
	want := workspace.FormatStatus(ws) + "\n\nclinic failed"
	if got != want {
		t.Fatalf("result mismatch:\n got: %q\nwant: %q", got, want)
	}
	if workspaceSvc.lastAction != "switch" {
		t.Fatalf("workspace action = %q, want switch", workspaceSvc.lastAction)
	}
	if workspaceSvc.lastRepo != "tidb" || workspaceSvc.lastRef != "release-8.5" {
		t.Fatalf("switch args = repo:%q ref:%q", workspaceSvc.lastRepo, workspaceSvc.lastRef)
	}
	if responder.calls != 0 {
		t.Fatalf("responder calls = %d, want 0", responder.calls)
	}
}

func TestHandlePreparedReplyReturnsModelStatus(t *testing.T) {
	responder := &fakeResponder{
		modelState: codex.ModelState{
			DefaultModel:           "gpt-5.3-codex",
			EffectiveModel:         "gpt-5.4",
			OverrideModel:          "gpt-5.4",
			DefaultReasoningEffort: "medium",
			ReasoningEffort:        "medium",
			ReasoningOptions:       []codex.ReasoningEffortOption{{Effort: "low"}, {Effort: "medium"}, {Effort: "high"}},
		},
	}
	workspaceSvc := &fakeWorkspaceService{}

	prepared := &preparedReply{
		modelCmd:        &modelcmd.Command{Action: "status"},
		conversationKey: "conv-model",
		userKey:         "ou_user",
	}

	got, err := handlePreparedReply(context.Background(), responder, &fakePrefetcher{}, workspaceSvc, prepared)
	if err != nil {
		t.Fatalf("handlePreparedReply error: %v", err)
	}
	if responder.lastModelAction != "status" {
		t.Fatalf("model action = %q, want status", responder.lastModelAction)
	}
	if workspaceSvc.lastAction != "" {
		t.Fatalf("workspace action = %q, want none", workspaceSvc.lastAction)
	}
	if got == "" || !containsAll(got, "Model: gpt-5.4", "Default Model: gpt-5.3-codex", "Reasoning Options: low, medium, high") {
		t.Fatalf("unexpected result: %q", got)
	}
}

func TestHandlePreparedReplySetsModelThenAnswersQuestion(t *testing.T) {
	ws := newWorkspaceFixture()
	responder := &fakeResponder{
		answer: "model answer",
		setModelResult: codex.ModelChangeResult{
			State: codex.ModelState{
				DefaultModel:           "gpt-5.3-codex",
				EffectiveModel:         "gpt-5.4",
				OverrideModel:          "gpt-5.4",
				DefaultReasoningEffort: "medium",
				ReasoningEffort:        "medium",
				ReasoningOptions:       []codex.ReasoningEffortOption{{Effort: "low"}, {Effort: "medium"}, {Effort: "high"}},
			},
			Changed: true,
		},
	}
	prefetcher := &fakePrefetcher{passthrough: true}
	workspaceSvc := &fakeWorkspaceService{ensureWS: ws}

	prepared := &preparedReply{
		question:        "analyze this query",
		modelCmd:        &modelcmd.Command{Action: "set", Model: "gpt-5.4"},
		conversationKey: "conv-model-set",
		userKey:         "ou_user",
	}

	got, err := handlePreparedReply(context.Background(), responder, prefetcher, workspaceSvc, prepared)
	if err != nil {
		t.Fatalf("handlePreparedReply error: %v", err)
	}
	if responder.lastModelAction != "set" || responder.lastModel != "gpt-5.4" {
		t.Fatalf("unexpected model call action=%q model=%q", responder.lastModelAction, responder.lastModel)
	}
	if workspaceSvc.lastAction != "ensure" {
		t.Fatalf("workspace action = %q, want ensure", workspaceSvc.lastAction)
	}
	if !containsAll(got, "Model: gpt-5.4", "Default Model: gpt-5.3-codex", "model answer") {
		t.Fatalf("unexpected result: %q", got)
	}
	if strings.Contains(got, "the next question will start a new Codex session") {
		t.Fatalf("unexpected session restart message: %q", got)
	}
}

func TestHandlePreparedReplySetsReasoningEffortThenAnswersQuestion(t *testing.T) {
	ws := newWorkspaceFixture()
	responder := &fakeResponder{
		answer: "effort answer",
		setEffortResult: codex.ModelChangeResult{
			State: codex.ModelState{
				DefaultModel:            "gpt-5.3-codex",
				EffectiveModel:          "gpt-5.3-codex",
				DefaultReasoningEffort:  "medium",
				OverrideReasoningEffort: "high",
				ReasoningEffort:         "high",
				ReasoningOptions:        []codex.ReasoningEffortOption{{Effort: "low"}, {Effort: "medium"}, {Effort: "high"}},
			},
			Changed: true,
		},
	}
	prefetcher := &fakePrefetcher{passthrough: true}
	workspaceSvc := &fakeWorkspaceService{ensureWS: ws}

	prepared := &preparedReply{
		question:        "analyze this query",
		modelCmd:        &modelcmd.Command{Action: "effort", Effort: "high"},
		conversationKey: "conv-effort-set",
		userKey:         "ou_user",
	}

	got, err := handlePreparedReply(context.Background(), responder, prefetcher, workspaceSvc, prepared)
	if err != nil {
		t.Fatalf("handlePreparedReply error: %v", err)
	}
	if responder.lastModelAction != "effort" || responder.lastEffort != "high" {
		t.Fatalf("unexpected effort call action=%q effort=%q", responder.lastModelAction, responder.lastEffort)
	}
	if !containsAll(got, "Reasoning: high", "Default Reasoning: medium", "effort answer") {
		t.Fatalf("unexpected result: %q", got)
	}
}

func newWorkspaceFixture() *workspace.Workspace {
	return &workspace.Workspace{
		UserKey:         "ou_user",
		RootDir:         "/tmp/ws/root",
		UserFilesDir:    "/tmp/ws/root/user-files",
		ClinicFilesDir:  "/tmp/ws/root/clinic-files",
		EnvironmentHash: "envhash",
		Repos: []workspace.RepoState{{
			Name:         "tidb",
			RelativePath: "contrib/tidb",
			RequestedRef: "master",
			ResolvedSHA:  "1234567890abcdef1234567890abcdef12345678",
		}},
	}
}

func runtimeContextEmpty(runtime codex.RuntimeContext) bool {
	return runtime.Attachment.RootDir == "" &&
		len(runtime.Attachment.Items) == 0 &&
		runtime.ClinicLibrary == nil &&
		runtime.Clinic == nil &&
		runtime.Thread == nil &&
		runtime.ThreadLoader == nil &&
		runtime.Workspace == nil
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
