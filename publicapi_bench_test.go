package eventlog_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gunesis-game/eventlog"
)

// The public-API benchmark shares one Config and one process-lifetime
// runtime across every run in this process, because Config is one-time per
// process. Its storage directory is left in the system temporary directory:
// the runtime holds it until the process exits.
var publicAPI struct {
	once    sync.Once
	err     error
	invoice *InvoiceIssued
	ledger  *Ledger
	amount  int64
}

func newBenchmarkInvoice(tx chan<- func() error) *InvoiceIssued {
	return &InvoiceIssued{ID: InvoiceID{9}, tx: tx}
}

func newBenchmarkLedger(notifications chan eventlog.Event, errs chan *eventlog.ChannelError) *Ledger {
	return &Ledger{ID: LedgerID{9}, Currency: "EUR", notifications: notifications, errs: errs}
}

func preparePublicAPI() {
	dir, err := os.MkdirTemp("", "eventlog-bench-*")
	if err != nil {
		publicAPI.err = err
		return
	}
	if err := eventlog.Config(eventlog.WithStorage(eventlog.Local{Directory: dir})); err != nil {
		publicAPI.err = err
		return
	}
	publicAPI.invoice, publicAPI.ledger, publicAPI.err = eventlog.PrepareLog[*InvoiceIssued, *Ledger](context.Background(), newBenchmarkInvoice, newBenchmarkLedger)
}

// B-API03: through the public API only, send a change to an Event and
// receive its notification.
func BenchmarkPublicAppendAndReceive(b *testing.B) {
	publicAPI.once.Do(preparePublicAPI)
	if publicAPI.err != nil {
		b.Fatal(publicAPI.err)
	}
	invoice, ledger := publicAPI.invoice, publicAPI.ledger
	b.ReportAllocs()
	for b.Loop() {
		publicAPI.amount++
		amount := publicAPI.amount
		invoice.Transaction() <- func() error {
			invoice.Amount = amount
			return nil
		}
		select {
		case event := <-ledger.Notifications():
			issued, ok := event.(*InvoiceIssued)
			if !ok || issued.Amount != amount {
				b.Fatalf("notification %#v, want *InvoiceIssued with amount %d", event, amount)
			}
		case failure := <-ledger.Errors():
			b.Fatal(failure)
		case <-time.After(time.Minute):
			b.Fatalf("timed out waiting for the notification with amount %d", amount)
		}
	}
}
