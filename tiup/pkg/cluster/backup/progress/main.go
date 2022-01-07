package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/fatih/color"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/backup"
	"github.com/spf13/pflag"
)

var (
	cluster   = pflag.String("cluster-id", "", "the cluster for updating")
	authKey   = pflag.String("auth-key", "", "the authkey of your account")
	backupURL = pflag.String("url", "", "the backup url")
)

func main() {
	pflag.Parse()
	trace := backup.TraceByLog(os.Stdin)
	endro := make(chan struct{})
	closeOnce := new(sync.Once)
	trace.OnProgress(func(progress backup.Progress) {
		if progress.Precent >= 1 {
			closeOnce.Do(func() {
				close(endro)
				trace.Stop()
			})
		}
		if err := api.CreateProgress(api.CreateProgressRequest{
			ClusterID: *cluster,
			AuthKey:   *authKey,
			Progress:  int(progress.Precent * 100),
			BackupURL: *backupURL,
		}); err != nil {
			fmt.Println("failed to upload progress", color.RedString("%s", err))
		}
	})
	trace.Init()
	<-endro
}
