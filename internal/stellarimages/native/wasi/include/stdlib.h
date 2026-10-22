#ifndef STELLAR_WASI_STDLIB_H
#define STELLAR_WASI_STDLIB_H
#include_next <stdlib.h>
void arc4random_buf(void *, size_t);
#endif
