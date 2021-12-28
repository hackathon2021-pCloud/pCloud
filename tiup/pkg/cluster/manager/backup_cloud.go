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
	"fmt"
	"os/exec"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/backup"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	mockS3 = "s3://tmp/br-restore/%s?access-key=minioadmin&secret-access-key=minioadmin&endpoint=http://minio.pingcap.net:9000&force-path-style=true"
)

func (m *Manager) DoBackup(pdAddr string, metadata spec.Metadata, us string) error {
	env := environment.GlobalEnv()

	ver, err := env.DownloadComponentIfMissing("br", utils.Version(metadata.GetBaseMeta().Version))
	if err != nil {
		return err
	}
	fmt.Printf("using BR version %s\n", ver)
	br, err := env.BinaryPath("br", ver)
	if err != nil {
		return err
	}

	builder := backup.NewBackup(pdAddr)
	s := fmt.Sprintf(mockS3, us)
	builder.Storage(s)
	b := backup.BR{Path: br, Version: ver}
	return b.Execute(context.TODO(), *builder...)
}

func (m *Manager) StartsIncrementalBackup(pdAddr string, metadata spec.Metadata, us string) error {
	env := environment.GlobalEnv()
	ver, err := env.DownloadComponentIfMissing("ctl", utils.Version(metadata.GetBaseMeta().Version))
	if err != nil {
		return err
	}
	cdcCtl, err := env.BinaryPath("cdc", ver)
	if err != nil {
		return err
	}
	c := backup.CdcCtl{Path: cdcCtl, Version: ver}
	builder := backup.GetInrementalBackup(us, pdAddr)
	out, err := c.Execute(context.TODO(), *builder...)
	if err != nil && !strings.Contains(string(out), "ErrChangeFeedNotExists") {
		return errors.Annotate(err, "run getChangeFeed failed and error not expected")
	}
	if err == nil {
		// changefeed exists in cdc
		return errors.New("backup to cloud is enabled already")
	}
	builder = backup.NewIncrementalBackup(us, pdAddr)
	s := fmt.Sprintf(mockS3, us)
	builder.Storage(s)
	out, err = c.Execute(context.TODO(), *builder...)
	fmt.Println("out", string(out))
	if err != nil {
		return err
	}
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
	// TODO get uuid from service
	uuid, _ := uuid.NewUUID()
	us := uuid.String()
	fmt.Println("unique string", us)

	if err := m.DoBackup(pdHost, metadata, us); err != nil {
		return err
	}
	return m.StartsIncrementalBackup(pdHost, metadata, us)
}

// RestoreFromCloud start a full backup and log backup from cloud.
func (m *Manager) RestoreFromCloud(name string, opt operator.Options) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}
	// 1. start interact with services.
	// 2. use br to do a full restore.
	// 3. use br to do a cdc log restore.
	// 4. tell user restore finished and the costs.
	return nil
}

func run(name string, args ...string) *exec.Cmd {
	// Handle `cdc cli`
	if strings.Contains(name, " ") {
		xs := strings.Split(name, " ")
		name = xs[0]
		args = append(xs[1:], args...)
	}
	cmd := exec.Command(name, args...)
	return cmd
}

func binaryPath(home, cmd string) (string, error) {
	switch cmd {
	case "tidb", "tikv", "pd":
		return path.Join(home, cmd+"-ctl"), nil
	case "binlog", "etcd":
		return path.Join(home, cmd+"ctl"), nil
	case "cdc":
		return path.Join(home, cmd+" cli"), nil
	default:
		return "", errors.New("ctl only supports tidb, tikv, pd, binlog, etcd and cdc currently")
	}
}
