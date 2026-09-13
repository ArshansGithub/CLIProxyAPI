package helps

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Upstream's claude -> openai translator hoists a tool_result's images out of the
// tool message (the OpenAI API rejects image parts on a role=tool message) and
// relays them in the user message that follows, labelled with this notice. These
// two literals mirror internal/translator/openai/claude/openai_claude_request.go.
const (
	relayedToolResultImageNotice      = "Images returned by the preceding tool call(s):"
	relayedToolResultImagePlaceholder = "[Tool returned image content; the images follow in the next user message.]"
)

// foldRelayedToolResultImages folds a relayed tool-image user message back into
// the tool message it came from, as the omitted-image marker, and drops the
// relay message. It runs only for an upstream whose configured input modalities
// exclude images: without it the relay hands a text-only upstream the very
// image_url part that ShouldNormalizeOpenAIToolResultsForModel exists to remove.
func foldRelayedToolResultImages(payload []byte) []byte {
	for {
		messages := gjson.GetBytes(payload, "messages")
		if !messages.IsArray() {
			return payload
		}
		arr := messages.Array()
		folded := false
		for i := 1; i < len(arr); i++ {
			if arr[i].Get("role").String() != "user" || arr[i-1].Get("role").String() != "tool" {
				continue
			}
			items := arr[i].Get("content").Array()
			if len(items) == 0 || strings.TrimSpace(items[0].Get("text").String()) != relayedToolResultImageNotice {
				continue
			}
			markers := make([]string, 0, len(items))
			for _, item := range items[1:] {
				if isOpenAIImageToolResultPart(item) {
					markers = append(markers, openAIToolResultImageOmittedText)
				}
			}
			if len(markers) == 0 {
				continue
			}
			merged := strings.Join(markers, "\n\n")
			if toolText := arr[i-1].Get("content").String(); strings.TrimSpace(toolText) != "" && toolText != relayedToolResultImagePlaceholder {
				merged = toolText + "\n\n" + merged
			}
			updated, errSet := sjson.SetBytes(payload, fmt.Sprintf("messages.%d.content", i-1), merged)
			if errSet != nil {
				continue
			}
			updated, errDelete := sjson.DeleteBytes(updated, fmt.Sprintf("messages.%d", i))
			if errDelete != nil {
				continue
			}
			payload = updated
			folded = true
			break
		}
		if !folded {
			return payload
		}
	}
}
