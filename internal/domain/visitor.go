package domain

import "time"

// VisitorRecord tracks unique visitors by IP + User-Agent.
// On first visit a document is created; subsequent visits from the same
// device increment VisitCount and refresh LastVisit.
type VisitorRecord struct {
	IP         string    `bson:"ip"`
	UserAgent  string    `bson:"user_agent"`
	Device     string    `bson:"device"`   // mobile | desktop | tablet
	OS         string    `bson:"os"`       // Windows | macOS | Linux | iOS | Android | Other
	Browser    string    `bson:"browser"`  // Chrome | Firefox | Safari | Edge | Other
	FirstVisit time.Time `bson:"first_visit"`
	LastVisit  time.Time `bson:"last_visit"`
	VisitCount int64     `bson:"visit_count"`
}
