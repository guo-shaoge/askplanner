package codex

import (
	"fmt"
	"strconv"
	"strings"
)

type ModelOption struct {
	Slug                      string
	Description               string
	DefaultReasoningEffort    string
	SupportedReasoningEfforts []ReasoningEffortOption
}

type ReasoningEffortOption struct {
	Effort      string
	Description string
}

type ModelState struct {
	ConversationKey         string
	DefaultModel            string
	OverrideModel           string
	EffectiveModel          string
	ModelOptions            []ModelOption
	DefaultReasoningEffort  string
	OverrideReasoningEffort string
	ReasoningEffort         string
	ReasoningOptions        []ReasoningEffortOption
}

type ModelChangeResult struct {
	State                ModelState
	Changed              bool
	SessionRestartNeeded bool
}

func (r *Responder) GetModelState(conversationKey string) ModelState {
	record, _ := r.store.Get(conversationKey)
	return r.modelStateForRecord(conversationKey, record)
}

func (r *Responder) SetModel(conversationKey, model string) (ModelChangeResult, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return ModelChangeResult{}, fmt.Errorf("usage: /model set <model> [-- question]")
	}

	record, ok := r.store.Get(conversationKey)
	if !ok {
		record = SessionRecord{ConversationKey: conversationKey}
	}

	oldOverride := strings.TrimSpace(record.ModelOverride)
	oldEffortOverride := strings.TrimSpace(record.ReasoningEffortOverride)
	record.ModelOverride = r.normalizeModelOverride(model)
	r.normalizeRecordReasoningEffortOverride(&record)
	changed := oldOverride != record.ModelOverride || oldEffortOverride != record.ReasoningEffortOverride
	restartNeeded := false

	if err := r.persistConversationRecord(conversationKey, record); err != nil {
		return ModelChangeResult{}, err
	}
	return ModelChangeResult{
		State:                r.modelStateForRecord(conversationKey, record),
		Changed:              changed,
		SessionRestartNeeded: restartNeeded,
	}, nil
}

func (r *Responder) ResetModel(conversationKey string) (ModelChangeResult, error) {
	record, ok := r.store.Get(conversationKey)
	if !ok {
		record = SessionRecord{ConversationKey: conversationKey}
	}

	oldOverride := strings.TrimSpace(record.ModelOverride)
	oldEffortOverride := strings.TrimSpace(record.ReasoningEffortOverride)
	record.ModelOverride = ""
	r.normalizeRecordReasoningEffortOverride(&record)
	changed := oldOverride != "" || oldEffortOverride != record.ReasoningEffortOverride
	restartNeeded := false

	if err := r.persistConversationRecord(conversationKey, record); err != nil {
		return ModelChangeResult{}, err
	}
	return ModelChangeResult{
		State:                r.modelStateForRecord(conversationKey, record),
		Changed:              changed,
		SessionRestartNeeded: restartNeeded,
	}, nil
}

func (r *Responder) SetReasoningEffort(conversationKey, effort string) (ModelChangeResult, error) {
	effort = strings.TrimSpace(strings.ToLower(effort))
	if effort == "" {
		return ModelChangeResult{}, fmt.Errorf("usage: /model effort <level|reset> [-- question]")
	}

	record, ok := r.store.Get(conversationKey)
	if !ok {
		record = SessionRecord{ConversationKey: conversationKey}
	}
	if err := r.validateReasoningEffortForModel(r.effectiveModel(record), effort); err != nil {
		return ModelChangeResult{}, err
	}

	oldOverride := strings.TrimSpace(record.ReasoningEffortOverride)
	record.ReasoningEffortOverride = r.normalizeReasoningEffortOverride(r.effectiveModel(record), effort)
	changed := oldOverride != record.ReasoningEffortOverride

	if err := r.persistConversationRecord(conversationKey, record); err != nil {
		return ModelChangeResult{}, err
	}
	return ModelChangeResult{
		State:                r.modelStateForRecord(conversationKey, record),
		Changed:              changed,
		SessionRestartNeeded: false,
	}, nil
}

