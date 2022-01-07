// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package manager

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"go.uber.org/multierr"

	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/tui"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/backup"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	mockS3         = "s3://tmp/br-restore/%s/%s?access-key=minioadmin&secret-access-key=minioadmin&endpoint=http://minio.pingcap.net:9000&force-path-style=true"
	localMinio     = "s3://brie/%s/%s?endpoint=http://192.168.56.102:9000"
	s3AddressNoArg = "s3://pcloud2021/backups/%s/%s"
	s3Address      = "s3://pcloud2021/backups/%s/%s?access-key=%s&secret-access-key=%s&force-path-style=true&region=us-west-2"
	cloudDir       = "/tmp/cloud"
)

func (m *Manager) RunCheckpointDaemon(info *ClusterInfo) error {
	clusterID, err := m.GetPCloudClusterID(info.Name)
	if err != nil {
		return err
	}
	cmd := exec.Command("bin/checkpoint-daemon",
		"--cluster-id",
		clusterID,
		"--auth-key",
		authKeyForCluster(info.Name),
		"--url",
		fmt.Sprintf(s3AddressNoArg, clusterID, "incr"),
	)
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Process.Release(); err != nil {
		return err
	}
	return nil
}

func (m *Manager) getAccessKey() string {
	return os.Getenv("ACCESS_KEY")
}

func (m *Manager) getSecretAccessKey() string {
	return os.Getenv("SECRET_ACCESS_KEY")
}

func (m *Manager) getS3Address(s, t string) string {
	return fmt.Sprintf(s3Address, s, t, m.getAccessKey(), m.getSecretAccessKey())
}

