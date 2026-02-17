package services

import "encoding/json"

func mustMarshalJSON(value interface{}) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "null"
	}

	return string(encoded)
}
