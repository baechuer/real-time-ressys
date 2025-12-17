package models

// Email represents an email to be sent
type Email struct {
	To      string
	Subject string
	Body    string // HTML body
	From    string
	FromName string
}

