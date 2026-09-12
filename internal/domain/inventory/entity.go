package inventory

import "time"

type MaterialSummary struct {
	Material string
	Color    string
	Total    int
}

type Transaction struct {
	ID        string
	SpoolID   string
	Type      string
	Weight    int
	CreatedAt time.Time
}
