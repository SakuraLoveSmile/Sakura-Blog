/* WASI preview1 platform boundaries: no host accounts or processes. */
#include <errno.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <pwd.h>
#include <sys/wait.h>
#include <pthread.h>
#include <time.h>

/* WASI libc exposes only UTC; changing TZ cannot access a host timezone. */
void tzset(void) {}
uid_t getuid(void) { return 1000; }
uid_t geteuid(void) { return 1000; }
gid_t getgid(void) { return 1000; }
gid_t getegid(void) { return 1000; }
pid_t getppid(void) { return 0; }
static struct passwd account = { "stellar", "", 1000, 1000, "Stellar renderer", "/", "" };
struct passwd *getpwuid(uid_t id) { if (id == 1000) return &account; errno = ENOENT; return NULL; }
struct passwd *getpwnam(const char *name) { if (!strcmp(name, account.pw_name)) return &account; errno = ENOENT; return NULL; }
static int account_copy(struct passwd *found, struct passwd *out, char *buf, size_t n, struct passwd **result) {
  *result = NULL;
  if (!found) return 0;
  const char *src[] = { found->pw_name, found->pw_passwd, found->pw_gecos, found->pw_dir, found->pw_shell };
  size_t total = 0;
  for (size_t i=0;i<5;i++) total += strlen(src[i])+1;
  if (n < total) return ERANGE;
  *out = *found;
  char **dest[] = { &out->pw_name, &out->pw_passwd, &out->pw_gecos, &out->pw_dir, &out->pw_shell };
  for (size_t i=0;i<5;i++) { size_t len=strlen(src[i])+1; memcpy(buf,src[i],len); *dest[i]=buf; buf+=len; }
  *result = out;
  return 0;
}
int getpwuid_r(uid_t id, struct passwd *out, char *buf, size_t n, struct passwd **result) { return account_copy(getpwuid(id),out,buf,n,result); }
int getpwnam_r(const char *name, struct passwd *out, char *buf, size_t n, struct passwd **result) { return account_copy(getpwnam(name),out,buf,n,result); }
pid_t fork(void) { errno=ENOSYS; return -1; }
int pipe(int fds[2]) { fds[0]=fds[1]=-1; errno=ENOSYS; return -1; }
int dup(int fd) { (void)fd; errno=ENOSYS; return -1; }
int dup2(int oldfd,int newfd) { (void)oldfd; (void)newfd; errno=ENOSYS; return -1; }
int execvp(const char *path,char *const argv[]) { (void)path; (void)argv; errno=ENOSYS; return -1; }
int execve(const char *path,char *const argv[],char *const env[]) { (void)path; (void)argv; (void)env; errno=ENOSYS; return -1; }
int setpgid(pid_t pid,pid_t pgid) { (void)pid; (void)pgid; errno=ENOSYS; return -1; }
pid_t setsid(void) { errno=ENOSYS; return -1; }
pid_t waitpid(pid_t pid,int *status,int options) { (void)pid; (void)status; (void)options; errno=ENOSYS; return -1; }
/* Single-thread WASI cannot end a worker which was never created. */
_Noreturn void pthread_exit(void *result) { (void)result; abort(); }

#include <signal.h>
int sigemptyset(sigset_t *set) { memset(set,0,sizeof(*set)); return 0; }
int sigfillset(sigset_t *set) { memset(set,255,sizeof(*set)); return 0; }
int sigaction(int sig,const struct sigaction *action,struct sigaction *old) { (void)sig; (void)action; (void)old; errno=ENOSYS; return -1; }
int pthread_sigmask(int how,const sigset_t *set,sigset_t *old) { (void)how; (void)set; (void)old; return ENOSYS; }
int kill(pid_t pid,int sig) { (void)pid; (void)sig; errno=ENOSYS; return -1; }
int execv(const char *path,char *const argv[]) { return execvp(path,argv); }

#include <sys/socket.h>
#include <resolv.h>
int socket(int domain,int type,int protocol) { (void)domain; (void)type; (void)protocol; errno=ENOSYS; return -1; }
int res_query(const char *name,int dnsclass,int type,unsigned char *answer,int length) { (void)name; (void)dnsclass; (void)type; (void)answer; (void)length; errno=ENOSYS; return -1; }

/* XML hash hardening uses WASI random_get through libc getentropy. */
__attribute__((weak)) void arc4random_buf(void *buffer, size_t length) {
  unsigned char *cursor=buffer;
  while (length) {
    size_t chunk=length>256?256:length;
    if (getentropy(cursor,chunk)!=0) abort();
    cursor+=chunk;
    length-=chunk;
  }
}
