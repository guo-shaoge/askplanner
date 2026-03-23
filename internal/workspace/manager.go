package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"lab/askplanner/internal/config"
)

const metadataFileName = "workspace.json"

type RepoSpec struct {
	Name          string
	RelativePath  string
	RemoteURL     string
	DefaultRef    string
	TrackLatest   bool
	FollowTidbRef bool
}

type RepoState struct {
	Name           string    `json:"name"`
	RelativePath   string    `json:"relative_path"`
	RequestedRef   string    `json:"requested_ref"`
	ResolvedSHA    string    `json:"resolved_sha"`
	TrackingLatest bool      `json:"tracking_latest,omitempty"`
	LastSyncedAt   time.Time `json:"last_synced_at,omitempty"`
}

type Workspace struct {
	UserKey         string
	RootDir         string
	UserFilesDir    string
	ClinicFilesDir  string
	EnvironmentHash string
	Repos           []RepoState
}

type workspaceMetadata struct {
	UserKey         string                `json:"user_key"`
	LastActiveAt    time.Time             `json:"last_active_at"`
	EnvironmentHash string                `json:"environment_hash"`
	Repos           map[string]*RepoState `json:"repos"`
}

type Manager struct {
	rootDir                string
	usersDir               string
	mirrorsDir             string
	locksDir               string
	trashDir               string
	uploadRoot             string
	clinicRoot             string
	idleTTL                time.Duration
	gcInterval             time.Duration
	agentRulesSyncInterval time.Duration
	repos                  map[string]RepoSpec
	repoOrder              []string

	mu                 sync.Mutex
	lastGCAttempt      time.Time
	lastAgentRulesSync time.Time
}

func NewManager(cfg *config.Config) (*Manager, error) {
	rootDir := strings.TrimSpace(cfg.WorkspaceRoot)
	if rootDir == "" {
		return nil, fmt.Errorf("workspace root is empty")
	}

	m := &Manager{
		rootDir:                rootDir,
		usersDir:               filepath.Join(rootDir, "users"),
		mirrorsDir:             filepath.Join(rootDir, "mirrors"),
		locksDir:               filepath.Join(rootDir, "locks"),
		trashDir:               filepath.Join(rootDir, ".trash"),
		uploadRoot:             cfg.FeishuFileDir,
		clinicRoot:             cfg.ClinicStoreDir,
		idleTTL:                time.Duration(cfg.WorkspaceIdleTTLHours) * time.Hour,
		gcInterval:             time.Duration(cfg.WorkspaceGCIntervalMin) * time.Minute,
		agentRulesSyncInterval: time.Duration(cfg.AgentRulesSyncIntervalMin) * time.Minute,
		repos: map[string]RepoSpec{
			"tidb": {
				Name:         "tidb",
				RelativePath: filepath.Join("contrib", "tidb"),
				RemoteURL:    strings.TrimSpace(cfg.WorkspaceRepoTidbURL),
				DefaultRef:   strings.TrimSpace(cfg.WorkspaceRepoTidbDefaultRef),
			},
			"agent-rules": {
				Name:         "agent-rules",
				RelativePath: filepath.Join("contrib", "agent-rules"),
				RemoteURL:    strings.TrimSpace(cfg.WorkspaceRepoAgentRulesURL),
				DefaultRef:   strings.TrimSpace(cfg.WorkspaceRepoAgentRulesDefaultRef),
				TrackLatest:  true,
			},
			"tidb-docs": {
				Name:          "tidb-docs",
				RelativePath:  filepath.Join("contrib", "tidb-docs"),
				RemoteURL:     strings.TrimSpace(cfg.WorkspaceRepoTidbDocsURL),
				DefaultRef:    strings.TrimSpace(cfg.WorkspaceRepoTidbDocsDefaultRef),
				FollowTidbRef: true,
			},
		},
		repoOrder: []string{"tidb", "agent-rules", "tidb-docs"},
	}

	for _, dir := range []string{m.rootDir, m.usersDir, m.mirrorsDir, m.locksDir, m.trashDir, m.uploadRoot, m.clinicRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create workspace dir %s: %w", dir, err)
		}
	}
	for _, spec := range m.repos {
		if spec.RemoteURL == "" {
			return nil, fmt.Errorf("workspace repo %s remote URL is empty", spec.Name)
		}
		if spec.DefaultRef == "" {
			return nil, fmt.Errorf("workspace repo %s default ref is empty", spec.Name)
		}
	}

	return m, nil
}

