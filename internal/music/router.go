package music

import (
	"context"
	"fmt"
)

// ────────────────────────────────────────────────────────────────────────────
//  SourceRouter dispatches MusicSource method calls to the correct backend
//  based on the Source prefix in composite IDs (e.g. "netease:12345").
//  It also supports specifying the source explicitly via context or parameter.
// ────────────────────────────────────────────────────────────────────────────

// SourceRouter multiplexes multiple MusicSource backends.
type SourceRouter struct {
	sources  map[Source]MusicSource
	default_ Source
}

// NewSourceRouter creates a router with the given backends.
// The default source is used when no source is specified.
func NewSourceRouter(defaultSrc Source, sources ...MusicSource) *SourceRouter {
	m := make(map[Source]MusicSource, len(sources))
	for _, s := range sources {
		m[s.Source()] = s
	}
	return &SourceRouter{sources: m, default_: defaultSrc}
}

// DefaultSource returns the default music source.
func (r *SourceRouter) DefaultSource() Source {
	return r.default_
}

// SourceNames returns the list of registered source names.
func (r *SourceRouter) SourceNames() []string {
	names := make([]string, 0, len(r.sources))
	for k := range r.sources {
		names = append(names, string(k))
	}
	return names
}

// Resolve returns the MusicSource for the given source name.
// If src is empty, the default source is used.
func (r *SourceRouter) Resolve(src Source) (MusicSource, error) {
	if src == "" {
		src = r.default_
	}
	s, ok := r.sources[src]
	if !ok {
		return nil, fmt.Errorf("unknown music source: %q (available: %v)", src, r.SourceNames())
	}
	return s, nil
}

// ByCompositeID parses a composite ID ("netease:12345") and returns the
// appropriate source and raw ID.
func (r *SourceRouter) ByCompositeID(compositeID string) (MusicSource, string, error) {
	src, rawID := ParseSourceID(compositeID)
	s, err := r.Resolve(src)
	if err != nil {
		return nil, "", err
	}
	return s, rawID, nil
}

// ── MusicSource interface implementation (delegates to resolved source) ──

func (r *SourceRouter) Source() Source { return r.default_ }

func (r *SourceRouter) Search(ctx context.Context, query string, limit int, src Source) (*SearchResult, error) {
	s, err := r.Resolve(src)
	if err != nil {
		return nil, err
	}
	return s.Search(ctx, query, limit)
}

func (r *SourceRouter) GetSongByID(ctx context.Context, rawID string, src Source) (*Song, error) {
	s, err := r.Resolve(src)
	if err != nil {
		return nil, err
	}
	return s.GetSongByID(ctx, rawID)
}

func (r *SourceRouter) GetAudioURL(ctx context.Context, compositeID string) (string, error) {
	s, rawID, err := r.ByCompositeID(compositeID)
	if err != nil {
		return "", err
	}
	return s.GetAudioURL(ctx, rawID)
}

func (r *SourceRouter) GetLyrics(ctx context.Context, compositeID string) (*Lyrics, error) {
	s, rawID, err := r.ByCompositeID(compositeID)
	if err != nil {
		return nil, err
	}
	return s.GetLyrics(ctx, rawID)
}

func (r *SourceRouter) GetPlaylistDetail(ctx context.Context, compositeID string) (*PlaylistDetail, error) {
	s, rawID, err := r.ByCompositeID(compositeID)
	if err != nil {
		return nil, err
	}
	return s.GetPlaylistDetail(ctx, rawID)
}

func (r *SourceRouter) GetRankings(ctx context.Context, src Source) ([]RankingEntry, error) {
	s, err := r.Resolve(src)
	if err != nil {
		return nil, err
	}
	return s.GetRankings(ctx)
}

func (r *SourceRouter) GetTopList(ctx context.Context, compositeID string) (*PlaylistDetail, error) {
	s, rawID, err := r.ByCompositeID(compositeID)
	if err != nil {
		return nil, err
	}
	return s.GetTopList(ctx, rawID)
}

func (r *SourceRouter) GetNewSongs(ctx context.Context, limit int, src Source) ([]Song, error) {
	s, err := r.Resolve(src)
	if err != nil {
		return nil, err
	}
	return s.GetNewSongs(ctx, limit)
}

func (r *SourceRouter) GetDailyRecommend(ctx context.Context, src Source) (*PlaylistDetail, error) {
	s, err := r.Resolve(src)
	if err != nil {
		return nil, err
	}
	return s.GetDailyRecommend(ctx)
}
