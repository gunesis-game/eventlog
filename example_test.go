package eventlog_test

import (
	"context"
	"errors"
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
	tx     chan<- func() error
}

func (i *InvoiceIssued) Fields() eventlog.Fields {
	return eventlog.Fields{
		eventlog.IdentityOf(func(i *InvoiceIssued) *InvoiceID { return &i.ID }),
		eventlog.FieldOf(func(i *InvoiceIssued) *int64 { return &i.Amount }),
		eventlog.FieldOfNullable(func(i *InvoiceIssued) **string { return &i.Memo }),
	}
}

func (i *InvoiceIssued) Transaction() chan<- func() error { return i.tx }

// Ledger is a consumer-defined Channel.
type Ledger struct {
	ID            LedgerID
	Currency      string
	notifications chan eventlog.Event
	errs          chan *eventlog.ChannelError
}

func (l *Ledger) Fields() eventlog.Fields {
	return eventlog.Fields{
		eventlog.IdentityOf(func(l *Ledger) *LedgerID { return &l.ID }),
		eventlog.FieldOf(func(l *Ledger) *string { return &l.Currency }),
	}
}

func (l *Ledger) Notifications() <-chan eventlog.Event  { return l.notifications }
func (l *Ledger) Errors() <-chan *eventlog.ChannelError { return l.errs }

var errOverLimit = errors.New("over credit limit")

// API-01, API-13: a consumer compiles against only the approved public API.
// The example is compiled but not run, because Config is one-time per process.
func Example() {
	if err := eventlog.Config(eventlog.WithStorage(eventlog.Local{Directory: "./data"})); err != nil {
		panic(err)
	}
	invoice, ledger, err := eventlog.PrepareLog[*InvoiceIssued, *Ledger](context.Background(),
		func(tx chan<- func() error) *InvoiceIssued {
			return &InvoiceIssued{ID: InvoiceID{7}, tx: tx}
		},
		func(notifications chan eventlog.Event, errs chan *eventlog.ChannelError) *Ledger {
			return &Ledger{ID: LedgerID{1}, Currency: "EUR", notifications: notifications, errs: errs}
		},
	)
	if err != nil {
		panic(err)
	}

	go func() {
		for failure := range ledger.Errors() {
			if failure.Kind == eventlog.TransactionRefused && errors.Is(failure, errOverLimit) {
				fmt.Println("refused:", failure)
			}
		}
	}()

	invoice.Transaction() <- func() error {
		if invoice.Amount+1200 > 10000 {
			return errOverLimit
		}
		invoice.Amount += 1200
		return nil
	}
	for event := range ledger.Notifications() {
		if issued, ok := event.(*InvoiceIssued); ok {
			fmt.Println("invoice amount", issued.Amount)
		}
	}
}
