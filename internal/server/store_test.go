package server

import "testing"

func TestAwardDefault(t *testing.T) {
	pts, suffix, level, minutes, err := award("default", 3, "", 0)
	if err != nil || pts != 3 || suffix == "" || level != "" || minutes != 0 {
		t.Fatalf("default: pts=%d suffix=%q level=%q minutes=%d err=%v", pts, suffix, level, minutes, err)
	}
}

func TestAwardQuality(t *testing.T) {
	pts, _, level, _, err := award("quality", 2, "excellent", 0)
	if err != nil || pts != 6 || level != "excellent" {
		t.Fatalf("quality excellent: pts=%d level=%s err=%v", pts, level, err)
	}
	_, _, _, _, err = award("quality", 2, "bad", 0)
	if err == nil {
		t.Fatal("expected invalid level error")
	}
}

func TestAwardDuration(t *testing.T) {
	pts, _, _, minutes, err := award("duration", 1, "", 35)
	if err != nil || pts != 3 || minutes != 35 {
		t.Fatalf("duration: pts=%d minutes=%d err=%v", pts, minutes, err)
	}
	_, _, _, _, err = award("duration", 1, "", 5)
	if err == nil {
		t.Fatal("expected min minutes error")
	}
}
