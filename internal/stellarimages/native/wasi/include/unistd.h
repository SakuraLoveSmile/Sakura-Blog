#ifndef STELLAR_WASI_UNISTD_H
#define STELLAR_WASI_UNISTD_H
#include_next <unistd.h>
uid_t getuid(void);
uid_t geteuid(void);
gid_t getgid(void);
gid_t getegid(void);
pid_t getppid(void);
pid_t fork(void);
int pipe(int [2]);
int dup(int);
int dup2(int, int);
int execv(const char *, char *const []);
int execvp(const char *, char *const []);
int execve(const char *, char *const [], char *const []);
int setpgid(pid_t, pid_t);
pid_t setsid(void);
#endif
