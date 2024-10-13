package models

// User represents the structure of a user, including their ID and session.
type User struct {
	// the post struct
	Username string `json:"username"`
	Password string `json:"password"`
}

type VisitorRecord struct {
	UA string `json:"UA"`
	IP string `json:"IP"`
}
