package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/pingcap/tiup/pkg/cluster/backup"
)

func main() {
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
		fmt.Printf("%+v\n", progress)
	})
	<-endro
}
