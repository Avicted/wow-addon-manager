package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"strings"
	"time"

	"github.com/Avicted/wow-addon-manager/internal/source"
	"github.com/Avicted/wow-addon-manager/internal/state"
)

// With -json, stdout is one JSON object per line ("event" says which kind) and nothing else.
// The last line is always a result. The GUI reads this; see docs/DESIGN.md.
//
//	{"event":"log","level":"info|warning","message":"...","code":"wow_running"}
//	{"event":"checking","addons":[row...]}             before update checks start
//	{"event":"checked","addon":row,"done":3,"total":14} as each check finishes
//	{"event":"item","item":{...}}                       an update's progress, per addon
//	{"event":"result","ok":true,"data":{...}}
//	{"event":"result","ok":false,"error":{...},"data":{...}}  data when there is some to show

type logEvent struct {
	Event   string `json:"event"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

type checkingEvent struct {
	Event  string     `json:"event"`
	Addons []addonRow `json:"addons"`
}

type checkedEvent struct {
	Event string   `json:"event"`
	Addon addonRow `json:"addon"`
	Done  int      `json:"done"`
	Total int      `json:"total"`
}

type itemEvent struct {
	Event string     `json:"event"`
	Item  updateItem `json:"item"`
}

type resultEvent struct {
	Event string   `json:"event"`
	OK    bool     `json:"ok"`
	Data  any      `json:"data,omitempty"`
	Error *errInfo `json:"error,omitempty"`
}

// errInfo is an error with a code the GUI turns into a plain-words message.
type errInfo struct {
	Message string `json:"message"`
	// Code: locked, addons_dir, cancelled, rate_limit, http, network, bad_ref, not_found,
	// in_use, disk_full, permission or other.
	Code    string `json:"code"`
	Status  int    `json:"status,omitempty"`   // the HTTP status, for code http
	RetryAt string `json:"retry_at,omitempty"` // RFC 3339, for code rate_limit
	Path    string `json:"path,omitempty"`     // the missing directory, for code addons_dir
}

// badRef marks a failure to look up what the user typed (install, track), so the GUI can say
// "couldn't find an addon at that address" rather than "no version works with this game".
type badRef struct{ error }

func (b badRef) Unwrap() error { return b.error }

var (
	errAddonsDir   = errors.New("AddOns directory") // missing or unreadable
	errInterrupted = errors.New("interrupted")      // the user cancelled an update
)

func describe(err error) *errInfo {
	if err == nil {
		return nil
	}
	ei := &errInfo{Message: err.Error(), Code: "other"}
	var ue *url.Error
	var pe *fs.PathError
	switch {
	case errors.Is(err, state.ErrLocked):
		ei.Code = "locked"
	case errors.Is(err, errInterrupted):
		ei.Code = "cancelled"
	case errors.Is(err, errAddonsDir):
		ei.Code = "addons_dir"
		if errors.As(err, &pe) {
			ei.Path = pe.Path
		}
	case source.HTTPStatus(err) != 0:
		ei.Code, ei.Status = "http", source.HTTPStatus(err)
		if t, ok := source.RateLimitReset(err); ok {
			ei.Code, ei.RetryAt = "rate_limit", t.UTC().Format(time.RFC3339)
		}
	case errors.As(err, &ue):
		ei.Code = "network"
	case errors.As(err, new(badRef)):
		ei.Code = "bad_ref"
	case errors.Is(err, source.ErrNotFound):
		ei.Code = "not_found"
	case inUse(err):
		ei.Code = "in_use"
	case diskFull(err):
		ei.Code = "disk_full"
	case errors.Is(err, fs.ErrPermission):
		ei.Code = "permission"
	}
	return ei
}

// emit writes one JSON line. Update checks run in parallel, so lines are serialised.
func (e *env) emit(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		b, _ = json.Marshal(resultEvent{Event: "result", Error: describe(err)})
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	_, _ = e.out.Write(append(b, '\n'))
}

// logf writes a message for the user to stderr, or a log event with -json. A "warning: "
// prefix makes it a warning.
func (e *env) logf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if !e.json {
		_, _ = fmt.Fprintln(e.err, msg)
		return
	}
	level := "info"
	if rest, ok := strings.CutPrefix(msg, "warning: "); ok {
		level, msg = "warning", rest
	}
	e.emit(logEvent{Event: "log", Level: level, Message: strings.TrimSpace(msg)})
}

// warnWowRunning tells the user that the game only loads changed addons after a reload.
func (e *env) warnWowRunning(text string) {
	if e.json {
		e.emit(logEvent{Event: "log", Level: "warning", Message: text, Code: "wow_running"})
		return
	}
	e.logf("warning: %s", text)
}
