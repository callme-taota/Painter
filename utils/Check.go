package utils

import "regexp"

func IsValidEmail(email string) bool {
	regex := `^[A-Za-z0-9]+([_\.][A-Za-z0-9]+)*@([A-Za-z0-9\-]+\.)+[A-Za-z]{2,6}$`
	match, _ := regexp.MatchString(regex, email)
	return match
}
