package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

// deadLetters is the operator's handle on parked outbox messages (outbox.PostgresStore).
type deadLetters interface {
	DeadLetters(ctx context.Context, limit int) ([]outbox.Message, error)
	Retry(ctx context.Context, id int64) error
}

const outboxUsage = "usage: kyber outbox dead-letters | kyber outbox retry <id>"

// runOutbox serves `kyber outbox dead-letters` (list parked messages) and `kyber outbox retry <id>`
// (return one to the queue once its handler is fixed).
func runOutbox(ctx context.Context, args []string, store deadLetters, w io.Writer) error {
	switch {
	case len(args) == 1 && args[0] == "dead-letters":
		list, err := store.DeadLetters(ctx, 500)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			_, err := fmt.Fprintln(w, "no dead letters")
			return err
		}
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tEVENT\tWORKSPACE\tACTOR\tRECORDED\tATTEMPTS\tLAST ERROR")
		for _, m := range list {
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%d\t%s\n", m.ID, m.Name, m.Workspace, m.Actor, m.At.UTC().Format("2006-01-02 15:04:05"), m.Attempts, m.LastError)
		}
		return tw.Flush()
	case len(args) == 2 && args[0] == "retry":
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return errors.New(outboxUsage)
		}
		if err := store.Retry(ctx, id); err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "message %d queued again\n", id)
		return err
	}
	return errors.New(outboxUsage)
}
