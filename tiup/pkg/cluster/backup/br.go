package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/pingcap/tiup/pkg/utils"
)

type BR struct {
	Path    string
	Version utils.Version
}

type BRBuilder []string

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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = []string{"AWS_ACCESS_KEY=root", "AWS_SECRET_KEY=a123456;"}
	fmt.Println("executing ", args)
	cmd.Start()
	return cmd.Wait()
}