func (m *Manager) Ensure(ctx context.Context, userKey string) (*Workspace, error) {
	if err := m.syncAgentRulesMirrorIfDue(ctx); err != nil {
		log.Printf("[workspace] scheduled agent-rules sync failed: %v", err)
	}
	lock, userKey, err := m.lockUser(userKey, false)
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	return m.ensureLocked(ctx, userKey)
}

func (m *Manager) Status(ctx context.Context, userKey string) (*Workspace, error) {
	return m.Ensure(ctx, userKey)
}

func (m *Manager) SwitchRepo(ctx context.Context, userKey, repoName, ref string) (*Workspace, error) {
	spec, ok := m.repos[repoName]
	if !ok {
		return nil, fmt.Errorf("unsupported workspace repo: %s", repoName)
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("workspace ref is empty")
	}
	lock, userKey, err := m.lockUser(userKey, false)
	if err != nil {
		return nil, err
	}
	defer lock.Close()

	meta, dirs, err := m.ensureMetadata(userKey)
	if err != nil {
		return nil, err
	}
	if err := m.fetchMirror(ctx, spec); err != nil {
		return nil, err
	}
	sha, err := m.resolveRef(ctx, spec, ref)
	if err != nil {
		return nil, err
	}
	state := meta.reposForUpdate(spec)
	state.RequestedRef = ref
	state.ResolvedSHA = sha
	state.TrackingLatest = spec.TrackLatest && ref == spec.DefaultRef
	state.LastSyncedAt = time.Now().UTC()
	if err := m.ensureCheckout(ctx, spec, dirs.rootDir, sha); err != nil {
		return nil, err
	}

	if repoName == "tidb" {
		docsSpec := m.repos["tidb-docs"]
		docsState := meta.reposForUpdate(docsSpec)
		docsRef := docsSpec.DefaultRef
		if err := m.fetchMirror(ctx, docsSpec); err != nil {
			return nil, err
		}
		if !isLikelyCommitSHA(ref) {
			if _, err := m.resolveRef(ctx, docsSpec, ref); err == nil {
				docsRef = ref
			}
		}
		docsSHA, err := m.resolveRef(ctx, docsSpec, docsRef)
		if err != nil {
			return nil, err
		}
		docsState.RequestedRef = docsRef
		docsState.ResolvedSHA = docsSHA
		docsState.TrackingLatest = false
		docsState.LastSyncedAt = time.Now().UTC()
		if err := m.ensureCheckout(ctx, docsSpec, dirs.rootDir, docsSHA); err != nil {
			return nil, err
		}
	}

	return m.finalizeLocked(meta, dirs)
}

func (m *Manager) Sync(ctx context.Context, userKey, repoName string) (*Workspace, error) {
	lock, userKey, err := m.lockUser(userKey, false)
	if err != nil {
		return nil, err
	}
	defer lock.Close()

	meta, dirs, err := m.ensureMetadata(userKey)
	if err != nil {
		return nil, err
	}

	targets := m.syncTargets(repoName)
	if len(targets) == 0 {
		return nil, fmt.Errorf("unsupported workspace repo: %s", repoName)
	}

	now := time.Now().UTC()
	for _, spec := range targets {
		if err := m.fetchMirror(ctx, spec); err != nil {
			return nil, err
		}
		state := meta.reposForUpdate(spec)
		requestedRef := strings.TrimSpace(state.RequestedRef)
		if requestedRef == "" {
			requestedRef = spec.DefaultRef
		}
		sha, err := m.resolveRef(ctx, spec, requestedRef)
		if err != nil {
			return nil, err
		}
		state.RequestedRef = requestedRef
		state.ResolvedSHA = sha
		state.LastSyncedAt = now
		if spec.TrackLatest && requestedRef == spec.DefaultRef {
			state.TrackingLatest = true
		}
		if err := m.ensureCheckout(ctx, spec, dirs.rootDir, sha); err != nil {
			return nil, err
		}
	}

	return m.finalizeLocked(meta, dirs)
}

