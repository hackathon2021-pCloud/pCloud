package backup

import (
	"context"
	"os/exec"

	"github.com/pingcap/tiup/pkg/utils"
)

type BR struct {
	Path    string
	Version utils.Version
}

type BRBuilder []string

func NewRestore(pdAddr string) *BRBuilder {
	return &BRBuilder{"restore", "full", "-u", pdAddr}
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
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	cmd.Env = []string{"AWS_ACCESS_KEY=root", "AWS_SECRET_KEY=a123456;"}
	cmd.Start()
	return cmd.Wait()
}

type CdcCtl struct {
	changeFeedId string
	Path         string
	Version      utils.Version
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
	cmd := exec.CommandContext(ctx, c.Path, args...)
	return cmd.Output()
}
