package backup

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/pingcap/tiup/pkg/utils"
)

type BR struct {
	Path    string
	Version utils.Version
	Wait    bool
}

type BRBuilder []string

func NewRestore(pdAddr string) *BRBuilder {
	// ignore checksum for log restore
	return &BRBuilder{"restore", "full", "-u", pdAddr, "--checksum=false"}
}

func NewLogRestore(pdAddr string) *BRBuilder {
	return &BRBuilder{"restore", "cdclog", "-u", pdAddr}
}

func NewBackup(pdAddr string) *BRBuilder {
	return &BRBuilder{"backup", "full", "-u", pdAddr}
}

func (builder *BRBuilder) Storage(s string) {
	*builder = append(*builder, "-s", s)
}

func (builder *BRBuilder) Build() []string {
	return *builder
}

func (br *BR) Execute(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, br.Path, args...)
	fmt.Println(br.Path, args)
	if br.Wait {
		return cmd.Run()
	}
	// Don't wait
	return cmd.Start()
}

type CdcCtl struct {
	changeFeedId string
	Path         string
	Version      utils.Version
	PipeYes      bool
}

type CdcCtlBuilder []string

func NewIncrementalBackup(changeFeedId string, pdAddr string) *CdcCtlBuilder {
	return &CdcCtlBuilder{"cli", "changefeed", "create", "--pd", pdAddr, "--changefeed-id", changeFeedId}
}

func (builder *CdcCtlBuilder) Storage(s string) {
	*builder = append(*builder, "--sink-uri", s)
}

func (builder *CdcCtlBuilder) Build() []string {
	return *builder
}

func GetIncrementalBackup(changeFeedId string, pdAddr string) *CdcCtlBuilder {
	return &CdcCtlBuilder{"cli", "changefeed", "query", "--pd", pdAddr, "--changefeed-id", changeFeedId}
}

func (c *CdcCtl) Execute(ctx context.Context, args ...string) ([]byte, error) {
	// use pipeline to avoid input yes in cdc ctl
	if c.PipeYes {
		c1 := exec.CommandContext(ctx, "echo", "Y")
		c2 := exec.CommandContext(ctx, c.Path, args...)

		c2.Stdin, _ = c1.StdoutPipe()
		var outb bytes.Buffer
		c2.Stdout = &outb
		err := c2.Start()
		if err != nil {
			return nil, err
		}
		err = c1.Run()
		if err != nil {
			return nil, err
		}
		err = c2.Wait()
		if err != nil {
			return nil, err
		}
		return outb.Bytes(), nil
	} else {
		cmd := exec.CommandContext(ctx, c.Path, args...)
		return cmd.Output()
	}


}
