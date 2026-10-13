#ifndef STELLAR_WASI_PTHREAD_H
#define STELLAR_WASI_PTHREAD_H
#include_next <pthread.h>
_Noreturn void pthread_exit(void *);
#endif
