package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"lab/askplanner/internal/admin"
	"lab/askplanner/internal/clinic"
	"lab/askplanner/internal/codex"
	"lab/askplanner/internal/config"
	"lab/askplanner/internal/modelcmd"
	"lab/askplanner/internal/selfcmd"
	"lab/askplanner/internal/usage"
	"lab/askplanner/internal/usererr"
	"lab/askplanner/internal/workspace"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatalStartup("load config", err, "Check PROJECT_ROOT and PROMPT_FILE, or start askplanner from the repository root.")
	}

	logFile, err := config.SetupLogging(cfg.LogFile)
	if err != nil {
		fatalStartup("setup logging", err, "Check LOG_FILE and make sure the target directory is writable.")
	}
	defer logFile.Close()
	if _, err := exec.LookPath(cfg.CodexBin); err != nil {
		fatalStartup("locate Codex CLI", err, fmt.Sprintf("Install Codex CLI or point CODEX_BIN to a valid executable. Current value: %s", cfg.CodexBin))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	responder, err := codex.NewResponder(cfg)
	if err != nil {
		fatalStartup("build codex responder", err, "Check PROMPT_FILE and CODEX_SESSION_STORE. The prompt must exist, and the session-store directory must be writable.")
	}
	prefetcher, err := clinic.NewPrefetcher(cfg)
	if err != nil {
		fatalStartup("build clinic prefetcher", err, "If Clinic auto-analysis is enabled, make sure CLINIC_STORE_DIR is writable.")
	}
	workspaceManager, err := workspace.NewManager(cfg)
	if err != nil {
		fatalStartup("build workspace manager", err, "Check workspace-related storage paths and make sure they are writable.")
	}
	tracker, err := usage.NewQuestionTracker(cfg)
	if err != nil {
		log.Printf("[askplanner] usage tracker disabled: %v", err)
	}

	fmt.Printf("askplanner v2 (backend: codex-cli, model: %s)\n", cfg.CodexModel)
	fmt.Println("Type your question, or 'quit' to exit. Use 'reset' to start a new session. Use '/model' to inspect or switch the model for this conversation.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	const conversationKey = "cli:default"
	const clinicUserKey = "cli_default"
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		question := strings.TrimSpace(scanner.Text())
		if question == "" {
			continue
		}
		if question == "quit" || question == "exit" {
			fmt.Println("Bye!")
			break
		}
		if err := workspaceManager.MaybeSweep(ctx); err != nil {
			log.Printf("[askplanner] workspace GC failed: %v", err)
		}
		if question == "reset" {
			if err := responder.Reset(conversationKey); err != nil {
				fmt.Printf("%s\n\n", usererr.OrDefault(err, "Agent couldn't reset the current session. Please retry."))
			} else {
				fmt.Println("Session reset.")
				fmt.Println()
			}
			continue
		}
		if selfcmd.IsWhoAmI(question) {
			fmt.Println()
			fmt.Printf("User Key: %s\nConversation Key: %s\n\n", clinicUserKey, conversationKey)
			continue
		}
		if cmd, matched, err := admin.ParseCommand(question); matched {
			fmt.Println()
			if err != nil {
				fmt.Printf("%s\n\n", usererr.OrDefault(err, "Agent couldn't process that request. Please retry. If it keeps failing, check the relay logs."))
				continue
			}
			answer, err := runAdminCommand(ctx, workspaceManager, responder, cmd)
			if err != nil {
				fmt.Printf("%s\n\n", usererr.OrDefault(err, "Agent couldn't process that request. Please retry. If it keeps failing, check the relay logs."))
				continue
			}
			fmt.Println(answer)
			fmt.Println()
			continue
		}
		if cmd, matched, err := modelcmd.ParseCommand(question); matched {
			fmt.Println()
			if err != nil {
				fmt.Printf("%s\n\n", usererr.OrDefault(err, "Agent couldn't process that request. Please retry. If it keeps failing, check the relay logs."))
				continue
			}
			answer, err := runModelCommand(ctx, workspaceManager, responder, prefetcher, tracker, conversationKey, clinicUserKey, cmd)
			if err != nil {
				fmt.Printf("%s\n\n", usererr.OrDefault(err, "Agent couldn't process that request. Please retry. If it keeps failing, check the relay logs."))
				continue
			}
			fmt.Println(answer)
			fmt.Println()
			continue
		}
		if cmd, matched, err := workspace.ParseCommand(question); matched {
			fmt.Println()
			if err != nil {
				fmt.Printf("%s\n\n", usererr.OrDefault(err, "Agent couldn't process that request. Please retry. If it keeps failing, check the relay logs."))
				continue
			}
			answer, err := runWorkspaceCommand(ctx, workspaceManager, responder, prefetcher, tracker, conversationKey, clinicUserKey, cmd)
			if err != nil {
				fmt.Printf("%s\n\n", usererr.OrDefault(err, "Agent couldn't process that request. Please retry. If it keeps failing, check the relay logs."))
				continue
			}
			fmt.Println(answer)
			fmt.Println()
			continue
		}

		fmt.Println()
		start := time.Now()
		ws, err := workspaceManager.Ensure(ctx, clinicUserKey)
		if err != nil {
			fmt.Printf("%s\n\n", usererr.OrDefault(err, "Agent couldn't process that request. Please retry. If it keeps failing, check the relay logs."))
			continue
		}
		enriched, err := prefetcher.Enrich(ctx, clinicUserKey, question, workspace.BindRuntimeContext(codex.RuntimeContext{UserKey: clinicUserKey}, ws))
		if err != nil {
			if msg := clinic.UserFacingMessage(err); msg != "" {
				log.Printf("[askplanner] clinic prefetch user-visible error: %v", err)
				if span := tracker.Begin("cli", clinicUserKey, conversationKey, question, responder.GetModelState(conversationKey).EffectiveModel, ws.EnvironmentHash); span != nil {
					span.ShortCircuit()
				}
				fmt.Printf("%s\n\n", msg)
				continue
			}
			if span := tracker.Begin("cli", clinicUserKey, conversationKey, question, responder.GetModelState(conversationKey).EffectiveModel, ws.EnvironmentHash); span != nil {
				span.Error(err)
			}
			fmt.Printf("%s\n\n", usererr.OrDefault(err, "Agent couldn't process that request. Please retry. If it keeps failing, check the relay logs."))
			continue
		}
		enriched.RuntimeContext = workspace.BindRuntimeContext(enriched.RuntimeContext, ws)
		enriched.RuntimeContext.UserKey = clinicUserKey
		if strings.TrimSpace(enriched.IntroReply) != "" {
			if span := tracker.Begin("cli", clinicUserKey, conversationKey, question, responder.GetModelState(conversationKey).EffectiveModel, ws.EnvironmentHash); span != nil {
				span.ShortCircuit()
			}
			fmt.Println(joinReplySections(enriched.IntroReply, formatWarning(enriched.Warning)))
			fmt.Println()
			continue
		}

		span := tracker.Begin("cli", clinicUserKey, conversationKey, question, responder.GetModelState(conversationKey).EffectiveModel, ws.EnvironmentHash)
		answer, err := responder.AnswerWithContext(ctx, conversationKey, question, enriched.RuntimeContext)
		if err != nil {
			if span != nil {
				span.Error(err)
			}
			fmt.Printf("%s\n\n", usererr.OrDefault(err, "Agent couldn't process that request. Please retry. If it keeps failing, check the relay logs."))
			continue
		}
		if span != nil {
			span.Success()
		}
		if strings.TrimSpace(enriched.Warning) != "" {
			answer = joinReplySections(formatWarning(enriched.Warning), answer)
		}
		log.Printf("[askplanner] request done conversation=%s elapsed=%s", conversationKey, time.Since(start))

		fmt.Println()
		fmt.Println(answer)
		fmt.Println()
	}
}