func (m *Manager) DoBackup(info ClusterInfo, us string) error {
	env := environment.GlobalEnv()

	ver, err := env.DownloadComponentIfMissing("br", utils.Version(info.Meta.GetBaseMeta().Version))
	if err != nil {
		return err
	}
	br, err := env.BinaryPath("br", ver)
	if err != nil {
		return err
	}

	builder := backup.NewBackup(info.PDAddr[0])
	backupURL := m.getS3Address(us, "full")
	builder.Storage(backupURL)
	b := backup.BR{Path: br, Version: ver}
	cmd := b.CreateCmd(context.TODO(), *builder...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	fmt.Println("Started BR with PID:", color.GreenString("%d", cmd.Process.Pid))
	if err := cmd.Process.Release(); err != nil {
		return errors.New("failed to release BR")
	}
	return multierr.Append(backup.StartTracerProcess(out, "bin/br-progtracer", us, authKeyForCluster(info.Name), backupURL),
		m.RunCheckpointDaemon(&info))
}

func (m *Manager) DoRestore(pdAddr string, metadata spec.Metadata, us string, toTS uint) error {
	env := environment.GlobalEnv()

	ver, err := env.DownloadComponentIfMissing("br", utils.Version(metadata.GetBaseMeta().Version))
	if err != nil {
		return err
	}
	br, err := env.BinaryPath("br", ver)
	if err != nil {
		return err
	}
	// Do full restore
	builder := backup.NewRestore(pdAddr)
	builder.Storage(m.getS3Address(us, "full"))
	b := backup.BR{Path: br, Version: ver}
	fmt.Println(color.GreenString("start downloading..."))
	cmd := b.Execute(context.TODO(), *builder...)
	cmd.Trace.OnProgress(func(progress backup.Progress) {
		fmt.Println(color.HiGreenString("%+v", progress))
	})

	if err := cmd.Handle.Wait(); err != nil {
		return err
	}
	// Do log restore
	builder = backup.NewLogRestore(pdAddr)
	builder.Storage(m.getS3Address(us, "inc"))
	builder.TimeRange(0, toTS)
	b = backup.BR{Path: br, Version: ver}
	fmt.Println(color.GreenString("start incremental downloading..."))
	return b.Execute(context.TODO(), *builder...).Handle.Wait()
}

func (m *Manager) GetCDC(metadata spec.Metadata) (*backup.CdcCtl, error) {
	env := environment.GlobalEnv()
	ver, err := env.DownloadComponentIfMissing("ctl", utils.Version(metadata.GetBaseMeta().Version))
	if err != nil {
		return nil, err
	}
	ctl, err := env.BinaryPath("ctl", ver)
	if err != nil {
		return nil, err
	}
	cdcCtl := path.Join(filepath.Dir(ctl), "cdc")
	c := &backup.CdcCtl{Path: cdcCtl, Version: ver}
	return c, nil
}

func (m *Manager) StartsIncrementalBackup(pdAddr string, metadata spec.Metadata, us string) error {
	c, err := m.GetCDC(metadata)
	if err != nil {
		return err
	}
	builder := backup.GetIncrementalBackup(us, pdAddr)
	out, err := c.Execute(context.TODO(), *builder...)
	if err != nil && !strings.Contains(string(out), "ErrChangeFeedNotExists") {
		return errors.Annotate(err, "run getChangeFeed failed and error not expected")
	}
	if err == nil {
		// changefeed exists in cdc
		return errors.New("backup to cloud is enabled already")
	}
	c.PipeYes = true
	builder = backup.NewIncrementalBackup(us, pdAddr)
	builder.Storage(m.getS3Address(us, "inc"))
	out, err = c.Execute(context.TODO(), *builder...)
	if err != nil {
		return err
	}
	return nil
}

func authKeyForCluster(name string) string {
	hasher := sha1.New()
	hasher.Write([]byte(name))
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	return sha
}

func (m *Manager) GetPCloudClusterID(name string) (string, error) {
	clusterFile := path.Join(cloudDir, authKeyForCluster(name), "cloudFile")
	clusterID, err := os.ReadFile(clusterFile)
	if os.IsNotExist(err) {
		return "", errors.Errorf("the cluster %s hasn't been registered to pCloud")
	}
	if err != nil {
		return "", err
	}
	return string(clusterID), nil
}

// TODO: interface for making checkpoint
// TODO: restore from checkpoint
func (m *Manager) SetCheckpoint(name string, skipConfirm bool) error {
	info := m.getClusterInfo(name)
	if err := multierr.Append(info.AssertCDCExists(), info.AssertPDExists()); err != nil {
		return err
	}
	clusterID, err := m.GetPCloudClusterID(name)
	if err != nil {
		return err
	}
	cf := backup.GetIncrementalBackup(string(clusterID), info.PDAddr[0])
	cdc, err := m.GetCDC(info.Meta)
	out, err := cdc.Execute(context.TODO(), cf.Build()...)
	if err != nil {
		return err
	}

	type Stat struct {
		Checkpoint uint64 `json:"checkpoint-ts"`
	}
	type ChangeFeed struct {
		Status Stat `json:"status"`
	}
	cfs := ChangeFeed{}
	if err := json.Unmarshal(out, &cfs); err != nil {
		return err
	}
	fmt.Println("Your current checkpoint ts is:", color.HiBlueString("%d", cfs.Status.Checkpoint))
	phyTime := cfs.Status.Checkpoint >> 18
	t := time.UnixMilli(int64(phyTime))
	fmt.Println("The logic time is:", color.HiBlueString("%s", t))
	if !skipConfirm {
		ok, _ := tui.PromptForConfirmYes("Create the checkpoint? ")
		if !ok {
			return nil
		}
	}
	usr, err := user.Current()
	userName := usr.Username
	if err != nil {
		userName = "UNKNOWN"
	}
	cp, err := api.CreateCheckpoint(api.CreateCheckpointRequest{
		AuthKey:        authKeyForCluster(info.Name),
		ClusterID:      string(clusterID),
		UploadStatus:   "finish",
		UploadProgress: 100,
		CheckpointTime: int64(phyTime),
		URL:            "s3://tbd",
		BackupSize:     42,
		Operator:       userName,
	})
	if err != nil {
		return errors.Annotatef(err, "failed to create checkpoint")
	}
	fmt.Println("Your checkpoint has been created with ID:", color.HiBlackString("%s", cp))
	fmt.Println("Check it at:", color.GreenString("%s/cluster?id=%s", api.HOST, clusterID))
	return nil
}

type ClusterInfo struct {
	Name      string
	PDAddr    []string
	CDCExists bool
	Meta      spec.Metadata
}

func (c *ClusterInfo) AssertPDExists() error {
	if len(c.PDAddr) == 0 {
		return errors.New("cluster doesn't find pd server")
	}
	return nil
}

func (c *ClusterInfo) AssertCDCExists() error {
	if !c.CDCExists {
		return errors.New("cluster doesn't find cdc server")
	}
	return nil
}

func (m *Manager) getClusterInfo(cluster string) ClusterInfo {
	metadata, _ := m.meta(cluster)
	topo := metadata.GetTopology()

	info := ClusterInfo{Meta: metadata, Name: cluster}
	topo.IterInstance(func(instance spec.Instance) {
		if instance.Role() == "cdc" {
			info.CDCExists = true
		}
		if instance.Role() == "pd" {
			info.PDAddr = append(info.PDAddr, fmt.Sprintf("http://%s:%d", instance.GetHost(), instance.GetPort()))
		}
	})
	return info
}

// Backup2Cloud start full backup and log backup to cloud.
func (m *Manager) Backup2Cloud(name string, opt operator.Options) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	info := m.getClusterInfo(name)
	if err := multierr.Append(info.AssertCDCExists(), info.AssertPDExists()); err != nil {
		return err
	}
	// authKey is the validation code for one cluster.
	// the same cluster has the same authKey.
	hasher := sha1.New()
	hasher.Write([]byte(name))
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	authKey := sha
	authDir := filepath.Join(cloudDir, authKey)
	tokenFile := filepath.Join(authDir, "tokenFile")
	var (
		err       error
		token     string
		clusterID string
	)
	if _, err = os.Stat(tokenFile); os.IsNotExist(err) {
		// try get token from service
		err = os.MkdirAll(authDir, 0755)
		if err != nil {
			return err
		}

		token, err = api.GetRegisterToken(authKey)
		if err != nil {
			return err
		}
		err = m.SaveToFile(tokenFile, token)
		if err != nil {
			return err
		}
	}
	token, err = m.GetFromFile(tokenFile)
	// cannot get token from file
	if err != nil {
		return err
	}
	clusterFile := filepath.Join(authDir, "cloudFile")
	// try get cluster from file
	if _, err = os.Stat(clusterFile); os.IsNotExist(err) {
		fmt.Println("please login pCloud service(" + api.GetRegisterTokenUrl(token) + ") and paste unique token")
		fmt.Print("unique token: ")
		fmt.Scanf("%s", &clusterID)
		if len(clusterID) == 0 {
			return errors.New("input unique token is invalid")
		}
		err = m.SaveToFile(clusterFile, clusterID)
		if err != nil {
			return err
		}
		fmt.Println(color.GreenString("Starting streaming.."))
		err = m.StartsIncrementalBackup(info.PDAddr[0], info.Meta, clusterID)
		if err != nil {
			return err
		}

		fmt.Println(color.GreenString("Starting upload.."))
		err = m.DoBackup(info, clusterID)
		if err != nil {
			return err
		}

		fmt.Println("pitr to cloud enabled! you can check the progress in ", color.BlueString(api.HOST))
	} else {
		clusterID, err = m.GetFromFile(clusterFile)
		if err != nil {
			return err
		}
		fmt.Println("this cluster(ID:"+color.YellowString(clusterID)+") has enable pitr before! please check in ", color.BlueString(api.HOST))
	}
	return nil
}

