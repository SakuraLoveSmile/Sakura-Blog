/* Fatal native codec errors terminate their isolated WASI instance. */
#ifndef STELLAR_WASI_SETJMP_H
#define STELLAR_WASI_SETJMP_H
#include <stdlib.h>
typedef int jmp_buf[1];
#define setjmp(buffer) (0)
static _Noreturn inline void stellar_longjmp(jmp_buf buffer, int value) {
  (void)buffer;
  (void)value;
  exit(70);
}
#define longjmp stellar_longjmp
#endif
