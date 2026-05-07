package domain

import (
	"errors"
	"time"
)

var ErrNamespaceRequired = errors.New("namespace required")

type ConfigItem struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	ValueType string    `json:"valueType"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Query struct {
	Namespace string
}

func (q Query) Validate() error {
	if q.Namespace == "" {
		return ErrNamespaceRequired
	}
	return nil
}
