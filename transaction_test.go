package eventlog

import "testing"

func TestTransactionChecksumExactSerializedBytes(t *testing.T) {
	records := [][]byte{
		[]byte("{\"kind\":\"channel\"}\n"),
		[]byte("{\"kind\":\"event\"}\n"),
	}
	const want = "40b89aed5ed45a476a7e7984de6f0f58e6e732ea06b4a759bd45555108a9c63b"
	if got := transactionChecksum(records); got != want {
		t.Fatalf("checksum = %s, want %s", got, want)
	}
}

func TestTransactionChecksumIncludesNewline(t *testing.T) {
	withNewline := transactionChecksum([][]byte{[]byte("record\n")})
	withoutNewline := transactionChecksum([][]byte{[]byte("record")})
	if withNewline == withoutNewline {
		t.Fatal("checksum must include the terminating newline")
	}
}

func TestTransactionChecksumChangesWithRecordOrder(t *testing.T) {
	first := [][]byte{[]byte("channel\n"), []byte("event\n")}
	second := [][]byte{[]byte("event\n"), []byte("channel\n")}
	if transactionChecksum(first) == transactionChecksum(second) {
		t.Fatal("record order must affect checksum")
	}
}