func (m *Manager) Reset(ctx context.Context, userKey, repoName string) (*Workspace, error) {
	lock, userKey, err := m.lockUser(userKey, false)
	if err != nil {
		return nil, err
	}
	defer lock.Close()

	meta, dirs, err := m.ensureMetadata(userKey)
	if err != nil {
		return nil, err
	}

	targets := m.syncTargets(repoName)
	if len(targets) == 0 {
		return nil, fmt.Errorf("unsupported workspace repo: %s", repoName)
	}

	now := time.Now().UTC()
	for _, spec := range targets {
		if err := m.fetchMirror(ctx, spec); err != nil {
			return nil, err
		}
		state := meta.reposForUpdate(spec)
		sha, err := m.resolveRef(ctx, spec, spec.DefaultRef)
		if err != nil {
			return nil, err
		}
		state.RequestedRef = spec.DefaultRef
		state.ResolvedSHA = sha
		state.TrackingLatest = spec.TrackLatest
		state.LastSyncedAt = now
		if err := m.ensureCheckout(ctx, spec, dirs.rootDir, sha); err != nil {
			return nil, err
		}
	}

	return m.finalizeLocked(meta, dirs)
}

func (m *Manager) MaybeSweep(ctx context.Context) error {
	if m.gcInterval <= 0 {
		return nil
	}
	m.mu.Lock()
	if time.Since(m.lastGCAttempt) < m.gcInterval {
		m.mu.Unlock()
		return nil
	}
	m.lastGCAttempt = time.Now()
	m.mu.Unlock()
	return m.Sweep(ctx)
}

func (m *Manager) Sweep(ctx context.Context) error {
	lockPath := filepath.Join(m.locksDir, "gc.lock")
	lock, err := acquireFileLock(lockPath, true)
	if err != nil {
		if errors.Is(err, errLockBusy) {
			return nil
		}
		return err
	}
	defer lock.Close()

	entries, err := os.ReadDir(m.usersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read workspace users dir: %w", err)
	}

	now := time.Now().UTC()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		userKey := entry.Name()
		lock, _, err := m.lockUser(userKey, true)
		if err != nil {
			if errors.Is(err, errLockBusy) {
				continue
			}
			return err
		}
		if lock == nil {
			continue
		}

		metaPath := filepath.Join(m.usersDir, userKey, "data", metadataFileName)
		meta, err := loadMetadataFile(metaPath)
		if err != nil && !os.IsNotExist(err) {
			lock.Close()
			return err
		}
		lastActive := time.Time{}
		if meta != nil {
			lastActive = meta.LastActiveAt
		}
		if m.idleTTL > 0 && !lastActive.IsZero() && now.Sub(lastActive) <= m.idleTTL {
			lock.Close()
			continue
		}
		if err := m.removeWorkspaceLocked(ctx, userKey); err != nil {
			lock.Close()
			return err
		}
		lock.Close()
	}
	return nil
}

func (m *Manager) StartBackgroundJobs(ctx context.Context) {
	if m.agentRulesSyncInterval > 0 {
		go func() {
			ticker := time.NewTicker(m.agentRulesSyncInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := m.syncAgentRulesMirror(ctx); err != nil {
						log.Printf("[workspace] background agent-rules sync failed: %v", err)
					}
				}
			}
		}()
	}
	if m.gcInterval > 0 {
		go func() {
			ticker := time.NewTicker(m.gcInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := m.Sweep(ctx); err != nil {
						log.Printf("[workspace] background GC failed: %v", err)
					}
				}
			}
		}()
	}
}

func (m *Manager) syncTargets(repoName string) []RepoSpec {
	repoName = strings.TrimSpace(repoName)
	if repoName == "" || repoName == "all" {
		out := make([]RepoSpec, 0, len(m.repoOrder))
		for _, name := range m.repoOrder {
			out = append(out, m.repos[name])
		}
		return out
	}
	spec, ok := m.repos[repoName]
	if !ok {
		return nil
	}
	return []RepoSpec{spec}
}

