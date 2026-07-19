package domain

import "fmt"

// Award computes points for a score mode.
// Returns: points, reasonSuffix, normalizedLevel, minutes, error.
func Award(mode string, base int, level string, minutes int) (int, string, string, int, error) {
	switch mode {
	case ScoreQuality:
		if level == "" {
			level = "pass"
		}
		multipliers := map[string]int{"pass": 1, "good": 2, "excellent": 3}
		labels := map[string]string{"pass": "及格", "good": "良好", "excellent": "优秀"}
		m, ok := multipliers[level]
		if !ok {
			return 0, "", "", 0, NewAppError("invalid_level", "无效的质量档位", ErrInvalidInput)
		}
		return base * m, labels[level], level, 0, nil
	case ScoreDuration:
		if minutes < 10 {
			return 0, "", "", 0, NewAppError("invalid_duration", "时长至少 10 分钟", ErrInvalidInput)
		}
		units := minutes / 10
		return base * units, fmt.Sprintf("%d分钟（%d个单位）", minutes, units), "", minutes, nil
	default:
		return base, "默认分", "", 0, nil
	}
}

// StreakBonus returns consecutive-day bonus points (capped).
// streakDays is the current consecutive length after including today's approval.
func StreakBonus(streakDays int) int {
	return Min(Max(streakDays-1, 0), 6)
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func ValidScoreMode(v string) bool {
	return v == ScoreDefault || v == ScoreQuality || v == ScoreDuration
}
