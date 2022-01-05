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
	"fmt"
	"github.com/fatih/color"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pingcap/tiup/pkg/cluster/api"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/backup"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	mockS3      = "s3://tmp/br-restore/%s/%s?access-key=minioadmin&secret-access-key=minioadmin&endpoint=http://minio.pingcap.net:9000&force-path-style=true"
	s3Address   = "s3://pcloud2021/backups/%s/%s?access-key=%s&secret-access-key=%s&force-path-style=true&region=us-west-2"
	cloudDir    = "/tmp/cloud"
)

func (m *Manager) getAccessKey() string {
	return os.Getenv("ACCESS_KEY")
}

func (m *Manager) getSecretAccessKey() string {
	return os.Getenv("SECRET_ACCESS_KEY")
}

func (m *Manager) getS3Address(s, t string) string {
	return fmt.Sprintf(s3Address, s, t, m.getAccessKey(), m.getSecretAccessKey())
}

func (m *Manager) DoBackup(pdAddr string, metadata spec.Metadata, us string) error {
	env := environment.GlobalEnv()

	ver, err := env.DownloadComponentIfMissing("br", utils.Version(metadata.GetBaseMeta().Version))
	if err != nil {
		return err
	}
	br, err := env.BinaryPath("br", ver)
	if err != nil {
		return err
	}

	builder := backup.NewBackup(pdAddr)
	builder.Storage(m.getS3Address(us, "full"))
	b := backup.BR{Path: br, Version: ver}
	return b.Execute(context.TODO(), *builder...)
}

func (m *Manager) DoRestore(pdAddr string, metadata spec.Metadata, us string) error {
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
	err = b.Execute(context.TODO(), *builder...)
	if err != nil {
		return err
	}
	// Do log restore
	builder = backup.NewLogRestore(pdAddr)
	builder.Storage(m.getS3Address(us, "inc"))
	b = backup.BR{Path: br, Version: ver}
	return b.Execute(context.TODO(), *builder...)
}

func (m *Manager) StartsIncrementalBackup(pdAddr string, metadata spec.Metadata, us string) error {
	env := environment.GlobalEnv()
	ver, err := env.DownloadComponentIfMissing("ctl", utils.Version(metadata.GetBaseMeta().Version))
	if err != nil {
		return err
	}
	ctl, err := env.BinaryPath("ctl", ver)
	if err != nil {
		return err
	}
	cdcCtl := path.Join(filepath.Dir(ctl), "cdc")
	c := backup.CdcCtl{Path: cdcCtl, Version: ver}
	builder := backup.GetIncrementalBackup(us, pdAddr)
	out, err := c.Execute(context.TODO(), *builder...)
	if err != nil && !strings.Contains(string(out), "ErrChangeFeedNotExists") {
		return errors.Annotate(err, "run getChangeFeed failed and error not expected")
	}
	if err == nil {
		// changefeed exists in cdc
		return errors.New("backup to cloud is enabled already")
	}
	builder = backup.NewIncrementalBackup(us, pdAddr)
	builder.Storage(m.getS3Address(us, "inc"))
	out, err = c.Execute(context.TODO(), *builder...)
	if err != nil {
		return err
	}
	fmt.Println("out:", out)
	return nil
}

// Backup2Cloud start full backup and log backup to cloud.
func (m *Manager) Backup2Cloud(name string, opt operator.Options) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	metadata, _ := m.meta(name)
	topo := metadata.GetTopology()
	// 1. start interact with services.
	// 2. use br to do a full backup.
	// 3. use cdc ctl/api to create a changefeed to s3.
	// 4. tell user backup finished

	var cdcExists bool
	var pdHost string
	topo.IterInstance(func(instance spec.Instance) {
		if instance.Role() == "cdc" {
			cdcExists = true
		}
		if instance.Role() == "pd" {
			pdHost = fmt.Sprintf("http://%s:%d", instance.GetHost(), instance.GetPort())
		}
	})
	if len(pdHost) == 0 {
		return errors.New("cluster doesn't find pd server")
	}
	if !cdcExists {
		return errors.New("cluster doesn't have any cdc server")
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
		err error
		token string
	    clusterID string
	)
	if _, err = os.Stat(tokenFile); os.IsNotExist(err) {
		// try get token from service
		err = os.MkdirAll(authDir, 0644)
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
		fmt.Println(color.GreenString("Starting upload.."))
		err = m.DoBackup(pdHost, metadata, clusterID)
		if err != nil {
			return err
		}
		err = m.StartsIncrementalBackup(pdHost, metadata, clusterID)
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
func (m *Manager) RestoreFromCloud(name string, us string, opt operator.Options) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	metadata, _ := m.meta(name)
	topo := metadata.GetTopology()
	// 1. start interact with services.
	// 2. use br to do a full restore.
	// 3. use br to do a cdc log restore.
	// 4. tell user restore finished and the costs.
	var pdHost string
	topo.IterInstance(func(instance spec.Instance) {
		if instance.Role() == "pd" {
			pdHost = fmt.Sprintf("http://%s:%d", instance.GetHost(), instance.GetPort())
		}
	})
	if len(pdHost) == 0 {
		return errors.New("cluster doesn't have any cdc server")
	}
	return m.DoRestore(pdHost, metadata, us)
}
