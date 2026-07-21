package mediastore

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// newS3Mock 模拟 S3 ListObjectsV2 与 DeleteObject，返回 server 与已删除对象键的记录。
func newS3Mock(t *testing.T, listXML string) (*httptest.Server, *sync.Map) {
	t.Helper()
	deleted := &sync.Map{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(listXML))
		case r.Method == http.MethodDelete:
			deleted.Store(strings.TrimPrefix(r.URL.Path, "/media-bucket/"), true)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	return srv, deleted
}

func newTestS3(t *testing.T, endpoint string, autoCleanup bool) *S3 {
	t.Helper()
	s3, err := NewS3(S3Options{
		Endpoint:    strings.TrimPrefix(endpoint, "http://"),
		Region:      "us-east-1",
		Bucket:      "media-bucket",
		AccessKey:   "ak",
		SecretKey:   "sk",
		UseSSL:      false,
		KeyPrefix:   "tmf",
		AutoCleanup: autoCleanup,
	}, newTestLocal(t, ""))
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}
	return s3
}

func listBucketXML(newTime string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>media-bucket</Name>
  <Prefix>tmf/</Prefix>
  <KeyCount>2</KeyCount>
  <MaxKeys>1000</MaxKeys>
  <IsTruncated>false</IsTruncated>
  <Contents>
    <Key>tmf/telegram/source_1/old.jpg</Key>
    <LastModified>2020-01-01T00:00:00.000Z</LastModified>
    <ETag>&quot;a&quot;</ETag>
    <Size>3</Size>
    <StorageClass>STANDARD</StorageClass>
  </Contents>
  <Contents>
    <Key>tmf/telegram/source_1/new.jpg</Key>
    <LastModified>%s</LastModified>
    <ETag>&quot;b&quot;</ETag>
    <Size>3</Size>
    <StorageClass>STANDARD</StorageClass>
  </Contents>
</ListBucketResult>`, newTime)
}

func TestS3CleanupRemovesExpiredObjects(t *testing.T) {
	newTime := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	srv, deleted := newS3Mock(t, listBucketXML(newTime))
	defer srv.Close()

	s3 := newTestS3(t, srv.URL, true)
	removed, err := s3.Cleanup(context.Background(), 30*24*time.Hour)
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if removed != 1 {
		t.Fatalf("应删除 1 个过期对象, got %d", removed)
	}
	if _, ok := deleted.Load("tmf/telegram/source_1/old.jpg"); !ok {
		t.Fatal("过期对象未被删除")
	}
	if _, ok := deleted.Load("tmf/telegram/source_1/new.jpg"); ok {
		t.Fatal("保留期内对象不应被删除")
	}
}

func TestS3CleanupDisabledSkipsRemote(t *testing.T) {
	newTime := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	srv, deleted := newS3Mock(t, listBucketXML(newTime))
	defer srv.Close()

	s3 := newTestS3(t, srv.URL, false)
	removed, err := s3.Cleanup(context.Background(), 30*24*time.Hour)
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if removed != 0 {
		t.Fatalf("未开启 auto_cleanup 不应删除远端对象, got %d", removed)
	}
	count := 0
	deleted.Range(func(_, _ any) bool { count++; return true })
	if count != 0 {
		t.Fatal("未开启 auto_cleanup 不应发起删除请求")
	}
}

func TestS3ManualCleanupCanExplicitlyDeleteRemote(t *testing.T) {
	newTime := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	srv, deleted := newS3Mock(t, listBucketXML(newTime))
	defer srv.Close()

	s3 := newTestS3(t, srv.URL, false)
	result, err := s3.CleanupDetailed(context.Background(), 30*24*time.Hour, false, true)
	if err != nil {
		t.Fatalf("CleanupDetailed: %v", err)
	}
	if result.DeletedLocalFiles != 0 || result.DeletedRemoteObjects != 1 {
		t.Fatalf("手动清理结果不符: %+v", result)
	}
	if _, ok := deleted.Load("tmf/telegram/source_1/old.jpg"); !ok {
		t.Fatal("显式选择远端后应删除过期对象")
	}
}
