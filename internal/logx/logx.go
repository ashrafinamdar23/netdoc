package logx

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/ashrafinamdar23/netdoc/internal/config"
)

type ctxKey string

const (
	ctxRunID     ctxKey = "run_id"
	ctxItemID    ctxKey = "item_id"
	ctxSystemID  ctxKey = "system_id"
	ctxComponent ctxKey = "component"
)

func WithRunID(ctx context.Context, runID string) context.Context {
	return context.WithValue(ctx, ctxRunID, runID)
}
func WithItemID(ctx context.Context, itemID int64) context.Context {
	return context.WithValue(ctx, ctxItemID, itemID)
}
func WithSystemID(ctx context.Context, systemID int64) context.Context {
	return context.WithValue(ctx, ctxSystemID, systemID)
}
func WithComponent(ctx context.Context, component string) context.Context {
	return context.WithValue(ctx, ctxComponent, component)
}

// BuildLogger creates a root logger that supports:
// - json/text output
// - global level
// - module-level overrides (by component string prefix match)
// - runtime filters (run_id/item_id/system_id)
func BuildLogger(cfg config.LoggingConfig) (*slog.Logger, func() error, error) {
	var out io.Writer = os.Stdout
	closeFn := func() error { return nil }

	if strings.EqualFold(cfg.Output, "file") {
		f, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, nil, err
		}
		out = f
		closeFn = f.Close
	}

	minLevel := parseLevel(cfg.Level)

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: minLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// keep default behavior for now
			return a
		},
	}

	if strings.EqualFold(cfg.Format, "text") {
		handler = slog.NewTextHandler(out, opts)
	} else {
		handler = slog.NewJSONHandler(out, opts)
	}

	// Wrap with filtering + module overrides
	h := &filterHandler{
		next:         handler,
		cfg:          cfg,
		globalMin:    minLevel,
		moduleLevels: normalizeMap(cfg.ModuleLevels),
	}

	return slog.New(h), closeFn, nil
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func normalizeMap(m map[string]string) map[string]slog.Level {
	out := map[string]slog.Level{}
	for k, v := range m {
		out[strings.TrimSpace(k)] = parseLevel(v)
	}
	return out
}

type filterHandler struct {
	next         slog.Handler
	cfg          config.LoggingConfig
	globalMin    slog.Level
	moduleLevels map[string]slog.Level
}

func (h *filterHandler) Enabled(ctx context.Context, level slog.Level) bool {
	// First: runtime filters (if set, drop non-matching)
	if h.cfg.Filters.RunID != "" {
		if v, _ := ctx.Value(ctxRunID).(string); v != h.cfg.Filters.RunID {
			return false
		}
	}
	if h.cfg.Filters.ItemID > 0 {
		if v, _ := ctx.Value(ctxItemID).(int64); v != h.cfg.Filters.ItemID {
			return false
		}
	}
	if h.cfg.Filters.SystemID > 0 {
		if v, _ := ctx.Value(ctxSystemID).(int64); v != h.cfg.Filters.SystemID {
			return false
		}
	}

	// Second: module-level override (based on component)
	min := h.globalMin
	if comp, _ := ctx.Value(ctxComponent).(string); comp != "" {
		for prefix, lvl := range h.moduleLevels {
			if strings.HasPrefix(comp, prefix) {
				min = lvl
				break
			}
		}
	}
	return level >= min && h.next.Enabled(ctx, level)
}

func (h *filterHandler) Handle(ctx context.Context, r slog.Record) error {
	// Enrich with context fields if present
	if v, _ := ctx.Value(ctxRunID).(string); v != "" {
		r.AddAttrs(slog.String("run_id", v))
	}
	if v, _ := ctx.Value(ctxItemID).(int64); v != 0 {
		r.AddAttrs(slog.Int64("item_id", v))
	}
	if v, _ := ctx.Value(ctxSystemID).(int64); v != 0 {
		r.AddAttrs(slog.Int64("system_id", v))
	}
	if v, _ := ctx.Value(ctxComponent).(string); v != "" {
		r.AddAttrs(slog.String("component", v))
	}
	return h.next.Handle(ctx, r)
}

func (h *filterHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &filterHandler{next: h.next.WithAttrs(attrs), cfg: h.cfg, globalMin: h.globalMin, moduleLevels: h.moduleLevels}
}
func (h *filterHandler) WithGroup(name string) slog.Handler {
	return &filterHandler{next: h.next.WithGroup(name), cfg: h.cfg, globalMin: h.globalMin, moduleLevels: h.moduleLevels}
}
