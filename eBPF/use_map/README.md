 使用 map 的案例。
 拦截并统计 openat 调用次数
 
 用到了 percpu-hash-map（key:pid, value:count）、ring-buffer（事件通知）
