package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

const HOST = "https://pcloud-fe.vercel.app"

func GetRegisterToken(authKey string) (string, error) {
	req, err := http.Post(fmt.Sprintf("%s/api/register-token", HOST),"application/json", strings.NewReader(fmt.Sprintf(`
	{
		"authKey": "%s"
	}
	`, authKey)))
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return "", err
	}
	var m map[string]interface{}
	err = json.Unmarshal(body, &m)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", m["register-token"]), nil
}
