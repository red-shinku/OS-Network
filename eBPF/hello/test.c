#include "test.skel.h"

#include <unistd.h>
#include <stdio.h>
#include <bpf/libbpf.h>
#include <sys/resource.h>

static int libbpf_print_fn(enum libbpf_print_level level, 
                            const char *format, va_list args)
{
    return vfprintf(stderr, format, args);
}

int main(void)
{
    struct test_ebpf *skel;
    int err;

    libbpf_set_print(libbpf_print_fn);

    skel = test_ebpf__open_and_load();
    if(!skel) {
        fprintf(stderr, "Failed to open and load BPF skeleton\n");
        return 1;
    }

    // 挂载到事件
    err = test_ebpf__attach(skel);
    if(err) {
        fprintf(stderr, "Failed to attach BPF skeleton\n");
        goto cleanup;
    }

    // 保持运行
    for(;;) {
        sleep(2);
    }

cleanup:
    test_ebpf__destroy(skel);
    return err;
}

    