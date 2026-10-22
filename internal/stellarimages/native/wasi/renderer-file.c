/* The buffer renderer has no base file and exposes no external GFile backend.
 * URI identity comes from upstream GDummyFile; reads fail explicitly.
 * Embedded data: resources are decoded by librsvg itself.
 */
#include <gio/gio.h>
#include "gdummyfile.h"

typedef GFileIface GFileInterface;
G_DEFINE_INTERFACE(GFile, g_file, G_TYPE_OBJECT)
static void g_file_default_init(GFileIface *iface) { (void)iface; }

GFile *g_file_new_for_uri(const char *uri) {
  g_return_val_if_fail(uri != NULL, NULL);
  return _g_dummy_file_new(uri);
}
GFile *g_file_resolve_relative_path(GFile *file,const char *path) {
  g_return_val_if_fail(G_IS_FILE(file), NULL);
  g_return_val_if_fail(path != NULL, NULL);
  return G_FILE_GET_IFACE(file)->resolve_relative_path(file,path);
}
GFile *g_file_get_child(GFile *file,const char *name) {
  return g_file_resolve_relative_path(file,name);
}
GFileInputStream *g_file_read(GFile *file,GCancellable *cancellable,GError **error) {
  g_return_val_if_fail(G_IS_FILE(file), NULL);
  if (g_cancellable_set_error_if_cancelled(cancellable,error)) return NULL;
  g_set_error_literal(error,G_IO_ERROR,G_IO_ERROR_NOT_SUPPORTED,"External SVG files are not exposed to the buffer renderer");
  return NULL;
}
gboolean g_file_load_contents(GFile *file,GCancellable *cancellable,char **contents,gsize *length,char **etag,GError **error) {
  if (contents) *contents=NULL;
  if (length) *length=0;
  if (etag) *etag=NULL;
  return g_file_read(file,cancellable,error)!=NULL;
}
