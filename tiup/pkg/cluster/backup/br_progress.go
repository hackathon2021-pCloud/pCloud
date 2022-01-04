package backup

import (
	"bufio"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
)

var log = logprinter.NewLogger("pCloud.backup")

type Progress struct {
	Precent  float64
	RecordAt time.Time
}

type ProgressTracer interface {
	OnProgress(func(progress Progress))
	Stop() error
}

// LogProgressTracer traces progress of BR via the log.
type LogProgressTracer struct {
	logStream     io.ReadCloser
	subscriptions []func(progress Progress)
}

func TraceByLog(logStream io.ReadCloser) ProgressTracer {
	lt := &LogProgressTracer{
		logStream: logStream,
	}
	go lt.ReadLoop()
	return lt
}

type BRProgress struct {
	Message  string `json:"message"`
	Time     string `json:"time"`
	Step     string `json:"step"`
	Progress string `json:"progress"`
}

func (prog *BRProgress) ToProgress() *Progress {
	var (
		err    error
		result = new(Progress)
	)

	p := strings.TrimSuffix(prog.Progress, "%")
	result.Precent, err = strconv.ParseFloat(p, 64)
	if err != nil {
		return nil
	}
	result.RecordAt, err = time.Parse("2006/01/02 15:04:05.999 -07:00", prog.Time)
	if err != nil {
		log.Warnf("failed to parse date (err = %s)", err)
		return nil
	}
	if prog.Step == "Checksum" {
		result.Precent /= 100.0
	} else {
		result.Precent /= 200.0
	}
	return result
}

func (lt *LogProgressTracer) ReadLoop() {
	lines := bufio.NewScanner(lt.logStream)
	for {
		if !lines.Scan() {
			lt.SendProgress(&Progress{
				RecordAt: time.Now(),
				Precent:  1,
			})
			return
		}
		prog := BRProgress{}
		err := json.Unmarshal([]byte(lines.Text()), &prog)
		if err != nil {
			log.Warnf("failed to parse progress (err = %s)", err)
		}
		if prog.Message == "progress" {
			lt.SendProgress(prog.ToProgress())
		}
	}
}

func (lt *LogProgressTracer) SendProgress(p *Progress) {
	if p == nil {
		return
	}
	for _, sub := range lt.subscriptions {
		sub(*p)
	}
}

func (lt *LogProgressTracer) Stop() error {
	return lt.logStream.Close()
}

func (lt *LogProgressTracer) OnProgress(f func(progress Progress)) {
	lt.subscriptions = append(lt.subscriptions, f)
}