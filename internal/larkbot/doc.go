// Package larkbot implements the Feishu/Lark relay layer for askplanner.
//
// The package is intentionally split by responsibility:
// - app/bootstrap: wiring long-lived dependencies and websocket lifecycle
// - handler: normalizing an incoming event into a single answer pipeline
// - message: extracting user-visible text and routing metadata from Feishu payloads
// - attachments: importing recent files and building Codex attachment context
// - reply: rendering outgoing messages and ephemeral typing reactions
package larkbot