func runModelCommand(ctx context.Context, workspaceManager *workspace.Manager, responder *codex.Responder, prefetcher *clinic.Prefetcher, tracker *usage.QuestionTracker, conversationKey, userKey string, cmd *modelcmd.Command) (string, error) {
	status := ""
	switch cmd.Action {
	case "status":
		status = codex.FormatModelStatus(responder.GetModelState(conversationKey), "", false)
	case "set":
		result, err := responder.SetModel(conversationKey, cmd.Model)
		if err != nil {
			return "", err
		}
		summary := "Model settings updated for this conversation."
		if !result.Changed {
			summary = "Model settings unchanged for this conversation."
		}
		status = codex.FormatModelStatus(result.State, summary, result.SessionRestartNeeded)
	case "effort":
		result, err := responder.SetReasoningEffort(conversationKey, cmd.Effort)
		if err != nil {
			return "", err
		}
		summary := "Reasoning effort updated for this conversation."
		if !result.Changed {
			summary = "Reasoning effort unchanged for this conversation."
		}
		status = codex.FormatModelStatus(result.State, summary, result.SessionRestartNeeded)
	case "reset-effort":
		result, err := responder.ResetReasoningEffort(conversationKey)
		if err != nil {
			return "", err
		}
		summary := "Reasoning effort override cleared for this conversation."
		if !result.Changed {
			summary = "Reasoning effort already uses the default for this conversation."
		}
		status = codex.FormatModelStatus(result.State, summary, result.SessionRestartNeeded)
	case "reset":
		result, err := responder.ResetModel(conversationKey)
		if err != nil {
			return "", err
		}
		summary := "Model override cleared for this conversation."
		if !result.Changed {
			summary = "Model settings already use the default for this conversation."
		}
		status = codex.FormatModelStatus(result.State, summary, result.SessionRestartNeeded)
	default:
		return "", fmt.Errorf("unsupported model command: %s", cmd.Action)
	}

	if strings.TrimSpace(cmd.Question) == "" {
		return status, nil
	}

	ws, err := workspaceManager.Ensure(ctx, userKey)
	if err != nil {
		return "", err
	}
	enriched, err := prefetcher.Enrich(ctx, userKey, cmd.Question, workspace.BindRuntimeContext(codex.RuntimeContext{}, ws))
	if err != nil {
		if msg := clinic.UserFacingMessage(err); msg != "" {
			if span := tracker.Begin("cli", userKey, conversationKey, cmd.Question, responder.GetModelState(conversationKey).EffectiveModel, ws.EnvironmentHash); span != nil {
				span.ShortCircuit()
			}
			return joinReplySections(status, msg), nil
		}
		if span := tracker.Begin("cli", userKey, conversationKey, cmd.Question, responder.GetModelState(conversationKey).EffectiveModel, ws.EnvironmentHash); span != nil {
			span.Error(err)
		}
		return "", err
	}
	enriched.RuntimeContext = workspace.BindRuntimeContext(enriched.RuntimeContext, ws)
	if strings.TrimSpace(enriched.IntroReply) != "" {
		if span := tracker.Begin("cli", userKey, conversationKey, cmd.Question, responder.GetModelState(conversationKey).EffectiveModel, ws.EnvironmentHash); span != nil {
			span.ShortCircuit()
		}
		return joinReplySections(status, enriched.IntroReply, formatWarning(enriched.Warning)), nil
	}
	span := tracker.Begin("cli", userKey, conversationKey, cmd.Question, responder.GetModelState(conversationKey).EffectiveModel, ws.EnvironmentHash)
	answer, err := responder.AnswerWithContext(ctx, conversationKey, cmd.Question, enriched.RuntimeContext)
	if err != nil {
		if span != nil {
			span.Error(err)
		}
		return "", err
	}
	if span != nil {
		span.Success()
	}
	if strings.TrimSpace(enriched.Warning) != "" {
		answer = joinReplySections(formatWarning(enriched.Warning), answer)
	}
	return joinReplySections(status, answer), nil
}

