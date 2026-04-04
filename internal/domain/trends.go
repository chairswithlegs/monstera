package domain

import "time"

// TrendingStatus is an entry in the pre-computed trending statuses index.
type TrendingStatus struct {
	StatusID string
	Score    float64
	RankedAt time.Time
}

// TrendingTagHistory is one row in the trending tag history cache table.
type TrendingTagHistory struct {
	HashtagID string
	Day       time.Time
	Uses      int64
	Accounts  int64
}

// HashtagDailyStats is the aggregated usage of a hashtag on a single calendar day.
type HashtagDailyStats struct {
	HashtagID   string
	HashtagName string
	Day         time.Time
	Uses        int64
	Accounts    int64
}

// TagHistoryDay is the usage statistics for a hashtag on a single calendar day.
type TagHistoryDay struct {
	Day      time.Time
	Uses     int64
	Accounts int64
}

// TrendingTag is a hashtag with its 7-day history, used for the trends/tags endpoint.
type TrendingTag struct {
	Hashtag Hashtag
	History []TagHistoryDay
}

// TrendingLinkStats is the aggregated usage of a URL on a single calendar day.
type TrendingLinkStats struct {
	URL      string
	Day      time.Time
	Uses     int64
	Accounts int64
}

// TrendingLinkHistoryDay is the usage statistics for a URL on a single calendar day.
type TrendingLinkHistoryDay struct {
	Day      time.Time
	Uses     int64
	Accounts int64
}

// TrendingLink is a URL with its 7-day history, used for the trends/links endpoint.
type TrendingLink struct {
	URL     string
	History []TrendingLinkHistoryDay
}
