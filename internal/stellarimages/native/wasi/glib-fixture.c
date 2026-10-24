#include <glib.h>
#include <glib-object.h>
#include <gio/gio.h>
#include <errno.h>
#include <pwd.h>
#include <unistd.h>
#include <stdio.h>
#include <sys/socket.h>
#include <time.h>
#include <string.h>
__attribute__((import_module("stellar_ffi"), import_name("type_init")))
extern void stellar_type_init(void *,void *,void *);
static int invalid_initializer(void *instance,void *data) { return instance==data; }
__attribute__((export_name("test_invalid_type_init")))
void test_invalid_type_init(void) { stellar_type_init((void *)invalid_initializer,NULL,NULL); }
static double measure(GObject *object,int input,void *data) { (void)object; return input+*(double *)data; }
static void *worker(void *data) { return data; }
int main(void) {
  GError *error=NULL;
  GRegex *regex=g_regex_new("^[[:alpha:]]+$",0,0,&error);
  g_assert_no_error(error);
  g_assert_true(g_regex_match(regex,"Stellar",0,NULL));
  g_regex_unref(regex);
  guint signal=g_signal_new("measure",G_TYPE_OBJECT,G_SIGNAL_RUN_LAST,0,NULL,NULL,g_cclosure_marshal_generic,G_TYPE_DOUBLE,1,G_TYPE_INT);
  GObject *object=g_object_new(G_TYPE_OBJECT,NULL);
  double delta=0.25,result=0;
  g_signal_connect(object,"measure",G_CALLBACK(measure),&delta);
  g_signal_emit(object,signal,0,17,&result);
  g_assert_cmpfloat(result,==,17.25);
  g_object_unref(object);
  struct passwd account,*found=NULL;
  char buffer[128];
  g_assert_cmpint(getpwuid_r(getuid(),&account,buffer,sizeof(buffer),&found),==,0);
  g_assert_nonnull(found);
  g_assert_cmpstr(account.pw_name,==,"stellar");
  errno=0;
  g_assert_cmpint(fork(),==,-1);
  g_assert_cmpint(errno,==,ENOSYS);
  g_assert_cmpint(socket(0,0,0),==,-1);
  g_assert_cmpint(errno,==,ENOSYS);
  g_assert_cmpint(setenv("TZ","Asia/Taipei",1),==,0);
  tzset();
  time_t epoch=0;
  g_assert_cmpint(localtime(&epoch)->tm_hour,==,0);
  GThread *thread=g_thread_try_new("denied",worker,NULL,&error);
  g_assert_null(thread);
  g_assert_error(error,G_THREAD_ERROR,G_THREAD_ERROR_AGAIN);
  g_clear_error(&error);
  static const unsigned char compressed[] = { 31,139,8,0,0,0,0,0,2,255,11,46,73,205,201,73,44,82,120,178,119,193,211,165,123,1,254,89,200,90,14,0,0,0 };
  GInputStream *memory=g_memory_input_stream_new_from_data(compressed,sizeof(compressed),NULL);
  GZlibDecompressor *decomp=g_zlib_decompressor_new(G_ZLIB_COMPRESSOR_FORMAT_GZIP);
  GInputStream *converted=g_converter_input_stream_new(memory,G_CONVERTER(decomp));
  char output[64]={0};
  gsize received=0;
  g_assert_true(g_input_stream_read_all(converted,output,sizeof(output),&received,NULL,&error));
  g_assert_no_error(error);
  g_assert_cmpstr(output,==,"Stellar 你好");
  g_assert_cmpint(received,==,14);
  g_object_unref(converted);
  g_object_unref(decomp);
  g_object_unref(memory);
  GCancellable *cancel=g_cancellable_new();
  g_cancellable_cancel(cancel);
  memory=g_memory_input_stream_new_from_data("x",1,NULL);
  g_assert_true(g_cancellable_set_error_if_cancelled(cancel,&error));
  g_assert_error(error,G_IO_ERROR,G_IO_ERROR_CANCELLED);
  g_clear_error(&error);
  g_object_unref(memory);
  g_object_unref(cancel);
  puts("glib platform gobject ffi and gio streams ok");
  return 0;
}
