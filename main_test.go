package main

import (
	"testing"
	"time"
)

func TestDate_BusinessDaysUntil_Mon_Mon(t *testing.T) {
	start := time.Date(2018, 11, 5, 0, 0, 0, 0, time.Local)
	end := time.Date(2018, 11, 5, 0, 0, 0, 0, time.Local)

	startDate := DateOf(start)
	endDate := DateOf(end)

	days := startDate.BusinessDaysUntil(endDate)
	if days != 0 {
		t.Fatalf("expected 0 for days, got %d", days)
	}
}

func TestDate_BusinessDaysUntil_Mon_Tue(t *testing.T) {
	start := time.Date(2018, 11, 5, 0, 0, 0, 0, time.Local)
	end := time.Date(2018, 11, 6, 0, 0, 0, 0, time.Local)

	startDate := DateOf(start)
	endDate := DateOf(end)

	days := startDate.BusinessDaysUntil(endDate)
	if days != 1 {
		t.Fatalf("expected 1 for days, got %d", days)
	}
}

func TestDate_BusinessDaysUntil_Mon_Wed(t *testing.T) {
	start := time.Date(2018, 11, 5, 0, 0, 0, 0, time.Local)
	end := time.Date(2018, 11, 7, 0, 0, 0, 0, time.Local)

	startDate := DateOf(start)
	endDate := DateOf(end)

	days := startDate.BusinessDaysUntil(endDate)
	if days != 2 {
		t.Fatalf("expected 2 for days, got %d", days)
	}
}

func TestDate_BusinessDaysUntil_Fri_Fri(t *testing.T) {
	start := time.Date(2018, 11, 2, 0, 0, 0, 0, time.Local)
	end := time.Date(2018, 11, 2, 0, 0, 0, 0, time.Local)

	startDate := DateOf(start)
	endDate := DateOf(end)

	days := startDate.BusinessDaysUntil(endDate)
	if days != 0 {
		t.Fatalf("expected 0 for days, got %d", days)
	}
}

func TestDate_BusinessDaysUntil_Fri_Sat(t *testing.T) {
	start := time.Date(2018, 11, 2, 0, 0, 0, 0, time.Local)
	end := time.Date(2018, 11, 3, 0, 0, 0, 0, time.Local)

	startDate := DateOf(start)
	endDate := DateOf(end)

	days := startDate.BusinessDaysUntil(endDate)
	if days != 1 {
		t.Fatalf("expected 1 for days, got %d", days)
	}
}

func TestDate_BusinessDaysUntil_Fri_Sun(t *testing.T) {
	start := time.Date(2018, 11, 2, 0, 0, 0, 0, time.Local)
	end := time.Date(2018, 11, 4, 0, 0, 0, 0, time.Local)

	startDate := DateOf(start)
	endDate := DateOf(end)

	days := startDate.BusinessDaysUntil(endDate)
	if days != 1 {
		t.Fatalf("expected 1 for days, got %d", days)
	}
}

func TestDate_BusinessDaysUntil_Fri_Mon(t *testing.T) {
	start := time.Date(2018, 11, 2, 0, 0, 0, 0, time.Local)
	end := time.Date(2018, 11, 5, 0, 0, 0, 0, time.Local)

	startDate := DateOf(start)
	endDate := DateOf(end)

	days := startDate.BusinessDaysUntil(endDate)
	if days != 1 {
		t.Fatalf("expected 1 for days, got %d", days)
	}
}

func TestDate_BusinessDaysUntil_Fri_Tue(t *testing.T) {
	start := time.Date(2018, 11, 2, 0, 0, 0, 0, time.Local)
	end := time.Date(2018, 11, 6, 0, 0, 0, 0, time.Local)

	startDate := DateOf(start)
	endDate := DateOf(end)

	days := startDate.BusinessDaysUntil(endDate)
	if days != 2 {
		t.Fatalf("expected 2 for days, got %d", days)
	}
}

func TestDate_BusinessDaysUntil_Fri_Fri2(t *testing.T) {
	start := time.Date(2018, 11, 2, 0, 0, 0, 0, time.Local)
	end := time.Date(2018, 11, 2+7, 0, 0, 0, 0, time.Local)

	startDate := DateOf(start)
	endDate := DateOf(end)

	days := startDate.BusinessDaysUntil(endDate)
	if days != 5 {
		t.Fatalf("expected 5 for days, got %d", days)
	}
}

func TestDate_BusinessDaysUntil_Fri_Fri3(t *testing.T) {
	start := time.Date(2018, 11, 2, 0, 0, 0, 0, time.Local)
	end := time.Date(2018, 11, 2+7+7, 0, 0, 0, 0, time.Local)

	startDate := DateOf(start)
	endDate := DateOf(end)

	days := startDate.BusinessDaysUntil(endDate)
	if days != 10 {
		t.Fatalf("expected 10 for days, got %d", days)
	}
}