func (m *Manager) ensureLocked(ctx context.Context, userKey string) (*Workspace, error) {
	meta, dirs, err := m.ensureMetadata(userKey)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for _, name := range m.repoOrder {
		spec := m.repos[name]
		state := meta.reposForUpdate(spec)
		if state.RequestedRef == "" {
			state.RequestedRef = spec.DefaultRef
			state.TrackingLatest = spec.TrackLatest
		}
		if spec.TrackLatest && state.TrackingLatest {
			state.RequestedRef = spec.DefaultRef
		}
		if err := m.ensureMirror(ctx, spec); err != nil {
			return nil, err
		}
		sha, err := m.resolveRef(ctx, spec, state.RequestedRef)
		if err != nil {
			return nil, err
		}
		if state.ResolvedSHA != sha || !isGitWorktree(m.checkoutPath(dirs.rootDir, spec)) {
			if err := m.ensureCheckout(ctx, spec, dirs.rootDir, sha); err != nil {
				return nil, err
			}
			state.ResolvedSHA = sha
			state.LastSyncedAt = now
		}
	}
	return m.finalizeLocked(meta, dirs)
}

func (m *Manager) finalizeLocked(meta *workspaceMetadata, dirs workspaceDirs) (*Workspace, error) {
	meta.LastActiveAt = time.Now().UTC()
	meta.EnvironmentHash = computeEnvironmentHash(dirs.rootDir, meta.Repos)
	if err := saveMetadataFile(dirs.metaPath, meta); err != nil {
		return nil, err
	}

	repos := make([]RepoState, 0, len(meta.Repos))
	for _, name := range m.repoOrder {
		state, ok := meta.Repos[name]
		if !ok || state == nil {
			continue
		}
		repos = append(repos, *state)
	}
	return &Workspace{
		UserKey:         meta.UserKey,
		RootDir:         dirs.rootDir,
		UserFilesDir:    filepath.Join(dirs.rootDir, "user-files"),
		ClinicFilesDir:  filepath.Join(dirs.rootDir, "clinic-files"),
		EnvironmentHash: meta.EnvironmentHash,
		Repos:           repos,
	}, nil
}

type workspaceDirs struct {
	userDir    string
	rootDir    string
	dataDir    string
	metaPath   string
	uploadsDir string
	clinicDir  string
}

func (m *Manager) ensureMetadata(userKey string) (*workspaceMetadata, workspaceDirs, error) {
	userKey = sanitizePathSegment(userKey, "")
	if userKey == "" {
		return nil, workspaceDirs{}, fmt.Errorf("workspace user key is empty")
	}
	dirs := workspaceDirs{
		userDir:    filepath.Join(m.usersDir, userKey),
		rootDir:    filepath.Join(m.usersDir, userKey, "root"),
		dataDir:    filepath.Join(m.usersDir, userKey, "data"),
		metaPath:   filepath.Join(m.usersDir, userKey, "data", metadataFileName),
		uploadsDir: filepath.Join(m.uploadRoot, userKey),
		clinicDir:  filepath.Join(m.clinicRoot, userKey),
	}
	for _, dir := range []string{dirs.rootDir, dirs.dataDir, filepath.Join(dirs.rootDir, "contrib"), dirs.uploadsDir, dirs.clinicDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, workspaceDirs{}, fmt.Errorf("create workspace dir %s: %w", dir, err)
		}
	}
	if err := ensureSymlink(dirs.uploadsDir, filepath.Join(dirs.rootDir, "user-files")); err != nil {
		return nil, workspaceDirs{}, err
	}
	if err := ensureSymlink(dirs.clinicDir, filepath.Join(dirs.rootDir, "clinic-files")); err != nil {
		return nil, workspaceDirs{}, err
	}

	meta, err := loadMetadataFile(dirs.metaPath)
	switch {
	case err == nil:
	case os.IsNotExist(err):
		meta = &workspaceMetadata{
			UserKey: userKey,
			Repos:   make(map[string]*RepoState),
		}
	default:
		return nil, workspaceDirs{}, err
	}
	if meta.Repos == nil {
		meta.Repos = make(map[string]*RepoState)
	}
	meta.UserKey = userKey
	return meta, dirs, nil
}

