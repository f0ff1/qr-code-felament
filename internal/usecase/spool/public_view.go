package spool

type PublicSpoolView struct {
	Material        string `json:"material"`
	Color           string `json:"color"`
	RemainingWeight int    `json:"remaining_weight"`
	InitialWeight   int    `json:"initial_weight"`
	Status          string `json:"status"`
	QRToken         string `json:"qr_token"`
}
