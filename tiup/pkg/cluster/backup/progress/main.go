package main

import (
	"fmt"
	"os"
	"os/signal"
	"path"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/backup"
	"github.com/spf13/pflag"
)

var (
	cluster   = pflag.String("cluster-id", "", "the cluster for updating")
	authKey   = pflag.String("auth-key", "", "the authkey of your account")
	backupURL = pflag.String("url", "", "the backup url")
	logFile   = pflag.String("log-file", path.Join(os.TempDir(), time.Now().Format("2006-01-02@15:04:05")), "the log file")
)

func main() {
	pflag.Parse()
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch)
		for c := range ch {
			if c == syscall.SIGTERM {
				os.Exit(0)
			}
		}
	}()
	trace := backup.TraceByLog(os.Stdin)
	endro := make(chan struct{})
	closeOnce := new(sync.Once)
	trace.OnProgress(func(progress backup.Progress) {
		if err := api.CreateProgress(api.CreateProgressRequest{
			ClusterID: *cluster,
			AuthKey:   *authKey,
			Progress:  int(progress.Precent * 100),
			BackupURL: *backupURL,
		}); err != nil {
			fmt.Println("failed to upload progress", color.RedString("%s", err))
		}
		if progress.Precent >= 1 {
			closeOnce.Do(func() {
				close(endro)
				trace.Stop()
			})
		}
	})
	trace.Init()
	<-endro
}
