#ifndef STELLAR_WASI_FCNTL_H
#define STELLAR_WASI_FCNTL_H
#include_next <fcntl.h>
/* Reserved unsupported operation: WASI libc rejects it with EINVAL. */
#define F_DUPFD 0
#endif
