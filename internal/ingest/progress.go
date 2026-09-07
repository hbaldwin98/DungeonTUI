package ingest

import "fmt"

// Stage is one visible step of import.
type Stage string

const (
	StageRead     Stage = "read"
	StageFetch    Stage = "fetch"
	StageConvert  Stage = "convert"
	StageClassify Stage = "classify"
	StageWrite    Stage = "write"
	StageAgent    Stage = "agent"
)

// Progress is a status update for the TUI or CLI while ingest runs.
type Progress struct {
	Stage   Stage
	Message string
	Current int
	Total   int
}

func (p Progress) String() string {
	msg := p.Message
	if msg == "" {
		msg = string(p.Stage)
	}
	switch {
	case p.Total > 0:
		return fmt.Sprintf("%s  %s  %d/%d", p.Stage, msg, p.Current, p.Total)
	case p.Current > 0:
		return fmt.Sprintf("%s  %s  (%d)", p.Stage, msg, p.Current)
	default:
		return fmt.Sprintf("%s  %s", p.Stage, msg)
	}
}

func (o Options) report(stage Stage, message string, current, total int) {
	if o.Progress == nil {
		return
	}
	o.Progress(Progress{Stage: stage, Message: message, Current: current, Total: total})
}