func runWorkspaceCommand(ctx context.Context, manager *workspace.Manager, responder *codex.Responder, prefetcher *clinic.Prefetcher, tracker *usage.QuestionTracker, conversationKey, userKey string, cmd *workspace.Command) (string, error) {
	start := time.Now()
	var (
		ws                 *workspace.Workspace
		err                error
		environmentChanged bool
	)
	switch cmd.Action {
	case "status":
		ws, err = manager.Status(ctx, userKey)
	case "switch":
		ws, environmentChanged, err = manager.SwitchRepo(ctx, userKey, cmd.Repo, cmd.Ref)
	case "sync":
		ws, environmentChanged, err = manager.Sync(ctx, userKey, cmd.Repo)
	case "reset":
		ws, environmentChanged, err = manager.Reset(ctx, userKey, cmd.Repo)
	default:
		return "", fmt.Errorf("unsupported workspace command: %s", cmd.Action)
	}
	if err != nil {
		return "", err
	}
	if environmentChanged {
		notice := codex.WorkspaceSessionNotice{
			Message:            buildCLIWorkspaceChangeNotice(cmd),
			NewEnvironmentHash: ws.EnvironmentHash,
			ChangedAt:          time.Now().UTC(),
		}
		if err := responder.MarkWorkspaceChanged(userKey, conversationKey, notice); err != nil {
			log.Printf("[askplanner] mark workspace changed failed conversation=%s user=%s: %v", conversationKey, userKey, err)
		}
	}
	status := workspace.FormatStatus(ws)
	if strings.TrimSpace(cmd.Question) == "" {
		return status, nil
	}

	enriched, err := prefetcher.Enrich(ctx, userKey, cmd.Question, workspace.BindRuntimeContext(codex.RuntimeContext{UserKey: userKey}, ws))
	if err != nil {
		if msg := clinic.UserFacingMessage(err); msg != "" {
			if span := tracker.Begin("cli", userKey, conversationKey, cmd.Question, responder.GetModelState(conversationKey).EffectiveModel, ws.EnvironmentHash); span != nil {
				span.ShortCircuit()
			}
			return joinReplySections(status, msg), nil
		}
		if span := tracker.Begin("cli", userKey, conversationKey, cmd.Question, responder.GetModelState(conversationKey).EffectiveModel, ws.EnvironmentHash); span != nil {
			span.Error(err)
		}
		return "", err
	}
	enriched.RuntimeContext = workspace.BindRuntimeContext(enriched.RuntimeContext, ws)
	enriched.RuntimeContext.UserKey = userKey
	if strings.TrimSpace(enriched.IntroReply) != "" {
		if span := tracker.Begin("cli", userKey, conversationKey, cmd.Question, responder.GetModelState(conversationKey).EffectiveModel, ws.EnvironmentHash); span != nil {
			span.ShortCircuit()
		}
		return joinReplySections(status, enriched.IntroReply, formatWarning(enriched.Warning)), nil
	}
	span := tracker.Begin("cli", userKey, conversationKey, cmd.Question, responder.GetModelState(conversationKey).EffectiveModel, ws.EnvironmentHash)
	answer, err := responder.AnswerWithContext(ctx, conversationKey, cmd.Question, enriched.RuntimeContext)
	if err != nil {
		if span != nil {
			span.Error(err)
		}
		return "", err
	}
	if span != nil {
		span.Success()
	}
	if strings.TrimSpace(enriched.Warning) != "" {
		answer = joinReplySections(formatWarning(enriched.Warning), answer)
	}
	log.Printf("[askplanner] workspace command answered conversation=%s action=%s elapsed=%s", conversationKey, cmd.Action, time.Since(start))
	return joinReplySections(status, answer), nil
}

