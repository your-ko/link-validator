package dd

// This file contains SLO query models

// sloSPElement represents an element in the "sp" array in SLO query string
type sloSPElement struct {
	P panel  `json:"p"`
	I string `json:"i"`
}

// panel represents the panel configuration with ID, active tab, and time frame
type panel struct {
	ID        string    `json:"id"`
	ActiveTab string    `json:"activeTab"`
	TimeFrame timeFrame `json:"timeFrame"`
}

// timeFrame represents the time configuration for the panel
type timeFrame struct {
	Start    int64  `json:"start"`
	End      int64  `json:"end"`
	Mode     string `json:"mode"`
	FromUser bool   `json:"fromUser"`
	Paused   bool   `json:"paused"`
}
