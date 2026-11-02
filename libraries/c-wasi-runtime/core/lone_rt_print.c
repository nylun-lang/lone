// WASI import: fd_write(fd, iovs, iovs_len, nwritten)
__attribute__((import_module("wasi_snapshot_preview1"), import_name("fd_write")))
int __wasi_fd_write(int fd, const void *iovs, int iovs_len, int *nwritten);

typedef struct {
    const char *buf;
    int len;
} ciovec_t;

// Exported to be linked or called by Falone
__attribute__((export_name("__lone_rt_print")))
void __lone_rt_print(const char *str, int len) {
    ciovec_t iov = { .buf = str, .len = len };
    int written = 0;
    __wasi_fd_write(1, &iov, 1, &written);
}