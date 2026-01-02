package subscriber

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/ice-cream-psychics-club/dropbox/pkg/dropbox"
)

type Propagator struct {
	Source  string
	Targets []Target
	Client  *dropbox.Client
	Logger  *slog.Logger
}

type Target struct {
	Name      string
	Transform func(r io.Reader) (io.Reader, error)
}

func (p *Propagator) Handle(account string, files []dropbox.File) error {
	var propagate *dropbox.File
	for _, f := range files {
		// TODO: check f.IsDownloadable
		if f.Name == p.Source {
			propagate = &f
			break
		} else {
			p.Logger.Debug("skipping " + f.Name)
		}
	}
	if propagate == nil {
		return nil
	}

	p.Logger.Info("subscriber.Propagator: " + propagate.Name)

	in, err := p.Client.Download(propagate.Name)
	if err != nil {
		return fmt.Errorf("error requesting download: %w", err)
	}

	// TODO: best-effort
	for _, t := range p.Targets {
		out, err := t.Transform(in)
		if err != nil {
			return fmt.Errorf("error transforming source to target: %w", err)
		}

		if err := p.Client.Upload(t.Name, out); err != nil {
			return fmt.Errorf("error uploading target: %w", err)
		}
	}

	return nil
}
