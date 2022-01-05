package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

const HOST = "https://pcloud-fe.vercel.app"

func GetClusterInfoUrl(authKey string, clusterID string) string {
	return fmt.Sprintf("%s/api/cluster?authKey=%s&clusterId=%s", HOST, authKey, clusterID)
}

func GetRegisterTokenUrl(token string) string {
	return fmt.Sprintf("%s/register?register_token=%s", HOST, token)
}

func GetRegisterToken(authKey string) (string, error) {
	jsonStr := []byte(fmt.Sprintf(`{"authKey":"%s"}`, authKey))
	url := fmt.Sprintf("%s/api/register-token", HOST)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var m map[string]interface{}
	err = json.Unmarshal(body, &m)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", m["registerToken"]), nil
}