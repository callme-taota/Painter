package utils

import (
	"os"
	"painter-server-new/tolog"
)

func GetProjectDirRoot() (string, error) {
	rootDir, err := os.Getwd()
	if err != nil {
		tolog.Log().Errorf("Error while GetProjectDirRoot %e", err).PrintAndWriteSafe()
		return "", err
	}
	return rootDir, nil
}
