#ifndef STELLAR_WASI_TIME_H
#define STELLAR_WASI_TIME_H
#include_next <time.h>
void tzset(void);
#endif