func (m *Manager) mirrorPath(spec RepoSpec) string {
	return filepath.Join(m.mirrorsDir, spec.Name+".git")
}

func (m *Manager) checkoutPath(rootDir string, spec RepoSpec) string {
	return filepath.Join(rootDir, spec.RelativePath)
}

func (m *Manager) ensureMirror(ctx context.Context, spec RepoSpec) error {
	mirrorPath := m.mirrorPath(spec)
	if _, err := os.Stat(filepath.Join(mirrorPath, "HEAD")); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(mirrorPath), 0o755); err != nil {
		return fmt.Errorf("create mirror parent dir: %w", err)
	}
	if _, err := runGit(ctx, "", "clone", "--mirror", spec.RemoteURL, mirrorPath); err != nil {
		return fmt.Errorf("clone mirror for %s: %w", spec.Name, err)
	}
	return nil
}

func (m *Manager) fetchMirror(ctx context.Context, spec RepoSpec) error {
	if err := m.ensureMirror(ctx, spec); err != nil {
		return err
	}
	_, err := runGit(ctx, "", "--git-dir", m.mirrorPath(spec), "fetch", "--prune", "--tags", "origin", "+refs/heads/*:refs/heads/*", "+refs/tags/*:refs/tags/*")
	if err != nil {
		return fmt.Errorf("fetch mirror for %s: %w", spec.Name, err)
	}
	return nil
}

func (m *Manager) resolveRef(ctx context.Context, spec RepoSpec, ref string) (string, error) {
	if err := m.ensureMirror(ctx, spec); err != nil {
		return "", err
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		ref = spec.DefaultRef
	}
	candidates := []string{
		ref + "^{commit}",
		"refs/heads/" + ref + "^{commit}",
		"refs/tags/" + ref + "^{commit}",
		"refs/remotes/origin/" + ref + "^{commit}",
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out, err := runGit(ctx, "", "--git-dir", m.mirrorPath(spec), "rev-parse", "--verify", candidate)
		if err == nil {
			return strings.TrimSpace(out), nil
		}
	}
	return "", fmt.Errorf("resolve ref %q for %s", ref, spec.Name)
}

func (m *Manager) ensureCheckout(ctx context.Context, spec RepoSpec, rootDir, sha string) error {
	checkoutPath := m.checkoutPath(rootDir, spec)
	if err := os.MkdirAll(filepath.Dir(checkoutPath), 0o755); err != nil {
		return fmt.Errorf("create checkout parent dir: %w", err)
	}
	if isGitWorktree(checkoutPath) {
		_, err := runGit(ctx, checkoutPath, "checkout", "--detach", "--force", sha)
		if err != nil {
			return fmt.Errorf("checkout %s at %s: %w", spec.Name, sha, err)
		}
		return nil
	}

	if err := os.RemoveAll(checkoutPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale checkout %s: %w", checkoutPath, err)
	}
	_, _ = runGit(ctx, "", "--git-dir", m.mirrorPath(spec), "worktree", "prune")
	_, err := runGit(ctx, "", "--git-dir", m.mirrorPath(spec), "worktree", "add", "--force", "--detach", checkoutPath, sha)
	if err != nil {
		return fmt.Errorf("create worktree for %s: %w", spec.Name, err)
	}
	return nil
}

func (m *Manager) removeWorkspaceLocked(ctx context.Context, userKey string) error {
	rootDir := filepath.Join(m.usersDir, userKey, "root")
	for _, name := range m.repoOrder {
		spec := m.repos[name]
		_, _ = runGit(ctx, "", "--git-dir", m.mirrorPath(spec), "worktree", "remove", "--force", m.checkoutPath(rootDir, spec))
		_, _ = runGit(ctx, "", "--git-dir", m.mirrorPath(spec), "worktree", "prune")
	}
	for _, path := range []string{
		filepath.Join(m.usersDir, userKey),
		filepath.Join(m.uploadRoot, userKey),
		filepath.Join(m.clinicRoot, userKey),
	} {
		if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove workspace path %s: %w", path, err)
		}
	}
	return nil
}

