#ifndef STELLAR_WASI_SIGNAL_H
#define STELLAR_WASI_SIGNAL_H
#include_next <signal.h>
#include <sys/types.h>
#define SIG_SETMASK 2
#define SA_NOCLDSTOP 1
struct sigaction { void (*sa_handler)(int); sigset_t sa_mask; int sa_flags; };
int sigemptyset(sigset_t *);
int sigfillset(sigset_t *);
int sigaction(int, const struct sigaction *, struct sigaction *);
int pthread_sigmask(int, const sigset_t *, sigset_t *);
int kill(pid_t, int);
#endif
