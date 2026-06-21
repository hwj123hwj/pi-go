package bilibili

// ────────────────────────────────────────────────────────────────────────────
//  Bilibili API response types.
// ────────────────────────────────────────────────────────────────────────────

// SearchResponse is the wbi search API response.
type SearchResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Result []SearchResultItem `json:"result"`
	} `json:"data"`
}

// SearchResultItem is a single video in search results.
type SearchResultItem struct {
	Bvid    string `json:"bvid"`
	Title   string `json:"title"`    // May contain <em class="keyword"> tags
	Author  string `json:"author"`   // UP主 name
	Duration string `json:"duration"` // "4:58" format
	Pic     string `json:"pic"`      // Cover image URL
	Play    int64  `json:"play"`     // View count
}

// ViewResponse is the video info API response.
type ViewResponse struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Data    *ViewData `json:"data"`
}

// ViewData contains video metadata.
type ViewData struct {
	AID      int64  `json:"aid"`
	Bvid     string `json:"bvid"`
	CID      int64  `json:"cid"`
	Title    string `json:"title"`
	Desc     string `json:"desc"`
	Duration int    `json:"duration"` // seconds
	Pic      string `json:"pic"`      // cover URL
	Owner    struct {
		Name string `json:"name"`
	} `json:"owner"`
}

// PlayURLResponse is the DASH play URL API response.
type PlayURLResponse struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    *PlayURLData `json:"data"`
}

// PlayURLData contains DASH stream info.
type PlayURLData struct {
	Dash *DashData `json:"dash"`
	DURL []struct {
		URL string `json:"url"`
	} `json:"durl"`
}

// DashData contains separated audio/video streams.
type DashData struct {
	Audio []DashStream `json:"audio"`
	Video []DashStream `json:"video"`
}

// DashStream is a single DASH stream (audio or video).
type DashStream struct {
	ID        int      `json:"id"`        // Quality ID (e.g. 30280 for 180kbps AAC)
	Codecs    string   `json:"codecs"`    // e.g. "mp4a.40.2"
	Bandwidth int      `json:"bandwidth"` // bits/sec
	BaseURL   string   `json:"baseUrl"`   // Primary download URL
	BackupURL []string `json:"backupUrl"` // Backup URLs
}

// RankingResponse is the ranking API response.
type RankingResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []RankingItem `json:"list"`
	} `json:"data"`
}

// RankingItem is a single video in the ranking.
type RankingItem struct {
	AID      int64  `json:"aid"`
	Bvid     string `json:"bvid"`
	Title    string `json:"title"`
	Duration int    `json:"duration"` // seconds
	Pic      string `json:"pic"`
	Owner    struct {
		Name string `json:"name"`
	} `json:"owner"`
	Stat struct {
		View int64 `json:"view"`
	} `json:"stat"`
}
