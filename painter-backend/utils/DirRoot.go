package utils

import (
	"os"

	"github.com/callme-taota/tolog"
)

func GetProjectDirRoot() (string, error) {
	rootDir, err := os.Getwd()
	if err != nil {
		tolog.Errorf("Error while GetProjectDirRoot %e", err).PrintAndWriteSafe()
		return "", err
	}
	return rootDir, nil
}
