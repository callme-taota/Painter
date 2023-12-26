package utils

import (
	"backendQucikStart/tolog"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// JSONReader reads and parses JSON content from a file.
func JSONReader(filePath string) (map[string]interface{}, error) {
	// Get the current working directory.
	rootDir, err := os.Getwd()
	if err != nil {
		tolog.Log().Errorf("jsonReader %e", err).PrintAndWriteSafe()
		return nil, err
	}

	// Concatenate the file path.
	absPath := filepath.Join(rootDir, filePath)

	// Log the read JSON file path.
	tolog.Log().Infof("read json file:%s", absPath).PrintLog()

	// Read the file content.
	fileContent, err := os.ReadFile(absPath)
	if err != nil {
		tolog.Log().Errorf("jsonReader %e", err).PrintAndWriteSafe()
		return nil, err
	}

	// Parse JSON.
	var jsonData map[string]interface{}
	err = json.Unmarshal(fileContent, &jsonData)
	if err != nil {
		tolog.Log().Errorf("jsonReader %e", err).PrintAndWriteSafe()
		return nil, err
	}

	return jsonData, nil
}

// JSONConvertToMapString converts a map with arbitrary types to a map with string values.
func JSONConvertToMapString(originalMap interface{}) map[string]string {
	convertedMap := make(map[string]string)

	// Iterate over the original map.
	for key, value := range originalMap.(map[string]interface{}) {
		// Use type assertion to check the value's type.
		switch v := value.(type) {
		case int:
			// Convert int to string.
			convertedMap[key] = fmt.Sprintf("%d", v)
		case float64:
			// Convert float64 to string.
			convertedMap[key] = fmt.Sprintf("%f", v)
		case string:
			// Copy string type directly.
			convertedMap[key] = v
		default:
			// Handle other types as needed.
			// Additional type conversion rules can be added here.
			tolog.Log().Warningf("Unsupported type for key %s", key).PrintAndWriteSafe()
		}
	}

	return convertedMap
}
