package grpcapi

import (
	"context"
	"errors"
	"time"

	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

var (
	// ErrLockBusy 表示锁被其他持有者占用。
	ErrLockBusy = storage_lock.ErrLockBusy
	// ErrOwnerRequired 表示续租或释放没有携带 owner。
	ErrOwnerRequired = errors.New("owner id is required")
)

// LockService 是无状态锁服务。
// 服务端不启动看门狗，调用方必须在租约到期前 Renew。
// 这里不依赖生成的 protobuf 代码：传输层把请求填进这些结构即可。
type LockService struct {
	storage storage.Storage
	options storage_lock.StorageLockOptions
}

// AcquireRequest 是一次获取请求。
type AcquireRequest struct {
	LockID  string
	OwnerID string
	Mode    string
	TTL     time.Duration
	Wait    bool
}

// Lease 是获取或续租成功后返回给调用方的租约。
type Lease struct {
	LockID       string
	OwnerID      string
	FencingToken storage.Version
	TTL          time.Duration
}

// OwnerRequest 是续租或释放请求。
type OwnerRequest struct {
	LockID  string
	OwnerID string
}

// NewLockService 用默认租约创建服务。storage 不能为空。
func NewLockService(s storage.Storage) (*LockService, error) {
	if s == nil {
		return nil, storage_lock.ErrStorageNil
	}
	options := storage_lock.NewStorageLockOptions()
	options.DisableWatchDog = true
	return &LockService{storage: s, options: *options}, nil
}

// Acquire 获取一把锁。Wait 为 false 时被占用立即返回 ErrLockBusy。
func (s *LockService) Acquire(ctx context.Context, req AcquireRequest) (*Lease, error) {
	if req.LockID == "" {
		return nil, storage_lock.ErrLockIdEmpty
	}
	options := s.options
	options.LockId = req.LockID
	if req.TTL > 0 {
		options.LeaseExpireAfter = req.TTL
	}
	lock, err := storage_lock.NewStorageLockWithOptions(s.storage, &options)
	if err != nil {
		return nil, err
	}
	owner := req.OwnerID
	if owner == "" {
		owner = storage_lock.NewOwnerIdGenerator().GenOwnerId()
	}
	if req.Mode == "read" {
		err = lock.RLock(ctx, owner)
	} else if req.Wait {
		err = lock.Lock(ctx, owner)
	} else {
		err = lock.TryLock(ctx, owner)
	}
	if err != nil {
		return nil, err
	}
	token, err := lock.GetFencingToken(ctx, owner)
	if err != nil {
		_ = lock.UnLock(context.WithoutCancel(ctx), owner)
		return nil, err
	}
	ttl, _ := lock.TTL(ctx, owner)
	return &Lease{LockID: req.LockID, OwnerID: owner, FencingToken: token, TTL: ttl}, nil
}

// Renew 延长当前 owner 的租约。不是当前 owner 时返回原错误。
func (s *LockService) Renew(ctx context.Context, req OwnerRequest) (*Lease, error) {
	lock, owner, err := s.lockFor(req)
	if err != nil {
		return nil, err
	}
	if err := lock.Renew(ctx, owner); err != nil {
		return nil, err
	}
	ttl, _ := lock.TTL(ctx, owner)
	return &Lease{LockID: req.LockID, OwnerID: owner, TTL: ttl}, nil
}

// Release 释放当前 owner 的锁。
func (s *LockService) Release(ctx context.Context, req OwnerRequest) error {
	lock, owner, err := s.lockFor(req)
	if err != nil {
		return err
	}
	return lock.UnLock(ctx, owner)
}

func (s *LockService) lockFor(req OwnerRequest) (*storage_lock.StorageLock, string, error) {
	if req.LockID == "" {
		return nil, "", storage_lock.ErrLockIdEmpty
	}
	if req.OwnerID == "" {
		return nil, "", ErrOwnerRequired
	}
	options := s.options
	options.LockId = req.LockID
	lock, err := storage_lock.NewStorageLockWithOptions(s.storage, &options)
	if err != nil {
		return nil, "", err
	}
	return lock, req.OwnerID, nil
}