func (m *Manager) GetFromFile(file string) (string, error) {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (m *Manager) SaveToFile(file string, content string) error {
	err := ioutil.WriteFile(file, []byte(content), 0644)
	if err != nil {
		return err
	}
	return nil
}

// RestoreFromCloud start a full backup and log backup from cloud.
func (m *Manager) RestoreFromCloud(name string, predefined string) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	// 1. start interact with services.
	// 2. use br to do a full restore.
	// 3. use br to do a cdc log restore.
	// 4. tell user restore finished and the costs.
	info := m.getClusterInfo(name)
	if err := info.AssertPDExists(); err != nil {
		return err
	}
	clusterID, err := m.GetPCloudClusterID(name)
	if err != nil {
		return err
	}
	fmt.Println("Hint: you can generate a checkpoint from", color.YellowString("%s/cluster?id=%s", api.HOST, clusterID))
	token := tui.Prompt("Please input the checkpoint token generated:")

	// TODO use the token to restore to the checkpoint.
	cp, err := api.GetCheckpoint(token)
	if err != nil {
		return err
	}
	// TODO hint the cluster name here.
	fmt.Println("The checkpoint is at", color.BlueString("%s", time.UnixMilli(cp.CheckpointTime)))
	ok, _ := tui.PromptForConfirmYes("Continue? ")
	if !ok {
		return nil
	}
	return m.DoRestore(info.PDAddr[0], info.Meta, cp.ClusterID, uint(cp.CheckpointTime))
}
