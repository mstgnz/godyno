package godyno

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestQueryToStructContextCanceled(t *testing.T) {
	db, _ := newMock(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := QueryToStructContext(ctx, db, "SELECT 1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestQueryToStructContextInTransaction(t *testing.T) {
	db, mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT").WithArgs(7).WillReturnRows(mockRows([]string{"id"}, []any{int64(7)}))
	mock.ExpectCommit()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	res, err := QueryToStructContext(context.Background(), tx, "SELECT id FROM t WHERE id = ?", 7)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].GetInt("id") != 7 {
		t.Fatalf("got %v", res)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestQueryToStructContextOnConn(t *testing.T) {
	db, mock := newMock(t)
	mock.ExpectQuery("SELECT").WillReturnRows(mockRows([]string{"n"}, []any{int64(1)}))
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	res, err := QueryToStructContext(context.Background(), conn, "SELECT")
	if err != nil || len(res) != 1 {
		t.Fatalf("res = %v, err = %v", res, err)
	}
}

func TestScanRowsClosesRows(t *testing.T) {
	db, mock := newMock(t)
	mock.ExpectQuery("SELECT").WillReturnRows(mockRows([]string{"id"}, []any{int64(1)})).RowsWillBeClosed()
	rows, err := db.Query("SELECT")
	if err != nil {
		t.Fatal(err)
	}
	res, err := ScanRows(rows)
	if err != nil || len(res) != 1 {
		t.Fatalf("res = %v, err = %v", res, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestScanRowsEmptyReturnsNil(t *testing.T) {
	res, err := queryMock(t, []string{"id"})
	if err != nil || res != nil {
		t.Fatalf("res = %#v, err = %v, want nil, nil", res, err)
	}
}

func TestScanRowsIterationError(t *testing.T) {
	db, mock := newMock(t)
	rows := mockRows([]string{"id"}, []any{int64(1)}, []any{int64(2)}).RowError(1, errors.New("boom"))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)
	_, err := QueryToStruct(db, "SELECT")
	if err == nil || !strings.Contains(err.Error(), "row iteration error") {
		t.Fatalf("err = %v, want a row iteration error", err)
	}
}

func TestQueryIter(t *testing.T) {
	t.Run("yields every row", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery("SELECT").WillReturnRows(mockRows([]string{"id"}, []any{int64(1)}, []any{int64(2)}, []any{int64(3)}))
		var ids []int
		for r, err := range QueryIter(context.Background(), db, "SELECT") {
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, r.GetInt("id"))
		}
		if len(ids) != 3 || ids[0] != 1 || ids[2] != 3 {
			t.Fatalf("ids = %v", ids)
		}
	})

	t.Run("breaking early closes rows", func(t *testing.T) {
		db, mock := newMock(t)
		rows := mockRows([]string{"id"}, []any{int64(1)}, []any{int64(2)})
		mock.ExpectQuery("SELECT").WillReturnRows(rows).RowsWillBeClosed()
		for range QueryIter(context.Background(), db, "SELECT") {
			break
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
	})

	t.Run("query error is yielded once", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery("SELECT").WillReturnError(errors.New("no table"))
		n := 0
		for r, err := range QueryIter(context.Background(), db, "SELECT") {
			n++
			if r != nil || err == nil || !strings.Contains(err.Error(), "no table") {
				t.Fatalf("got (%v, %v)", r, err)
			}
		}
		if n != 1 {
			t.Fatalf("yielded %d times, want 1", n)
		}
	})

	t.Run("duplicate columns are yielded as an error", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery("SELECT").WillReturnRows(mockRows([]string{"id", "id"}, []any{int64(1), int64(2)}))
		for r, err := range QueryIter(context.Background(), db, "SELECT") {
			if r != nil || err == nil || !strings.Contains(err.Error(), "duplicate column") {
				t.Fatalf("got (%v, %v)", r, err)
			}
		}
	})

	t.Run("iteration error comes after the good rows", func(t *testing.T) {
		db, mock := newMock(t)
		rows := mockRows([]string{"id"}, []any{int64(1)}, []any{int64(2)}).RowError(1, errors.New("boom"))
		mock.ExpectQuery("SELECT").WillReturnRows(rows)
		var good int
		var lastErr error
		for r, err := range QueryIter(context.Background(), db, "SELECT") {
			if err != nil {
				lastErr = err
				continue
			}
			if r.GetInt("id") == 1 {
				good++
			}
		}
		if good != 1 || lastErr == nil || !strings.Contains(lastErr.Error(), "boom") {
			t.Fatalf("good = %d, err = %v", good, lastErr)
		}
	})
}