func (r *Responder) ResetReasoningEffort(conversationKey string) (ModelChangeResult, error) {
	record, ok := r.store.Get(conversationKey)
	if !ok {
		record = SessionRecord{ConversationKey: conversationKey}
	}

	oldOverride := strings.TrimSpace(record.ReasoningEffortOverride)
	record.ReasoningEffortOverride = ""
	changed := oldOverride != ""

	if err := r.persistConversationRecord(conversationKey, record); err != nil {
		return ModelChangeResult{}, err
	}
	return ModelChangeResult{
		State:                r.modelStateForRecord(conversationKey, record),
		Changed:              changed,
		SessionRestartNeeded: false,
	}, nil
}

func FormatModelStatus(state ModelState, summary string, sessionRestartNeeded bool) string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = "Model settings for this conversation."
	}

	effectiveModel := strings.TrimSpace(state.EffectiveModel)
	defaultModel := strings.TrimSpace(state.DefaultModel)
	reasoningEffort := strings.TrimSpace(state.ReasoningEffort)
	defaultReasoningEffort := strings.TrimSpace(state.DefaultReasoningEffort)

	var sb strings.Builder
	sb.WriteString(summary)
	sb.WriteByte('\n')
	sb.WriteString("- Model: ")
	sb.WriteString(effectiveModel)
	if defaultModel != "" && defaultModel != effectiveModel {
		sb.WriteByte('\n')
		sb.WriteString("- Default Model: ")
		sb.WriteString(defaultModel)
	}
	if reasoningEffort != "" {
		sb.WriteByte('\n')
		sb.WriteString("- Reasoning: ")
		sb.WriteString(reasoningEffort)
	}
	if defaultReasoningEffort != "" && defaultReasoningEffort != reasoningEffort {
		sb.WriteByte('\n')
		sb.WriteString("- Default Reasoning: ")
		sb.WriteString(defaultReasoningEffort)
	}
	if len(state.ReasoningOptions) > 0 {
		sb.WriteByte('\n')
		sb.WriteString("- Reasoning Options: ")
		sb.WriteString(joinReasoningEffortLabels(state.ReasoningOptions))
	}
	if sessionRestartNeeded {
		sb.WriteByte('\n')
		sb.WriteString("- Session: the next question will start a new Codex session with this model")
	}
	if len(state.ModelOptions) > 0 {
		sb.WriteByte('\n')
		sb.WriteString("Options:")
		for i, option := range state.ModelOptions {
			sb.WriteByte('\n')
			sb.WriteString(strconv.Itoa(i + 1))
			sb.WriteString(". ")
			sb.WriteString(strings.TrimSpace(option.Slug))
			if strings.TrimSpace(option.Slug) == strings.TrimSpace(state.EffectiveModel) {
				sb.WriteString(" (current)")
			}
			if desc := strings.TrimSpace(option.Description); desc != "" {
				sb.WriteString("  ")
				sb.WriteString(desc)
			}
		}
	}
	sb.WriteByte('\n')
	sb.WriteString("- Set: /model set <model>")
	sb.WriteByte('\n')
	sb.WriteString("- Set Effort: /model effort <level>")
	sb.WriteByte('\n')
	sb.WriteString("- Reset: /model reset")
	sb.WriteByte('\n')
	sb.WriteString("- Reset Effort: /model effort reset")
	if example := pickModelExample(state); example != "" {
		sb.WriteByte('\n')
		sb.WriteString("- Example: /model set ")
		sb.WriteString(example)
	}
	if effortExample := pickReasoningEffortExample(state); effortExample != "" {
		sb.WriteByte('\n')
		sb.WriteString("- Effort Example: /model effort ")
		sb.WriteString(effortExample)
	}
	return strings.TrimSpace(sb.String())
}

func (r *Responder) modelStateForRecord(conversationKey string, record SessionRecord) ModelState {
	options := r.availableModelOptions()
	effectiveModel := r.effectiveModel(record)
	currentModelOption := findModelOption(options, effectiveModel)
	reasoningOverride := r.reasoningEffortOverrideForModel(record)
	return ModelState{
		ConversationKey:         strings.TrimSpace(conversationKey),
		DefaultModel:            r.defaultModel,
		OverrideModel:           strings.TrimSpace(record.ModelOverride),
		EffectiveModel:          effectiveModel,
		ModelOptions:            options,
		DefaultReasoningEffort:  r.defaultReasoningEffortForModel(effectiveModel),
		OverrideReasoningEffort: reasoningOverride,
		ReasoningEffort:         r.effectiveReasoningEffort(record),
		ReasoningOptions:        r.reasoningOptionsForModel(currentModelOption),
	}
}

