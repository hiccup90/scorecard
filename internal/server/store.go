package server

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func scanActivities(rows *sql.Rows) ([]Activity, error) {
	out := []Activity{}
	for rows.Next() {
		var a Activity
		var enabled int
		if err := rows.Scan(&a.ID, &a.Label, &a.BasePoints, &a.ScoreMode, &a.Icon, &a.Color, &a.Category, &a.SortOrder, &enabled); err != nil {
			return nil, err
		}
		a.Enabled = enabled == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

func checkinSelect() string {
	return `SELECT c.id,c.user_id,u.name,c.activity_id,a.label,a.icon,a.color,a.category,c.activity_date,c.submitted_at,c.status,c.source,c.base_points,c.score_mode,c.review_level,c.review_minutes,c.awarded_points,c.streak_bonus,c.counts_for_streak,c.review_note,c.reviewed_at,c.reverse_reason
		FROM checkins c JOIN users u ON u.id=c.user_id JOIN activities a ON a.id=c.activity_id`
}

func scanCheckins(rows *sql.Rows) ([]Checkin, error) {
	out := []Checkin{}
	for rows.Next() {
		var c Checkin
		var level, reviewedAt, note, reverseReason sql.NullString
		var minutes sql.NullInt64
		var counts int
		if err := rows.Scan(&c.ID, &c.UserID, &c.UserName, &c.ActivityID, &c.ActivityLabel, &c.ActivityIcon, &c.ActivityColor, &c.Category, &c.ActivityDate, &c.SubmittedAt, &c.Status, &c.Source, &c.BasePoints, &c.ScoreMode, &level, &minutes, &c.AwardedPoints, &c.StreakBonus, &counts, &note, &reviewedAt, &reverseReason); err != nil {
			return nil, err
		}
		if level.Valid {
			c.ReviewLevel = &level.String
		}
		if minutes.Valid {
			v := int(minutes.Int64)
			c.ReviewMinutes = &v
		}
		c.CountsForStreak = counts == 1
		if note.Valid {
			c.ReviewNote = note.String
		}
		if reviewedAt.Valid {
			c.ReviewedAt = &reviewedAt.String
		}
		if reverseReason.Valid {
			c.ReverseReason = reverseReason.String
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func checkinForUpdate(tx *sql.Tx, id int64) (Checkin, error) {
	row := tx.QueryRow(checkinSelect()+` WHERE c.id=?`, id)
	var c Checkin
	var level, reviewedAt, note, reverseReason sql.NullString
	var minutes sql.NullInt64
	var counts int
	if err := row.Scan(&c.ID, &c.UserID, &c.UserName, &c.ActivityID, &c.ActivityLabel, &c.ActivityIcon, &c.ActivityColor, &c.Category, &c.ActivityDate, &c.SubmittedAt, &c.Status, &c.Source, &c.BasePoints, &c.ScoreMode, &level, &minutes, &c.AwardedPoints, &c.StreakBonus, &counts, &note, &reviewedAt, &reverseReason); err != nil {
		return c, err
	}
	if level.Valid {
		c.ReviewLevel = &level.String
	}
	if minutes.Valid {
		v := int(minutes.Int64)
		c.ReviewMinutes = &v
	}
	c.CountsForStreak = counts == 1
	if note.Valid {
		c.ReviewNote = note.String
	}
	if reviewedAt.Valid {
		c.ReviewedAt = &reviewedAt.String
	}
	if reverseReason.Valid {
		c.ReverseReason = reverseReason.String
	}
	return c, nil
}

func award(mode string, base int, level string, minutes int) (int, string, string, int, error) {
	switch mode {
	case "quality":
		if level == "" {
			level = "pass"
		}
		multipliers := map[string]int{"pass": 1, "good": 2, "excellent": 3}
		labels := map[string]string{"pass": "及格", "good": "良好", "excellent": "优秀"}
		m, ok := multipliers[level]
		if !ok {
			return 0, "", "", 0, errors.New("无效的质量档位")
		}
		return base * m, labels[level], level, 0, nil
	case "duration":
		if minutes < 10 {
			return 0, "", "", 0, errors.New("时长至少 10 分钟")
		}
		units := minutes / 10
		return base * units, fmt.Sprintf("%d分钟（%d个单位）", minutes, units), "", minutes, nil
	default:
		return base, "默认分", "", 0, nil
	}
}

func recalcStreakTx(tx *sql.Tx, userID, activityID int64) (int, error) {
	rows, err := tx.Query(`SELECT DISTINCT activity_date FROM checkins WHERE user_id=? AND activity_id=? AND status='approved' AND counts_for_streak=1 ORDER BY activity_date DESC`, userID, activityID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var dates []time.Time
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return 0, err
		}
		d, err := time.Parse("2006-01-02", raw)
		if err != nil {
			continue
		}
		dates = append(dates, d)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	if len(dates) == 0 {
		_, err := tx.Exec(`DELETE FROM streaks WHERE user_id=? AND activity_id=?`, userID, activityID)
		return 0, err
	}

	streak := 1
	last := dates[0]
	for i := 1; i < len(dates); i++ {
		if int(last.Sub(dates[i]).Hours()/24) == 1 {
			streak++
			last = dates[i]
			continue
		}
		break
	}
	_, err = tx.Exec(`INSERT INTO streaks (user_id,activity_id,streak_days,last_date) VALUES (?,?,?,?) ON CONFLICT(user_id,activity_id) DO UPDATE SET streak_days=excluded.streak_days,last_date=excluded.last_date`, userID, activityID, streak, dates[0].Format("2006-01-02"))
	if err != nil {
		return 0, err
	}
	return min(max(streak-1, 0), 6), nil
}

func balanceTx(tx *sql.Tx, userID int64) (int, error) {
	var balance int
	err := tx.QueryRow(`SELECT COALESCE(SUM(change),0) FROM point_transactions WHERE user_id=?`, userID).Scan(&balance)
	return balance, err
}

func scanRewards(rows *sql.Rows) ([]Reward, error) {
	out := []Reward{}
	for rows.Next() {
		var r Reward
		var auto, enabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.Cost, &r.Description, &r.Stock, &auto, &enabled); err != nil {
			return nil, err
		}
		r.AutoApprove = auto == 1
		r.Enabled = enabled == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

func redemptionSelect() string {
	return `SELECT rd.id,rd.user_id,u.name,rd.reward_id,r.name,rd.cost_at_time,rd.status,rd.created_at,COALESCE(rd.reviewed_at,''),COALESCE(rd.review_note,'')
		FROM redemptions rd JOIN users u ON u.id=rd.user_id JOIN rewards r ON r.id=rd.reward_id`
}

func scanRedemptions(rows *sql.Rows) ([]Redemption, error) {
	out := []Redemption{}
	for rows.Next() {
		var r Redemption
		if err := rows.Scan(&r.ID, &r.UserID, &r.UserName, &r.RewardID, &r.RewardName, &r.CostAtTime, &r.Status, &r.CreatedAt, &r.ReviewedAt, &r.ReviewNote); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
