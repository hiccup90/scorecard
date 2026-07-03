package server

type User struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type Activity struct {
	ID         int64  `json:"id"`
	Label      string `json:"label"`
	BasePoints int    `json:"base_points"`
	ScoreMode  string `json:"score_mode"`
	Icon       string `json:"icon"`
	Color      string `json:"color"`
	Category   string `json:"category"`
	SortOrder  int    `json:"sort_order"`
	Enabled    bool   `json:"enabled"`
}

type Checkin struct {
	ID              int64   `json:"id"`
	UserID          int64   `json:"user_id"`
	UserName        string  `json:"user_name,omitempty"`
	ActivityID      int64   `json:"activity_id"`
	ActivityLabel   string  `json:"activity_label,omitempty"`
	ActivityIcon    string  `json:"activity_icon,omitempty"`
	ActivityColor   string  `json:"activity_color,omitempty"`
	Category        string  `json:"category,omitempty"`
	ActivityDate    string  `json:"activity_date"`
	SubmittedAt     string  `json:"submitted_at"`
	Status          string  `json:"status"`
	Source          string  `json:"source"`
	BasePoints      int     `json:"base_points"`
	ScoreMode       string  `json:"score_mode"`
	ReviewLevel     *string `json:"review_level,omitempty"`
	ReviewMinutes   *int    `json:"review_minutes,omitempty"`
	AwardedPoints   int     `json:"awarded_points"`
	StreakBonus     int     `json:"streak_bonus"`
	CountsForStreak bool    `json:"counts_for_streak"`
	ReviewNote      string  `json:"review_note,omitempty"`
	ReviewedAt      *string `json:"reviewed_at,omitempty"`
	ReverseReason   string  `json:"reverse_reason,omitempty"`
}

type Reward struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Cost        int    `json:"cost"`
	Description string `json:"description"`
	Stock       int    `json:"stock"`
	AutoApprove bool   `json:"auto_approve"`
	Enabled     bool   `json:"enabled"`
}

type Redemption struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"user_id"`
	UserName    string `json:"user_name"`
	RewardID    int64  `json:"reward_id"`
	RewardName  string `json:"reward_name"`
	CostAtTime  int    `json:"cost_at_time"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	ReviewedAt  string `json:"reviewed_at,omitempty"`
	ReviewNote  string `json:"review_note,omitempty"`
}

type Transaction struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	Change     int    `json:"change"`
	Reason     string `json:"reason"`
	SourceType string `json:"source_type"`
	SourceID   *int64 `json:"source_id,omitempty"`
	CreatedAt  string `json:"created_at"`
}

type Summary struct {
	Balance       int `json:"balance"`
	TodayTotal    int `json:"today_total"`
	PendingCount  int `json:"pending_count"`
	MaxStreakDays int `json:"max_streak_days"`
}