func (r *Responder) effectiveModel(record SessionRecord) string {
	if override := strings.TrimSpace(record.ModelOverride); override != "" {
		return override
	}
	return r.defaultModel
}

func (r *Responder) normalizeModelOverride(model string) string {
	model = strings.TrimSpace(model)
	if model == "" || model == r.defaultModel {
		return ""
	}
	return model
}

func (r *Responder) effectiveReasoningEffort(record SessionRecord) string {
	if override := r.reasoningEffortOverrideForModel(record); override != "" {
		return override
	}
	return r.defaultReasoningEffortForModel(r.effectiveModel(record))
}

func (r *Responder) normalizeReasoningEffortOverride(model, effort string) string {
	effort = strings.TrimSpace(strings.ToLower(effort))
	if effort == "" || effort == r.defaultReasoningEffortForModel(model) {
		return ""
	}
	return effort
}

func (r *Responder) reasoningEffortOverrideForModel(record SessionRecord) string {
	override := strings.TrimSpace(strings.ToLower(record.ReasoningEffortOverride))
	if override == "" {
		return ""
	}
	model := r.effectiveModel(record)
	if err := r.validateReasoningEffortForModel(model, override); err != nil {
		return ""
	}
	return r.normalizeReasoningEffortOverride(model, override)
}

func (r *Responder) normalizeRecordReasoningEffortOverride(record *SessionRecord) {
	if record == nil {
		return
	}
	record.ReasoningEffortOverride = r.reasoningEffortOverrideForModel(*record)
}

func (r *Responder) persistConversationRecord(conversationKey string, record SessionRecord) error {
	record.ConversationKey = strings.TrimSpace(conversationKey)
	if shouldPersistRecord(record) {
		return r.store.Put(record)
	}
	return r.store.Delete(conversationKey)
}

func shouldPersistRecord(record SessionRecord) bool {
	if strings.TrimSpace(record.ModelOverride) != "" {
		return true
	}
	if strings.TrimSpace(record.ReasoningEffortOverride) != "" {
		return true
	}
	return hasActiveSession(record) ||
		record.TurnCount > 0 ||
		len(record.Turns) > 0 ||
		!record.CreatedAt.IsZero() ||
		!record.LastActiveAt.IsZero() ||
		strings.TrimSpace(record.WorkDir) != "" ||
		strings.TrimSpace(record.EnvironmentHash) != "" ||
		strings.TrimSpace(record.PromptHash) != "" ||
		strings.TrimSpace(record.LastError) != ""
}

func hasActiveSession(record SessionRecord) bool {
	return strings.TrimSpace(record.SessionID) != ""
}

func pickModelExample(state ModelState) string {
	current := strings.TrimSpace(state.EffectiveModel)
	for _, option := range state.ModelOptions {
		slug := strings.TrimSpace(option.Slug)
		if slug == "" || slug == current {
			continue
		}
		return slug
	}
	if len(state.ModelOptions) > 0 {
		return strings.TrimSpace(state.ModelOptions[0].Slug)
	}
	return ""
}

func pickReasoningEffortExample(state ModelState) string {
	current := strings.TrimSpace(strings.ToLower(state.ReasoningEffort))
	for _, option := range state.ReasoningOptions {
		effort := strings.TrimSpace(strings.ToLower(option.Effort))
		if effort == "" || effort == current {
			continue
		}
		return effort
	}
	if len(state.ReasoningOptions) > 0 {
		return strings.TrimSpace(strings.ToLower(state.ReasoningOptions[0].Effort))
	}
	return ""
}

func joinReasoningEffortLabels(options []ReasoningEffortOption) string {
	labels := make([]string, 0, len(options))
	for _, option := range options {
		effort := strings.TrimSpace(option.Effort)
		if effort == "" {
			continue
		}
		labels = append(labels, effort)
	}
	return strings.Join(labels, ", ")
}

func findModelOption(options []ModelOption, slug string) *ModelOption {
	slug = strings.TrimSpace(slug)
	for i := range options {
		if strings.TrimSpace(options[i].Slug) == slug {
			return &options[i]
		}
	}
	return nil
}
