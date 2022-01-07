package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/fatih/color"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/utils"
)

const HOST = "https://pcloud-fe.vercel.app"

func GetClusterInfoUrl(authKey string, clusterID string) string {
	return fmt.Sprintf("%s/api/cluster?authKey=%s&clusterId=%s", HOST, authKey, clusterID)
}

func GetRegisterTokenUrl(token string) string {
	return color.BlueString(fmt.Sprintf("%s/register?register_token=%s", HOST, token))
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

type CreateProgressRequest struct {
	ClusterID string `json:"clusterId"`
	AuthKey   string `json:"authKey"`
	Progress  int    `json:"progress"`
	BackupURL string `json:"backupUrl"`
}

func DefaultCli() *utils.HTTPClient {
	return utils.NewHTTPClient(30*time.Second, nil)
}

func API(path string) string {
	return fmt.Sprintf("%s/api/%s", HOST, path)
}

func RunPOST(site string, req, resp interface{}) error {
	cli := DefaultCli()
	j, err := json.Marshal(req)
	if err != nil {
		return err
	}
	br, err := cli.Post(context.TODO(), site, bytes.NewReader(j))
	if err != nil {
		return errors.Annotatef(err, "with request: %s to %s", string(j), site)
	}
	if resp == nil {
		return nil
	}
	return errors.Annotatef(json.Unmarshal(br, resp), "failed to unmarshal json: %s", string(br))
}

func RunGET(site string, resp interface{}) error {
	cli := DefaultCli()
	br, err := cli.Get(context.TODO(), site)
	if err != nil {
		return errors.Annotatef(err, "with request to %s", site)
	}
	if resp == nil {
		return nil
	}
	return errors.Annotatef(json.Unmarshal(br, resp), "failed to unmarshal json: %s", string(br))
}

func CreateProgress(cp CreateProgressRequest) error {
	return RunPOST(API("cluster-setup-progress"), cp, nil)
}

type CreateCheckpointRequest struct {
	AuthKey        string `json:"authKey"`
	ClusterID      string `json:"clusterId"`
	UploadStatus   string `json:"uploadStatus"`
	UploadProgress int    `json:"uploadProgress"`
	CheckpointTime int64  `json:"checkpointTime"`
	URL            string `json:"url"`
	BackupSize     int    `json:"backupSize"`
	Operator       string `json:"operator"`
}

type CreateCheckpointResponse struct {
	ID string `json:"id"`
}

func CreateCheckpoint(cr CreateCheckpointRequest) (string, error) {
	id := CreateCheckpointResponse{}
	if err := RunPOST(API("checkpoint"), cr, &id); err != nil {
		return "", err
	}
	return id.ID, nil
}

type GetCheckpointWrapper struct {
	Result string `json:"result"`
}

type GetCheckpointResponse struct {
	Checkpoint Checkpoint `json:"checkpoint"`
}

type Checkpoint struct {
	ID             string `json:"id"`
	CreateTime     string `json:"createTime"`
	ClusterID      string `json:"clusterId"`
	UploadProgress string `json:"uploadProgress"`
	UploadStatus   string `json:"uploadStatus"`
	URL            string `json:"url"`
	BackupSize     string `json:"backupSize"`
	CheckpointTime int64  `json:"checkpointTime"`
}

func GetCheckpoint(checkpoint string) (*Checkpoint, error) {
	wrap := &GetCheckpointWrapper{}
	if err := RunGET(fmt.Sprintf("%s?token=%s", API("temporary-token"), checkpoint), &wrap); err != nil {
		return nil, errors.Annotatef(err, "failed to get response")
	}
	if wrap.Result == "" {
		return nil, errors.Errorf("The token %s is expired or invalid", checkpoint)
	}
	resp := GetCheckpointResponse{}
	if err := json.Unmarshal([]byte(wrap.Result), &resp); err != nil {
		return nil, errors.Annotatef(err, "failed to parse the inner response (result = %s)", wrap.Result)
	}
	return &resp.Checkpoint, nil
}

type ClusterInfo struct {
	Cluster struct {
		SetupStatus        string      `json:"setupStatus"`
		ID                 string      `json:"id"`
		CreateTime         string      `json:"createTime"`
		StorageProvider    string      `json:"storageProvider"`
		Name               string      `json:"name"`
		LaskCheckpointTime interface{} `json:"laskCheckpointTime"`
		BackupSize         interface{} `json:"backupSize"`
	} `json:"cluster"`
}

func GetCluster(clusterID string, authKey string) (ClusterInfo, error) {
	clusterInfo := ClusterInfo{}
	if err := RunGET(fmt.Sprintf("%s?authKey=%s&clusterId=%s", API("cluster"), url.QueryEscape(authKey), url.QueryEscape(clusterID)), &clusterInfo); err != nil {
		return ClusterInfo{}, err
	}
	return clusterInfo, nil
}
