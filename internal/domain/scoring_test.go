package domain

import "testing"

func TestAward(t *testing.T) {
	pts, _, _, _, err := Award(ScoreDefault, 3, "", 0)
	if err != nil || pts != 3 {
		t.Fatalf("default: %v %v", pts, err)
	}
	pts, _, level, _, err := Award(ScoreQuality, 2, "excellent", 0)
	if err != nil || pts != 6 || level != "excellent" {
		t.Fatalf("quality: %v %v %v", pts, level, err)
	}
	_, _, _, _, err = Award(ScoreDuration, 1, "", 5)
	if err == nil {
		t.Fatal("expected duration error")
	}
}

func TestStreakBonus(t *testing.T) {
	if StreakBonus(1) != 0 || StreakBonus(2) != 1 || StreakBonus(10) != 6 {
		t.Fatal("streak bonus mismatch")
	}
}
