// Package context provides the tool that resolves a spill handle.
//
// This tool exists because the spill mechanism was, until it was written, a
// promise the system could not keep. Oversized tool results were replaced by a
// digest ending "fetch the rest by handle" — and no tool could fetch a handle.
// A model given a truncated file and told to retrieve the rest has exactly one
// move available: read the file again. It does that until its iteration budget
// is gone, which is what an agent loop looks like when the loop is the system's
// fault rather than the model's.
package context

import (
	stdcontext "context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/giri-ms19/testplan-agent/internal/memory"
	"github.com/giri-ms19/testplan-agent/internal/tool"
)

// FetchTool returns a payload that was spilled out of the conversation.
type FetchTool struct {
	Store *memory.SpillStore
	// MaxCharacters bounds one fetch so retrieving a spilled payload cannot
	// itself swamp the window. Zero means unbounded.
	MaxCharacters int
}

func (fetchTool *FetchTool) Name() string { return "context_fetch" }

func (fetchTool *FetchTool) Description() string {
	return "Retrieve the full content of an earlier tool result that was too large to keep in view. " +
		"Pass the handle shown with the digest, for example \"spill-001-1a2b3c4d\". " +
		"Use startCharacter to continue where a previous fetch ended."
}

func (fetchTool *FetchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type":"object",
  "properties":{
    "handle":{"type":"string","description":"The handle shown beside the digest, e.g. spill-001-1a2b3c4d"},
    "startCharacter":{"type":"integer","minimum":0,"description":"Resume from this offset; omit to start at the beginning"}
  },
  "required":["handle"],
  "additionalProperties":false
}`)
}

func (fetchTool *FetchTool) Idempotent() bool { return true }

type fetchArguments struct {
	Handle         string `json:"handle"`
	StartCharacter int    `json:"startCharacter"`
}

func (fetchTool *FetchTool) Invoke(
	ctx stdcontext.Context, arguments json.RawMessage,
) (tool.Result, error) {
	var parsedArguments fetchArguments
	if err := json.Unmarshal(arguments, &parsedArguments); err != nil {
		return tool.Result{}, tool.Correctable(fetchTool.Name(),
			"arguments must be a JSON object with a required string field \"handle\"")
	}
	handle := strings.TrimSpace(parsedArguments.Handle)
	if handle == "" {
		return tool.Result{}, tool.Correctable(fetchTool.Name(),
			"\"handle\" is required; use the handle printed beside the digest")
	}

	payload, found := fetchTool.Store.Get(handle)
	if !found {
		// Naming the handles that do exist turns a guess into a correction.
		known := fetchTool.Store.Handles()
		if len(known) == 0 {
			return tool.Result{}, tool.Correctable(fetchTool.Name(), fmt.Sprintf(
				"no such handle %q, and nothing has been spilled in this task; "+
					"you already have everything you were shown", handle))
		}
		return tool.Result{}, tool.Correctable(fetchTool.Name(), fmt.Sprintf(
			"no such handle %q. Available handles: %s", handle, strings.Join(known, ", ")))
	}

	startCharacter := parsedArguments.StartCharacter
	if startCharacter < 0 || startCharacter > len(payload) {
		startCharacter = 0
	}
	slice := payload[startCharacter:]

	truncated := false
	if fetchTool.MaxCharacters > 0 && len(slice) > fetchTool.MaxCharacters {
		slice = slice[:fetchTool.MaxCharacters]
		truncated = true
	}

	var content strings.Builder
	content.WriteString(slice)
	if truncated {
		fmt.Fprintf(&content,
			"\n\n… %d characters remain. Continue with startCharacter=%d, "+
				"but only if what you need is definitely not above.",
			len(payload)-startCharacter-len(slice), startCharacter+len(slice))
	}
	return tool.Text(content.String(), tool.Internal()), nil
}
