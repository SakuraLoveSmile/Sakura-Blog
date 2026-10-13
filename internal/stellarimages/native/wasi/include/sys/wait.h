#ifndef STELLAR_WASI_WAIT_H
#define STELLAR_WASI_WAIT_H
#include <sys/types.h>
#define WNOHANG 1
#define WUNTRACED 2
#define WIFEXITED(s) (((s) & 127) == 0)
#define WEXITSTATUS(s) (((s) >> 8) & 255)
#define WIFSIGNALED(s) (((s) & 127) != 0 && ((s) & 127) != 127)
#define WTERMSIG(s) ((s) & 127)
#define WIFSTOPPED(s) (((s) & 255) == 127)
#define WSTOPSIG(s) (((s) >> 8) & 255)
pid_t waitpid(pid_t, int *, int);
#endif
