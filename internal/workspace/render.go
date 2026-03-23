package workspace

import (
	"fmt"
	"strings"

	"lab/askplanner/internal/codex"
)

func (w *Workspace) ToCodexContext() *codex.WorkspaceContext {
	if w == nil {
		return nil
	}
	repos := make([]codex.WorkspaceRepoContext, 0, len(w.Repos))
	for _, repo := range w.Repos {
		repos = append(repos, codex.WorkspaceRepoContext{
			Name:           repo.Name,
			RelativePath:   repo.RelativePath,
			RequestedRef:   repo.RequestedRef,
			ResolvedSHA:    repo.ResolvedSHA,
			TrackingLatest: repo.TrackingLatest,
		})
	}
	return &codex.WorkspaceContext{
		RootDir:         w.RootDir,
		UserFilesDir:    w.UserFilesDir,
		ClinicFilesDir:  w.ClinicFilesDir,
		EnvironmentHash: w.EnvironmentHash,
		Repos:           repos,
	}
}

func BindRuntimeContext(runtime codex.RuntimeContext, w *Workspace) codex.RuntimeContext {
	runtime.Workspace = w.ToCodexContext()
	if w == nil {
		return runtime
	}
	if strings.TrimSpace(runtime.Attachment.RootDir) != "" {
		runtime.Attachment.RootDir = w.UserFilesDir
	}
	if runtime.ClinicLibrary != nil && strings.TrimSpace(runtime.ClinicLibrary.RootDir) != "" {
		runtime.ClinicLibrary.RootDir = w.ClinicFilesDir
	}
	return runtime
}

func FormatStatus(w *Workspace) string {
	if w == nil {
		return "Workspace: unavailable"
	}
	var sb strings.Builder
	sb.WriteString("Workspace ready.\n")
	sb.WriteString("- Root: ")
	sb.WriteString(w.RootDir)
	sb.WriteByte('\n')
	sb.WriteString("- User Files: ")
	sb.WriteString(w.UserFilesDir)
	sb.WriteByte('\n')
	sb.WriteString("- Clinic Files: ")
	sb.WriteString(w.ClinicFilesDir)
	sb.WriteByte('\n')
	sb.WriteString("- Environment Hash: ")
	sb.WriteString(w.EnvironmentHash)
	sb.WriteByte('\n')
	sb.WriteString("- Repos:\n")
	for _, repo := range w.Repos {
		sb.WriteString(fmt.Sprintf("  - %s ref=%s sha=%s", repo.RelativePath, repo.RequestedRef, shortSHA(repo.ResolvedSHA)))
		if repo.TrackingLatest {
			sb.WriteString(" tracking=latest")
		}
		sb.WriteByte('\n')
	}
	return strings.TrimSpace(sb.String())
}

func shortSHA(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
