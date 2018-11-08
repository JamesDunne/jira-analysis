package main

import "time"

type Date struct {
	time.Time
}

func DateOf(t time.Time) Date {
	// Grab local date:
	//_, zoneOffset := t.Zone()
	l := t.Location()
	y, m, d := t.Date()
	// Build new date:
	return Date{time.Date(y, m, d, 6, 0, 0, 0, l)}
}

func (date Date) NextDate() Date {
	return DateOf(date.Time.Add(25 * time.Hour))
}

func (date Date) BusinessDaysUntil(until Date) int {
	// Count weekdays, skipping weekends:
	days := 0
	d := date

	_, startOffset := date.Zone()
	_, untilOffset := until.Zone()
	untilTime := until.In(date.Location()).Add(time.Duration(untilOffset-startOffset) * time.Second)
	//fmt.Printf("from %s to %s\n", date.Time, untilTime)

	for d.Time.Before(untilTime) {
		//fmt.Printf("  %d %s\n", days, d)

		days++
		d = d.NextDate()

		if d.Time.Weekday() == time.Saturday {
			d = d.NextDate()
		}
		if d.Time.Weekday() == time.Sunday {
			d = d.NextDate()
		}
	}

	//fmt.Printf("  %d %s\n", days, d)

	return days
}
