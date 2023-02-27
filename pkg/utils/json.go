package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func PrettyString(data interface{}) (string, error) {
	str := fmt.Sprintf("%s", data)
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(str), "", "    "); err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
}

func PrintJSON(data interface{}) error {
	str, err := PrettyString(data)
	if err != nil {
		return err
	}
	fmt.Println(str)
	return nil
}
