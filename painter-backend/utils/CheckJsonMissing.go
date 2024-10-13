package utils

func CheckJsonMissing(map[string]interface{}) {

}

func StringIsEmpty(s string) bool {
	return s == ""
}

func CheckMissings(s ...string) bool {
	for _, str := range s {
		if str == "" {
			return false
		}
	}
	return true
}
