/* Each decode runs in its own WASI module. A libtiff codec fatal error exits
 * that instance instead of jumping into a native stack frame. This avoids a
 * dependency on WebAssembly exception handling and never masks decode failure. */
#ifndef STELLAR_WASI_SETJMP_H
#define STELLAR_WASI_SETJMP_H
#include <stdlib.h>
typedef int jmp_buf[1];
#define setjmp(buffer) (0)
#define longjmp(buffer, value) exit(70)
#endif
