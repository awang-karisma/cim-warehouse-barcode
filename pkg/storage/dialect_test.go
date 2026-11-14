package storage

import "testing"

func TestRebindPostgresPreservesQuotedQuestionMarks(t *testing.T) {
	s := &Storage{dialect: "postgres"}
	got := s.rebind("SELECT '?', \"?\" FROM entries WHERE a=? AND b='it''s ?' AND c=?")
	want := "SELECT '?', \"?\" FROM entries WHERE a=$1 AND b='it''s ?' AND c=$2"
	if got != want {
		t.Fatalf("rebind() = %q, want %q", got, want)
	}
}

func TestRebindSQLiteReturnsOriginal(t *testing.T) {
	query := "SELECT * FROM entries WHERE a=?"
	if got := (&Storage{dialect: "sqlite"}).rebind(query); got != query {
		t.Fatalf("rebind() = %q, want %q", got, query)
	}
}
