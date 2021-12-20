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

package command

import (
	perrs "github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newCloudCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cloud <cluster-name> <operation>",
		Short: "backup data to cloud for PiTR/restore backup data from cloud",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}

			clusterName := args[0]
			clusterReport.ID = scrubClusterName(clusterName)
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			exist, err := tidbSpec.Exist(clusterName)
			if err != nil {
				return err
			}

			if !exist {
				return perrs.Errorf("Cluster %s not found", clusterName)
			}

			operation := args[1]
			switch operation {
			case "backup": return cm.Backup2Cloud(clusterName, gOpt)
			case "restore": return cm.RestoreFromCloud(clusterName, gOpt)
			default:
				return perrs.Errorf("Cloud cmd not support", operation)
			}
		},
	}
	return cmd
}
