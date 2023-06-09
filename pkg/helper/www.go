package helper

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
)

type Payload struct {
	Errors []ErrorPayload `json:"errors"`
}

type Detail struct {
	Type   string `json:"Type"`
	Name   string `json:"Name"`
	Action string `json:"Action"`
}

func NewDetail(dType, dName, dAction string) Detail {
	if dName == "" {
		dName = "unknown"
	}
	switch dAction {
	case "HEAD", "GET":
		dAction = "pull"
	case "PUT", "POST", "PATCH":
		dAction = "pull"
	case "DELETE":
		dAction = "del"
	}
	return Detail{
		Type:   dType,
		Name:   dName,
		Action: dAction,
	}
}

type ErrorPayload struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []Detail `json:"detail"`
}

func NewErrPayload(code, msg string, det Detail) ErrorPayload {
	return ErrorPayload{
		Code:    code,
		Message: msg,
		Details: []Detail{det},
	}
}

func (p Payload) ToBytes() []byte {
	b, err := json.Marshal(p)
	if err != nil {
		logrus.Fatal(err.Error())
		return nil
	}
	return b
}

func WriteOCIErrror(w http.ResponseWriter, statusCode int, errPl ...ErrorPayload) {
	w.WriteHeader(statusCode)

	payload := Payload{
		Errors: errPl,
	}
	b := payload.ToBytes()
	w.Header().Add("Content-Type", "application/json")
	w.Header().Add("Content-Length", fmt.Sprint(len(b)))
	w.Write(b)

}

//[{"Type":"repository","Name":"samalba/my-app","Action":"pull"}

func WriteResponse(w http.ResponseWriter, statusCode int, b []byte) {
	w.Header().Add("Content-Type", "application/json")
	w.Header().Add("Content-Length", fmt.Sprint(len(b)))
	w.Write(b)
	w.WriteHeader(statusCode)

}