func (m *Manager) syncAgentRulesMirrorIfDue(ctx context.Context) error {
	if m.agentRulesSyncInterval <= 0 {
		return nil
	}
	m.mu.Lock()
	if time.Since(m.lastAgentRulesSync) < m.agentRulesSyncInterval {
		m.mu.Unlock()
		return nil
	}
	m.lastAgentRulesSync = time.Now()
	m.mu.Unlock()
	return m.syncAgentRulesMirror(ctx)
}

func (m *Manager) syncAgentRulesMirror(ctx context.Context) error {
	return m.fetchMirror(ctx, m.repos["agent-rules"])
}

type fileLock struct {
	file *os.File
}

func (l *fileLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

var errLockBusy = errors.New("lock busy")

func (m *Manager) lockUser(userKey string, nonBlocking bool) (*fileLock, string, error) {
	userKey = sanitizePathSegment(userKey, "")
	if userKey == "" {
		return nil, "", fmt.Errorf("workspace user key is empty")
	}
	lock, err := acquireFileLock(filepath.Join(m.locksDir, userKey+".lock"), nonBlocking)
	if err != nil {
		return nil, "", err
	}
	return lock, userKey, nil
}

func acquireFileLock(path string, nonBlocking bool) (*fileLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	mode := syscall.LOCK_EX
	if nonBlocking {
		mode |= syscall.LOCK_NB
	}
	if err := syscall.Flock(int(f.Fd()), mode); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errLockBusy
		}
		return nil, fmt.Errorf("acquire lock %s: %w", path, err)
	}
	return &fileLock{file: f}, nil
}

func ensureSymlink(target, linkPath string) error {
	current, err := os.Readlink(linkPath)
	if err == nil && current == target {
		return nil
	}
	if err == nil || !os.IsNotExist(err) {
		if removeErr := os.RemoveAll(linkPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("remove stale link %s: %w", linkPath, removeErr)
		}
	}
	if err := os.Symlink(target, linkPath); err != nil {
		return fmt.Errorf("create symlink %s -> %s: %w", linkPath, target, err)
	}
	return nil
}

func loadMetadataFile(path string) (*workspaceMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta workspaceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse workspace metadata: %w", err)
	}
	if meta.Repos == nil {
		meta.Repos = make(map[string]*RepoState)
	}
	return &meta, nil
}

func saveMetadataFile(path string, meta *workspaceMetadata) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create metadata dir: %w", err)
	}
	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".workspace-*.json")
	if err != nil {
		return fmt.Errorf("create metadata temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(meta); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("encode metadata: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close metadata temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace workspace metadata: %w", err)
	}
	return nil
}

func (m *workspaceMetadata) reposForUpdate(spec RepoSpec) *RepoState {
	if m.Repos == nil {
		m.Repos = make(map[string]*RepoState)
	}
	state, ok := m.Repos[spec.Name]
	if !ok || state == nil {
		state = &RepoState{
			Name:           spec.Name,
			RelativePath:   spec.RelativePath,
			RequestedRef:   spec.DefaultRef,
			TrackingLatest: spec.TrackLatest,
		}
		m.Repos[spec.Name] = state
	}
	state.Name = spec.Name
	state.RelativePath = spec.RelativePath
	return state
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func computeEnvironmentHash(rootDir string, repos map[string]*RepoState) string {
	keys := make([]string, 0, len(repos))
	for key := range repos {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(rootDir))
	sb.WriteByte('\n')
	for _, key := range keys {
		state := repos[key]
		if state == nil {
			continue
		}
		sb.WriteString(key)
		sb.WriteByte('|')
		sb.WriteString(strings.TrimSpace(state.RequestedRef))
		sb.WriteByte('|')
		sb.WriteString(strings.TrimSpace(state.ResolvedSHA))
		sb.WriteByte('|')
		if state.TrackingLatest {
			sb.WriteString("latest")
		}
		sb.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

func isGitWorktree(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	if err == nil {
		return !info.IsDir() || info.IsDir()
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
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

func isLikelyCommitSHA(ref string) bool {
	ref = strings.TrimSpace(ref)
	if len(ref) < 7 || len(ref) > 40 {
		return false
	}
	for _, r := range ref {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}
