package app

import "encoding/json"

// Date visibility travels with the saved draft and its immutable publication.
// The last edit timestamp is captured from the saved draft, never live draft data.
type ArticleDateDisplay struct {
	Created   bool `json:"created"`
	Updated   bool `json:"updated"`
	Published bool `json:"published"`
}

func dateDisplayJSON(value *ArticleDateDisplay) []byte {
	if value == nil {
		value = &ArticleDateDisplay{Created: true, Updated: true, Published: true}
	}
	b, _ := json.Marshal(value)
	return b
}

// Old clients may omit the new field. An existing saved choice must survive.
func optionalDateDisplayJSON(value *ArticleDateDisplay) any {
	if value == nil {
		return nil
	}
	return dateDisplayJSON(value)
}
