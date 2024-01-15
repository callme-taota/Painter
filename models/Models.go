package models

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

type ReturnData struct {
	Msg  string                 `json:"msg"`
	OK   bool                   `json:"ok"`
	Data map[string]interface{} `json:"data"`
}

// RDC Return data container
type RDC map[string]interface{}

func R(msg string, ok bool, data map[string]interface{}) *ReturnData {
	return &ReturnData{msg, ok, data}
}
