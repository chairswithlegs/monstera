package domain

import "time"

// TrendingStatus is an entry in the pre-computed trending statuses index.
type TrendingStatus struct {
	StatusID string
	Score    float64
	RankedAt time.Time
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
