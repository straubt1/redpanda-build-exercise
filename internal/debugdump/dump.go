package debugdump

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/straubt1/redpanda-build-exercise/internal/applog"
)

const Root = "/logs"

func Write(eventID, relPath string, data []byte) {
	id := filepath.Base(eventID)
	if id == "" || id == "." {
		applog.Err.Printf("debugdump skip bad event_id=%q", eventID)
		return
	}
	path := filepath.Join(Root, id, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		applog.Err.Printf("debugdump mkdir event_id=%s: %v", eventID, err)
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		applog.Err.Printf("debugdump write event_id=%s path=%s: %v", eventID, relPath, err)
	}
}

func WriteJSON(eventID, relPath string, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		applog.Err.Printf("debugdump json event_id=%s path=%s: %v", eventID, relPath, err)
		return
	}
	Write(eventID, relPath, buf.Bytes())
}

func WriteJSONBytes(eventID, relPath string, raw []byte) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		applog.Err.Printf("debugdump indent event_id=%s path=%s: %v", eventID, relPath, err)
		Write(eventID, relPath, raw)
		return
	}
	buf.WriteByte('\n')
	Write(eventID, relPath, buf.Bytes())
}
