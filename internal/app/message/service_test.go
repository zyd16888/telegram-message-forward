package message

import (
	"context"
	"testing"
	"time"

	domainmessage "telegram-message-forward/internal/domain/message"
)

type fakeQueryRepo struct {
	query domainmessage.Query
}

func (f *fakeQueryRepo) List(_ context.Context, q domainmessage.Query) ([]domainmessage.ListItem, bool, error) {
	f.query = q
	return []domainmessage.ListItem{}, false, nil
}

func (f *fakeQueryRepo) GetDetail(_ context.Context, id int64) (*domainmessage.Detail, error) {
	return &domainmessage.Detail{Item: &domainmessage.ListItem{Message: &domainmessage.NormalizedMessage{ID: id}}}, nil
}

func TestListNormalizesQuery(t *testing.T) {
	repo := &fakeQueryRepo{}
	svc := NewService(repo)
	if _, _, err := svc.List(context.Background(), domainmessage.Query{Keyword: "  hello  ", Limit: 999}); err != nil {
		t.Fatal(err)
	}
	if repo.query.Keyword != "hello" || repo.query.Limit != 100 {
		t.Fatalf("query = %#v", repo.query)
	}
}

func TestListRejectsInvalidRange(t *testing.T) {
	repo := &fakeQueryRepo{}
	svc := NewService(repo)
	from := time.Now()
	to := from.Add(-time.Hour)
	if _, _, err := svc.List(context.Background(), domainmessage.Query{From: &from, To: &to}); err != ErrInvalidQuery {
		t.Fatalf("err = %v", err)
	}
}
