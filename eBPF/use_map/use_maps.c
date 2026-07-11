#include "use_maps.skel.h"

#include <bpf/bpf.h>
#include <bpf/libbpf.h>

#include <stdio.h>
#include <stdlib.h>
#include <signal.h>
#include <unistd.h>
#include <time.h>
#include <stdint.h> 
#include <inttypes.h> 

// 处理退出信号
static volatile bool isexit = false;

static void sig_handler(int sig)
{
    isexit = true;
}

// libbpf 日志回调: 匹配 libbpf_print_fn_t 签名
static int libbpf_print_cb(enum libbpf_print_level level, const char *format, va_list ap)
{
    // 忽略 level, 统一输出到 stderr
    return vfprintf(stderr, format, ap);
}

// 获取event时回调，打印信息
static int handle_rb_event(void *ctx, void *data, size_t data_sz)
{
    const struct event {
        uint32_t pid;
        uint32_t uid;
        char comm[16];
        int ret;
    } *e = data;

    if (data_sz < sizeof(*e)) {
        fprintf(stderr, "short event: %zu < %zu\n", data_sz, sizeof(*e));
        return 0;
    }

    // if (e->ret == 0)
    //     printf("[ENTER] PID=%-6d UID=%-4d COMM=%-16s -> openat\n",
    //            e->pid, e->uid, e->comm);
    // else
    //     printf("[EXIT ] PID=%-6d UID=%-4d COMM=%-16s -> ret=%-6d\n",
    //            e->pid, e->uid, e->comm, e->ret);

    return 0;
}

int main(void)
{
    // eBPF 程序
    struct openat_counts *skel = NULL;
    struct ring_buffer *rb = NULL;
    int err;

    // 调试输出: 将 libbpf 的日志输出到 stderr
    // libbpf_set_print(libbpf_print_cb);

    signal(SIGINT, sig_handler);
    signal(SIGTERM, sig_handler);

    skel = openat_counts__open_and_load();
    if (!skel) {
        fprintf(stderr, "Failed to open & load BPF skeleton\n");
        return 1;
    }

    err = openat_counts__attach(skel);
    if(err) {
        fprintf(stderr, "Failed to attach BPF skeleton: %d\n", err);
        goto cleanup;
    }

    printf("eBPF maps program loaded & attached.\n");

    // new Ring Buffer 并设置回调函数
    rb = ring_buffer__new(bpf_map__fd(skel->maps.ringbuf), handle_rb_event, NULL, NULL);
    if(!rb) {
        fprintf(stderr, "Failed to create ring buffer\n");
        goto cleanup;
    }

    while(!isexit) {
        // poll
        ring_buffer__poll(rb, 200);

        // 硬核三秒读一次哈希表
        static time_t last = 0;
        time_t now = time(NULL);
        if(now - last >= 3) {
            last = now;

            uint32_t key = 0, prev_key;
            int cnt = 0;
            while(bpf_map_get_next_key(
                bpf_map__fd(skel->maps.openat_counts),
                cnt ? &prev_key : NULL,
                &key
                ) == 0) {
                uint64_t val;
                if(! bpf_map_lookup_elem(
                    bpf_map__fd(skel->maps.openat_counts),
                    &key, &val)) {
                    printf("[MAP] PID %u -> openat count: %" PRIu64 "\n", key, val);
                }
                prev_key = key;
                // 取前5
                if(++cnt >= 5) {
                    break;
                }
            }
            if(cnt) {
                printf("------\n");
            }
        }
    }
    printf("\nExit?!\n");

cleanup:
    ring_buffer__free(rb);
    openat_counts__destroy(skel);
    return err;
}