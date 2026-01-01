package subscriber

import (
	"fmt"
	"log/slog"

	"github.com/ice-cream-psychics-club/dropbox/pkg/dropbox"
)

type Logger struct {
	*slog.Logger
}

func (l *Logger) Handle(account string, files []dropbox.File) error {
	for _, f := range files {
		fmt.Printf(`
			name 			%s
			content hash 	%s
			modified at 	%s
			modified by 	%s
		`, f.Name, f.ContentHash, f.ClientModified, f.SharingInfo.ModifiedBy)
	}

	return nil
}
