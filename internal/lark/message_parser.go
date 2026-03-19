package lark

import (
	"encoding/json"
	"fmt"
	"strings"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type postElement struct {
	Tag      string `json:"tag"`
	Text     string `json:"text"`
	Href     string `json:"href"`
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	ImageKey string `json:"image_key"`
	FileKey  string `json:"file_key"`
}

type postBody struct {
	Title   string          `json:"title"`
	Content [][]postElement `json:"content"`
}

type postPayload struct {
	Post map[string]postBody `json:"post"`
}

func ParseMessageEvent(event *larkim.P2MessageReceiveV1) (ParsedMessage, error) {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return ParsedMessage{}, fmt.Errorf("missing event message")
	}
	message := event.Event.Message

	var parsed ParsedMessage
	if message.MessageId != nil {
		parsed.MessageID = strings.TrimSpace(*message.MessageId)
	}
	if message.MessageType != nil {
		parsed.MessageType = strings.TrimSpace(*message.MessageType)
	}

	raw := ""
	if message.Content != nil {
		raw = strings.TrimSpace(*message.Content)
	}

	switch parsed.MessageType {
	case "text":
		var payload struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return ParsedMessage{}, fmt.Errorf("parse text content: %w", err)
		}
		parsed.UserText = strings.TrimSpace(payload.Text)
	case "image":
		var payload struct {
			ImageKey string `json:"image_key"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return ParsedMessage{}, fmt.Errorf("parse image content: %w", err)
		}
		if key := strings.TrimSpace(payload.ImageKey); key != "" {
			parsed.Attachments = append(parsed.Attachments, AttachmentRef{
				Kind:         AttachmentKindImage,
				ResourceType: "image",
				ResourceKey:  key,
			})
		}
	case "file":
		var payload struct {
			FileKey string `json:"file_key"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return ParsedMessage{}, fmt.Errorf("parse file content: %w", err)
		}
		if key := strings.TrimSpace(payload.FileKey); key != "" {
			parsed.Attachments = append(parsed.Attachments, AttachmentRef{
				Kind:         AttachmentKindFile,
				ResourceType: "file",
				ResourceKey:  key,
			})
		}
	case "post":
		postParsed, err := parsePostContent(raw)
		if err != nil {
			return ParsedMessage{}, err
		}
		parsed.UserText = postParsed.UserText
		parsed.Attachments = postParsed.Attachments
	default:
		if raw != "" {
			parsed.UserText = raw
		}
	}

	parsed.UserText = strings.TrimSpace(parsed.UserText)
	parsed.Attachments = dedupeAttachmentRefs(parsed.Attachments)
	return parsed, nil
}

func parsePostContent(raw string) (ParsedMessage, error) {
	var payload postPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ParsedMessage{}, fmt.Errorf("parse post content: %w", err)
	}

	var textSections []string
	var refs []AttachmentRef
	order := []string{"zh_cn", "en_us", "ja_jp"}
	seenLang := make(map[string]struct{}, len(payload.Post))

	for _, lang := range order {
		body, ok := payload.Post[lang]
		if !ok {
			continue
		}
		seenLang[lang] = struct{}{}
		textSections = append(textSections, renderPostBody(lang, body))
		refs = append(refs, extractPostAttachmentRefs(body)...)
	}
	for lang, body := range payload.Post {
		if _, ok := seenLang[lang]; ok {
			continue
		}
		textSections = append(textSections, renderPostBody(lang, body))
		refs = append(refs, extractPostAttachmentRefs(body)...)
	}

	return ParsedMessage{
		UserText:    strings.TrimSpace(strings.Join(compactSections(textSections), "\n\n")),
		Attachments: refs,
	}, nil
}

func renderPostBody(lang string, body postBody) string {
	var lines []string
	if title := strings.TrimSpace(body.Title); title != "" {
		lines = append(lines, fmt.Sprintf("[%s] %s", lang, title))
	}
	for _, paragraph := range body.Content {
		var parts []string
		for _, element := range paragraph {
			switch element.Tag {
			case "text":
				if text := strings.TrimSpace(element.Text); text != "" {
					parts = append(parts, text)
				}
			case "a":
				text := strings.TrimSpace(element.Text)
				href := strings.TrimSpace(element.Href)
				switch {
				case text != "" && href != "":
					parts = append(parts, fmt.Sprintf("%s (%s)", text, href))
				case text != "":
					parts = append(parts, text)
				case href != "":
					parts = append(parts, href)
				}
			case "at":
				name := strings.TrimSpace(element.UserName)
				if name == "" {
					name = strings.TrimSpace(element.UserID)
				}
				if name != "" {
					parts = append(parts, "@"+name)
				}
			}
		}
		line := strings.TrimSpace(strings.Join(parts, " "))
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func extractPostAttachmentRefs(body postBody) []AttachmentRef {
	var refs []AttachmentRef
	for _, paragraph := range body.Content {
		for _, element := range paragraph {
			switch element.Tag {
			case "img":
				if key := strings.TrimSpace(element.ImageKey); key != "" {
					refs = append(refs, AttachmentRef{
						Kind:         AttachmentKindImage,
						ResourceType: "image",
						ResourceKey:  key,
					})
				}
			case "media":
				if key := strings.TrimSpace(element.ImageKey); key != "" {
					refs = append(refs, AttachmentRef{
						Kind:         AttachmentKindImage,
						ResourceType: "image",
						ResourceKey:  key,
					})
				}
				if key := strings.TrimSpace(element.FileKey); key != "" {
					refs = append(refs, AttachmentRef{
						Kind:         AttachmentKindFile,
						ResourceType: "file",
						ResourceKey:  key,
					})
				}
			}
		}
	}
	return refs
}

func dedupeAttachmentRefs(refs []AttachmentRef) []AttachmentRef {
	seen := make(map[string]struct{}, len(refs))
	deduped := make([]AttachmentRef, 0, len(refs))
	for _, ref := range refs {
		key := ref.Kind + "|" + ref.ResourceType + "|" + ref.ResourceKey
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, ref)
	}
	return deduped
}

func compactSections(sections []string) []string {
	out := make([]string, 0, len(sections))
	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		out = append(out, section)
	}
	return out
}
