package backup

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/pingcap/tiup/pkg/tui/progress"
	"github.com/pingcap/tiup/pkg/utils"
)

type BR struct {
	Path    string
	Version utils.Version
}

type BRBuilder []string

func NewRestore(pdAddr string) *BRBuilder {
	// ignore checksum for log restore
	return &BRBuilder{"restore", "full", "-u", pdAddr, "--checksum=false", "--log-format", "json"}
}

func NewLogRestore(pdAddr string) *BRBuilder {
	return &BRBuilder{"restore", "cdclog", "-u", pdAddr, "--log-format", "json"}
}

func NewBackup(pdAddr string) *BRBuilder {
	return &BRBuilder{"backup", "full", "-u", pdAddr, "--log-format", "json"}
}

func (builder *BRBuilder) Storage(s string) {
	*builder = append(*builder, "-s", s)
}

// TimeRange spec a time range (present in unix mills)
func (builder *BRBuilder) TimeRange(start, end uint) {
	if start != 0 {
		*builder = append(*builder, "--start-ts", strconv.FormatUint(uint64(start<<18), 10))
	}
	if end != 0 {
		*builder = append(*builder, "--end-ts", strconv.FormatUint(uint64(end<<18), 10))
	}
}

func (builder *BRBuilder) Build() []string {
	return *builder
}

type BRProcess struct {
	Trace  ProgressTracer
	Handle *exec.Cmd
}

func (br *BR) Execute(ctx context.Context, args ...string) BRProcess {
	cmd := exec.CommandContext(ctx, br.Path, args...)
	r, w := io.Pipe()
	tr := TraceByLog(r)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	cmd.Env = []string{"BR_LOG_TO_TERM=1"}
	cmd.Start()
	return BRProcess{
		Handle: cmd,
		Trace:  tr,
	}
}

func (br *BR) CreateCmd(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, br.Path, args...)
	cmd.Env = []string{"BR_LOG_TO_TERM=1"}
	return cmd
}

func (br *BRProcess) WaitAndPrintProgress(prefix string) error {
	bar := progress.NewSingleBar(prefix)
	defer func() {
		bar.UpdateDisplay(&progress.DisplayProps{
			Prefix: prefix,
			Mode:   progress.ModeDone,
		})
		bar.StopRenderLoop()
	}()
	bar.UpdateDisplay(&progress.DisplayProps{
		Prefix: prefix,
		Mode:   progress.ModeProgress,
		Suffix: "00.00%",
	})
	bar.StartRenderLoop()
	br.Trace.OnProgress(func(pg Progress) {
		if pg.Precent >= 1 {
			bar.UpdateDisplay(&progress.DisplayProps{
				Prefix: prefix,
				Mode:   progress.ModeDone,
			})
			return
		}
		bar.UpdateDisplay(&progress.DisplayProps{
			Prefix: prefix,
			Suffix: fmt.Sprintf("%02.2f%%", pg.Precent*100),
			Mode:   progress.ModeProgress,
		})
	})

	if err := br.Handle.Wait(); err != nil {
		return err
	}
	return nil
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
