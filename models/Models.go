package models

import (
	"reflect"
)

const (
	KReturnMsgOK      = "ok"
	KReturnMsgSuccess = "success"
	KReturnMsgError   = "error"

	KErrorInvalid        = "post data invalid"
	KErrorMissing        = "post data missing"
	KErrorTypeError      = "post data type error"
	KErrorNotFound       = "not found"
	KErrorNoUser         = "user not found"
	KErrorSessionInvalid = "user session invalid"
	KErrorPassword       = "password error"
)

const (
	KReturnFalse = false
	KReturnTrue  = true
)

type RDContainer interface {
	IsOk() bool
	Message() string
}

type ReturnData struct {
	Msg  string                 `json:"msg"`
	OK   bool                   `json:"ok"`
	Data map[string]interface{} `json:"data"`
}

func (rd *ReturnData) IsOk() bool {
	return rd.OK
}

func (rd *ReturnData) Message() string {
	return rd.Msg
}

type ReturnDatas struct {
	Msg  string `json:"msg"`
	OK   bool   `json:"ok"`
	Data any    `json:"data"`
}

func (rd *ReturnDatas) IsOk() bool {
	return rd.OK
}

func (rd *ReturnDatas) Message() string {
	return rd.Msg
}

type ReturnDatal struct {
	Msg  string        `json:"msg"`
	OK   bool          `json:"ok"`
	Data []interface{} `json:"data"`
}

func (rd *ReturnDatal) IsOk() bool {
	return rd.OK
}

func (rd *ReturnDatal) Message() string {
	return rd.Msg
}

// RDC Return data container
type RDC map[string]interface{}

func R(msg string, ok bool, data map[string]interface{}) *ReturnData {
	return &ReturnData{msg, ok, data}
}

func Rs(msg string, ok bool, data any) *ReturnDatas {
	return &ReturnDatas{msg, ok, data}
}

func Rl(msg string, ok bool, data []interface{}) *ReturnDatal {
	return &ReturnDatal{msg, ok, data}
}

func ShouldCheckJSON(obj interface{}, checking []string) bool {
	// Use reflection to retrieve information about the struct fields
	objValue := reflect.ValueOf(obj)
	if objValue.Kind() == reflect.Ptr {
		objValue = objValue.Elem()
	}

	// Ensure that the input is a struct
	if objValue.Kind() != reflect.Struct {
		return false
	}

	// Iterate through the struct fields and check if they exist in checking
	for _, field := range checking {
		// Get the field
		fieldValue := objValue.FieldByName(field)

		// Check if the field exists
		if !fieldValue.IsValid() || fieldValue.String() == "" {
			return false
		}
	}

	return true
}
