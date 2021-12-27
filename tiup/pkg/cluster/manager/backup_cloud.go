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
	"errors"
	"fmt"

	"github.com/pingcap/tiup/pkg/cluster/backup"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/utils"
)

func (m *Manager) DoBackup(pdAddr string) error {

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
	if len(pdAddr) == 0 {
		return errors.New("failed to find PD node")
	}
	builder := backup.NewBackup(pdAddr)
	builder.Storage("s3://brie/br2")
	*builder = append(*builder, "--s3.endpoint", "http://192.168.56.102:9000")
	b := backup.BR{Path: br, Version: ver}
	return b.Execute(context.TODO(), *builder...)
}

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
	var pdAddr string

	topo.IterInstance(func(instance spec.Instance) {
		if instance.Role() == "pd" {
			// TODO handle this when TLS enabled.
			pdAddr = fmt.Sprintf("http://%s:%d",
				instance.GetHost(),
				instance.GetPort())
		}
	})

	return m.DoBackup(pdAddr)
}

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
