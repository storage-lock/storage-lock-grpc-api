# storage-lock gRPC 管理接口

`admin.proto` 是管理面的线协议，与 HTTP 管理接口查询同一类信息：持有者、剩余租约、读写模式、等待队列和 fencing 版本。

接口只有 `ListLocks` 和 `GetLock`。不提供强制解锁或删除，因为删除墓碑会让 fencing token 回退。

生成 Go 代码：

```bash
protoc --go_out=. --go-grpc_out=. admin.proto
```

服务端实现直接复用 `storage-lock-http-api/admin` 的查询结果，不在 gRPC 层重写锁语义。
