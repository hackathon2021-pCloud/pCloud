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
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/localdata"
)

const (
	createChangeFeedCMD = "changefeed create --pd=%s --sink-uri=\"%s\" --changefeed-id=\"%s\""
	getChangeFeedCMD    = "changefeed query --pd=%s --changefeed-id=\"%s\""
)

// Backup2Cloud start full backup and log backup to cloud.
func (m *Manager) Backup2Cloud(name string, opt operator.Options) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	home := os.Getenv(localdata.EnvNameComponentInstallDir)
	if home == "" {
		return errors.New("component `ctl` cannot run in standalone mode")
	}
	cdcCtl, err := binaryPath(home, "cdc")
	if err != nil {
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
	if !cdcExists {
		return errors.New("cluster doesn't have any cdc server")
	}
	if len(pdHost) == 0 {
		return errors.New("cluster doesn't find pd server")
	}
	// TODO get uuid from service
	uuid, _ := uuid.NewUUID()
	c := run(cdcCtl, strings.Split(fmt.Sprintf(getChangeFeedCMD, pdHost, uuid), " ")...)
	out, err := c.Output()
	if err != nil && !strings.Contains(string(out), "ErrChangeFeedNotExists") {
		return errors.Annotate(err, "run getChangeFeed failed and error not expected")
	}
	if err == nil {
		// changefeed exists in cdc
		return errors.New("backup to cloud is enabled already")
	}
	// TODO get s3 info from service
	c = run(cdcCtl, strings.Split(fmt.Sprintf(createChangeFeedCMD, pdHost, "s3://tmp/br-restore/restore_test1?access-key=minioadmin&secret-access-key=minioadmin&endpoint=http%3a%2f%2fminio.pingcap.net%3a9000&force-path-style=true", uuid), " ")...)
	err = c.Run()
	if err != nil {
		return err
	}
	return nil
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
