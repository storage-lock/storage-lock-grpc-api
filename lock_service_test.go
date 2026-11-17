package grpcapi

import (
	"context"
	"errors"
	"testing"
	"time"

	memory_storage "github.com/storage-lock/go-memory-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

func TestLockServiceAcquireRenewRelease(t *testing.T) {
	ctx := context.Background()
	service, err := NewLockService(memory_storage.NewMemoryStorage())
	if err != nil {
		t.Fatal(err)
	}
	lease, err := service.Acquire(ctx, AcquireRequest{LockID: "order", OwnerID: "worker", TTL: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if lease.FencingToken == 0 || lease.OwnerID != "worker" {
		t.Fatalf("lease = %+v", lease)
	}
	_, err = service.Acquire(ctx, AcquireRequest{LockID: "order", OwnerID: "other"})
	if !errors.Is(err, storage_lock.ErrLockBusy) {
		t.Fatalf("second acquire = %v", err)
	}
	renewed, err := service.Renew(ctx, OwnerRequest{LockID: "order", OwnerID: "worker"})
	if err != nil || renewed.TTL <= 0 {
		t.Fatalf("renew = %+v, err %v", renewed, err)
	}
	if err := service.Release(ctx, OwnerRequest{LockID: "order", OwnerID: "worker"}); err != nil {
		t.Fatal(err)
	}
	again, err := service.Acquire(ctx, AcquireRequest{LockID: "order", OwnerID: "other"})
	if err != nil {
		t.Fatal(err)
	}
	if again.FencingToken <= lease.FencingToken {
		t.Fatalf("fencing token did not advance: old %d new %d", lease.FencingToken, again.FencingToken)
	}
}
