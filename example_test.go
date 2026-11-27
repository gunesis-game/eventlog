package eventlog_test

import (
	"context"
	"fmt"

	"github.com/gunesis-game/eventlog"
)

type InvoiceID [16]byte
type LedgerID [16]byte

// InvoiceIssued is a consumer-defined Event.
type InvoiceIssued struct {
	ID     InvoiceID
	Amount int64
	Memo   *string
}

func (i *InvoiceIssued) Fields() eventlog.Fields {
	return eventlog.Fields{
		eventlog.IdentityOf(func(i *InvoiceIssued) *InvoiceID { return &i.ID }),
		eventlog.FieldOf(func(i *InvoiceIssued) *int64 { return &i.Amount }),
		eventlog.FieldOfNullable(func(i *InvoiceIssued) **string { return &i.Memo }),
	}
}

// Ledger is a consumer-defined Channel.
type Ledger struct {
	ID            LedgerID
	notifications chan eventlog.Event
	errs          chan error
}

func (l *Ledger) Fields() eventlog.Fields {
	return eventlog.Fields{eventlog.IdentityOf(func(l *Ledger) *LedgerID { return &l.ID })}
}

func (l *Ledger) Notifications() <-chan eventlog.Event { return l.notifications }
func (l *Ledger) Errors() <-chan error                 { return l.errs }

// API-01, API-13: a consumer compiles against only the approved public API.
// The example is compiled but not run, because Config is one-time per process.
func Example() {
	if err := eventlog.Config(eventlog.WithStorageDirectory("./data")); err != nil {
		panic(err)
	}
	newLedger := func(notifications chan eventlog.Event, errs chan error) *Ledger {
		return &Ledger{ID: LedgerID{1}, notifications: notifications, errs: errs}
	}
	invoice, ledger, err := eventlog.PrepareLog[*InvoiceIssued, *Ledger](context.Background(), newLedger)
	if err != nil {
		panic(err)
	}
	invoice.ID = InvoiceID{7}
	invoice.Amount = 1200
	for {
		select {
		case event := <-ledger.Notifications():
			if issued, ok := event.(*InvoiceIssued); ok {
				fmt.Println("invoice amount", issued.Amount)
			}
		case err := <-ledger.Errors():
			fmt.Println("ledger error:", err)
		}
	}
}
