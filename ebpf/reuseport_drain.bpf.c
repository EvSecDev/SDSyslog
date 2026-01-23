#include "include/vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

/* Compile-time sanity checks */
_Static_assert(sizeof(__u8) == 1, "__u8 must be 1 byte");
_Static_assert(sizeof(__u64) == 8, "__u64 must be 8 bytes");
_Static_assert(__alignof__(__u64) == 8, "__u64 alignment unexpected");
_Static_assert(sizeof(struct sk_reuseport_md) >= 8, "sk_reuseport_md size is unexpectedly small");

/* BPF globals */
const char LICENSE[] SEC("license") = "GPL";
#define SOCKET_DRAINING 1

/* Key:   socket cookie (u64)
 * Value: 1 = draining, 0 = active
 * Exposed via pinned map to userspace */
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 4096);
    __type(key, __u64);
    __type(value, __u8);
} draining_sockets SEC(".maps");

/* Runs once per packet per candidate socket in a SO_REUSEPORT group.
 * Returning:
 *   SK_PASS -> socket is acceptable
 *   SK_DROP -> skip socket, try another */
SEC("sk_reuseport")
static int reuseport_select(struct sk_reuseport_md *ctx) {
    // Use socket pointer as unique key
    __u64 key = (__u64)(uintptr_t)ctx->sk;

    // Get current socket status (draining/not draining)
    __u8 *draining = bpf_map_lookup_elem(&draining_sockets, &key);

    // Socket marked as draining, don't route data to it
    if (draining && *draining == SOCKET_DRAINING) {
            // Not dropping the packet, just skipping this socket
            return SK_DROP;
    }

    // Default: allow new data to this socket
    return SK_PASS;
}
