// Package larkbot implements the Feishu/Lark relay layer for askplanner.
//
// The package is intentionally split by responsibility:
//   - app/bootstrap: wiring long-lived dependencies and websocket lifecycle
//   - handler: normalizing an incoming event into a single answer pipeline
//   - prepare: interpreting local commands and message-type-specific branching
//   - message: extracting user-visible text and routing metadata from Feishu payloads
//   - thread_context: prefetching earlier messages from the same Feishu topic thread
//     so new bot sessions can start with thread-local context
//   - attachments: importing recent files and building Codex attachment context
//   - reply: rendering outgoing messages and ephemeral typing reactions
package larkbot
