package usage

import "testing"

func TestDedupQuestionEventEntriesPrefersLiveOverBackfilled(t *testing.T) {
	entries := []dedupEntry{
		{
			raw: jsonLine(`{"event_id":"live-1","source":"lark","user_key":"u1","conversation_key":"c1","question":"same","question_fingerprint":"fp1"}`),
			event: dedupQuestionEvent{
				EventID:             "live-1",
				Source:              sourceLark,
				UserKey:             "u1",
				ConversationKey:     "c1",
				Question:            "same",
				QuestionFingerprint: "fp1",
			},
		},
		{
			raw: jsonLine(`{"event_id":"backfill-1","source":"lark","user_key":"u1","conversation_key":"c1","question":"same","question_fingerprint":"fp1","backfilled":true}`),
			event: dedupQuestionEvent{
				EventID:             "backfill-1",
				Source:              sourceLark,
				UserKey:             "u1",
				ConversationKey:     "c1",
				Question:            "same",
				QuestionFingerprint: "fp1",
				Backfilled:          true,
			},
		},
	}

	got := dedupQuestionEventEntries(entries)
	if len(got) != 1 {
		t.Fatalf("deduped entries = %d, want 1", len(got))
	}
	if got[0].event.EventID != "live-1" {
		t.Fatalf("kept event id = %q, want live-1", got[0].event.EventID)
	}
}

func TestDedupQuestionEventEntriesKeepsBackfilledWithoutLiveMatch(t *testing.T) {
	entries := []dedupEntry{
		{
			raw: jsonLine(`{"event_id":"backfill-1","source":"lark","user_key":"u1","conversation_key":"c1","question":"same","question_fingerprint":"fp1","backfilled":true}`),
			event: dedupQuestionEvent{
				EventID:             "backfill-1",
				Source:              sourceLark,
				UserKey:             "u1",
				ConversationKey:     "c1",
				Question:            "same",
				QuestionFingerprint: "fp1",
				Backfilled:          true,
			},
		},
	}

	got := dedupQuestionEventEntries(entries)
	if len(got) != 1 {
		t.Fatalf("deduped entries = %d, want 1", len(got))
	}
}

func TestDedupQuestionEventEntriesKeepsMultipleLiveQuestions(t *testing.T) {
	entries := []dedupEntry{
		{
			raw: jsonLine(`{"event_id":"live-1","source":"lark","user_key":"u1","conversation_key":"c1","question":"same","question_fingerprint":"fp1"}`),
			event: dedupQuestionEvent{
				EventID:             "live-1",
				Source:              sourceLark,
				UserKey:             "u1",
				ConversationKey:     "c1",
				Question:            "same",
				QuestionFingerprint: "fp1",
			},
		},
		{
			raw: jsonLine(`{"event_id":"live-2","source":"lark","user_key":"u1","conversation_key":"c1","question":"same","question_fingerprint":"fp1"}`),
			event: dedupQuestionEvent{
				EventID:             "live-2",
				Source:              sourceLark,
				UserKey:             "u1",
				ConversationKey:     "c1",
				Question:            "same",
				QuestionFingerprint: "fp1",
			},
		},
	}

	got := dedupQuestionEventEntries(entries)
	if len(got) != 2 {
		t.Fatalf("deduped entries = %d, want 2", len(got))
	}
}

func jsonLine(s string) []byte {
	return []byte(s)
}