func runAdminCommand(ctx context.Context, workspaceManager *workspace.Manager, responder *codex.Responder, cmd *admin.Command) (string, error) {
	start := time.Now()
	switch cmd.Action {
	case "reset-user":
		workDir, err := workspaceManager.ResetUser(ctx, cmd.UserKey)
		if err != nil {
			return "", err
		}
		deletedSessions, err := responder.ResetByWorkDirPrefix(workDir)
		if err != nil {
			return "", err
		}
		log.Printf("[askplanner] admin reset-user done user=%s workdir=%s deleted_sessions=%d elapsed=%s",
			cmd.UserKey, workDir, deletedSessions, time.Since(start))
		return fmt.Sprintf("Reset user %s.\n- Workspace root: %s\n- Deleted sessions: %d", cmd.UserKey, workDir, deletedSessions), nil
	default:
		return "", fmt.Errorf("unsupported admin command: %s", cmd.Action)
	}
}

func joinReplySections(parts ...string) string {
	trimmed := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		trimmed = append(trimmed, part)
	}
	return strings.Join(trimmed, "\n\n")
}

func formatWarning(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	return "**Warning**\n" + message
}

func buildCLIWorkspaceChangeNotice(cmd *workspace.Command) string {
	if cmd == nil {
		return "Another chat changed the shared workspace, so this conversation had to start a new Codex session."
	}
	switch cmd.Action {
	case "switch":
		return fmt.Sprintf("Another chat switched the shared workspace to %s@%s, so this conversation had to start a new Codex session.", cmd.Repo, cmd.Ref)
	case "sync":
		return fmt.Sprintf("Another chat synced the shared workspace repo %s, which changed the workspace environment, so this conversation had to start a new Codex session.", cmd.Repo)
	case "reset":
		return fmt.Sprintf("Another chat reset the shared workspace repo %s to its default ref, so this conversation had to start a new Codex session.", cmd.Repo)
	default:
		return "Another chat changed the shared workspace, so this conversation had to start a new Codex session."
	}
}

func fatalStartup(component string, err error, hints ...string) {
	var sb strings.Builder
	sb.WriteString("startup error: ")
	sb.WriteString(component)
	if err != nil {
		sb.WriteString(": ")
		sb.WriteString(err.Error())
	}
	for _, hint := range hints {
		hint = strings.TrimSpace(hint)
		if hint == "" {
			continue
		}
		sb.WriteString("\n- ")
		sb.WriteString(hint)
	}
	message := sb.String()
	fmt.Fprintln(os.Stderr, message)
	log.Printf("%s", strings.ReplaceAll(message, "\n", " | "))
	os.Exit(1)
}
