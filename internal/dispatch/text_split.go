package dispatch

import (
	"fmt"
	"unicode/utf8"

	domainsink "telegram-message-forward/internal/domain/sink"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

const segmentMarkerReserve = 16

type textLimit struct {
	maxRunes int
	maxBytes int
}

func splitPayload(payload pluginsink.Payload, caps domainsink.Capabilities) []pluginsink.Payload {
	limit := textLimit{maxRunes: caps.MaxTextLength}
	if caps.MaxTextBytes != nil {
		limit.maxBytes = caps.MaxTextBytes[payload.Format]
	}
	chunks := splitText(payload.Text, limit)
	if len(chunks) <= 1 {
		return []pluginsink.Payload{payload}
	}

	parts := make([]pluginsink.Payload, 0, len(chunks))
	for i, chunk := range chunks {
		part := payload
		part.Text = fmt.Sprintf("[%d/%d] %s", i+1, len(chunks), chunk)
		if i > 0 {
			part.Media = nil
			part.FallbackText = ""
		}
		parts = append(parts, part)
	}
	return parts
}

func splitText(text string, limit textLimit) []string {
	if text == "" || limit.fits(text) || (limit.maxRunes <= 0 && limit.maxBytes <= 0) {
		return []string{text}
	}

	contentLimit := limit
	if contentLimit.maxRunes > 0 {
		contentLimit.maxRunes -= segmentMarkerReserve
	}
	if contentLimit.maxBytes > 0 {
		contentLimit.maxBytes -= segmentMarkerReserve
	}
	if contentLimit.maxRunes <= 0 && limit.maxRunes > 0 || contentLimit.maxBytes <= 0 && limit.maxBytes > 0 {
		return []string{text}
	}

	runes := []rune(text)
	chunks := make([]string, 0, 2)
	for start := 0; start < len(runes); {
		end := maxChunkEnd(runes, start, contentLimit)
		if end >= len(runes) {
			chunks = append(chunks, string(runes[start:]))
			break
		}
		end = preferredBreak(runes, start, end)
		chunks = append(chunks, string(runes[start:end]))
		start = end
	}
	return chunks
}

func (l textLimit) fits(text string) bool {
	if l.maxRunes > 0 && utf8.RuneCountInString(text) > l.maxRunes {
		return false
	}
	return l.maxBytes <= 0 || len(text) <= l.maxBytes
}

func maxChunkEnd(runes []rune, start int, limit textLimit) int {
	bytesUsed := 0
	end := start
	for end < len(runes) {
		if limit.maxRunes > 0 && end-start >= limit.maxRunes {
			break
		}
		runeBytes := utf8.RuneLen(runes[end])
		if limit.maxBytes > 0 && bytesUsed+runeBytes > limit.maxBytes {
			break
		}
		bytesUsed += runeBytes
		end++
	}
	return end
}

func preferredBreak(runes []rune, start, end int) int {
	min := start + (end-start)/3
	for i := end - 1; i > min; i-- {
		if runes[i-1] == '\n' && runes[i] == '\n' {
			return i + 1
		}
	}
	for i := end - 1; i >= min; i-- {
		if runes[i] == '\n' {
			return i + 1
		}
	}
	for i := end - 1; i >= min; i-- {
		switch runes[i] {
		case '。', '！', '？', '；', '.', '!', '?', ';':
			return i + 1
		}
	}
	return end
}
