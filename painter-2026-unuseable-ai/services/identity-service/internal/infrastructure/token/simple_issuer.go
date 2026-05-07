package token

import "time"

type Issuer interface {
	Issue(subject string, expireAt time.Time) string
}

type SimpleIssuer struct{}

func NewSimpleIssuer() *SimpleIssuer {
	return &SimpleIssuer{}
}

func (s *SimpleIssuer) Issue(subject string, expireAt time.Time) string {
	return "dev-token-" + subject
}
