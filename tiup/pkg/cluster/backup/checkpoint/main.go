package main

import (
	"context"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/spf13/pflag"
)

var (
	cluster            = pflag.String("cluster-id", "", "the cluster for updating")
	authKey            = pflag.String("auth-key", "", "the authkey of your account")
	checkpointInterval = pflag.Duration("checkpoint-interval", 60*time.Second, "the interval of creating checkpoints")
	url                = pflag.String("url", "s3://pcloud2021/backups", "the url")
)

func run(ctx context.Context, timer <-chan time.Time) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer:
			clusterInfo, err := api.GetCluster(*cluster, *authKey)
			if err != nil {
				return err
			}
			if clusterInfo.Cluster.SetupStatus != "finish" {
				continue
			}
			cp, err := api.CreateCheckpoint(api.CreateCheckpointRequest{
				AuthKey:        *authKey,
				ClusterID:      *cluster,
				UploadStatus:   "finish",
				UploadProgress: 100,
				CheckpointTime: time.Now().UnixMilli(),
				URL:            *url,
				// How can we calcute the backup size?
				// Maybe we must inject the logic into CDC?
				// (Maybe it is a little dirty to let CDC known about the "cloud" API?)
				BackupSize: 42,
				Operator:   "pingcap",
			})
			if err != nil {
				return err
			}
			fmt.Println(color.GreenString("Checkpoint %s created.", cp))
		}
	}
}

func main() {
	pflag.Parse()
	tick := time.NewTicker(*checkpointInterval)
	if err := run(context.Background(), tick.C); err != nil {
		panic(err)
	}
}
